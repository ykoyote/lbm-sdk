package keeper

import (
	sdk "github.com/line/lbm-sdk/types"
	sdkerrors "github.com/line/lbm-sdk/types/errors"
	"github.com/line/lbm-sdk/x/collection"
)

func (k Keeper) CreateContract(ctx sdk.Context, creator sdk.AccAddress, contract collection.Contract) string {
	contractID := k.createContract(ctx, contract)

	event := collection.EventCreatedContract{
		Creator:    creator.String(),
		ContractId: contractID,
		Name:       contract.Name,
		Meta:       contract.Meta,
		BaseImgUri: contract.BaseImgUri,
	}
	ctx.EventManager().EmitEvent(collection.NewEventCreateCollection(event))
	if err := ctx.EventManager().EmitTypedEvent(&event); err != nil {
		panic(err)
	}

	eventGrant := collection.EventGranted{
		ContractId: contractID,
		Grantee:    creator.String(),
	}
	ctx.EventManager().EmitEvent(collection.NewEventGrantPermTokenHead(eventGrant))
	for _, permission := range collection.Permission_value {
		p := collection.Permission(permission)
		if p == collection.PermissionUnspecified {
			continue
		}

		eventGrant.Permission = p
		ctx.EventManager().EmitEvent(collection.NewEventGrantPermTokenBody(eventGrant))
		k.Grant(ctx, contractID, []byte{}, creator, collection.Permission(permission))
	}

	return contractID
}

func (k Keeper) createContract(ctx sdk.Context, contract collection.Contract) string {
	contractID := k.classKeeper.NewID(ctx)
	contract.ContractId = contractID
	k.setContract(ctx, contract)

	// set the next class ids
	nextIDs := collection.DefaultNextClassIDs(contractID)
	k.setNextClassIDs(ctx, nextIDs)

	return contractID
}

func (k Keeper) GetContract(ctx sdk.Context, contractID string) (*collection.Contract, error) {
	store := ctx.KVStore(k.storeKey)
	key := contractKey(contractID)
	bz := store.Get(key)
	if bz == nil {
		return nil, sdkerrors.ErrNotFound.Wrapf("no such a contract: %s", contractID)
	}

	var contract collection.Contract
	if err := contract.Unmarshal(bz); err != nil {
		panic(err)
	}
	return &contract, nil
}

func (k Keeper) setContract(ctx sdk.Context, contract collection.Contract) {
	store := ctx.KVStore(k.storeKey)
	key := contractKey(contract.ContractId)

	bz, err := contract.Marshal()
	if err != nil {
		panic(err)
	}
	store.Set(key, bz)
}

func (k Keeper) CreateTokenClass(ctx sdk.Context, contractID string, class collection.TokenClass) (*string, error) {
	if _, err := k.GetContract(ctx, contractID); err != nil {
		return nil, err
	}

	nextClassIDs := k.getNextClassIDs(ctx, contractID)
	class.SetId(&nextClassIDs)
	k.setNextClassIDs(ctx, nextClassIDs)

	if err := class.ValidateBasic(); err != nil {
		return nil, err
	}
	k.setTokenClass(ctx, contractID, class)

	if nftClass, ok := class.(*collection.NFTClass); ok {
		k.setNextTokenID(ctx, contractID, nftClass.Id, sdk.OneUint())

		// legacy
		k.setLegacyTokenType(ctx, contractID, nftClass.Id)
	}

	if ftClass, ok := class.(*collection.FTClass); ok {
		// legacy
		k.setLegacyToken(ctx, contractID, collection.NewFTID(ftClass.Id))
	}

	id := class.GetId()
	return &id, nil
}

func (k Keeper) GetTokenClass(ctx sdk.Context, contractID, classID string) (collection.TokenClass, error) {
	store := ctx.KVStore(k.storeKey)
	key := classKey(contractID, classID)
	bz := store.Get(key)
	if bz == nil {
		return nil, sdkerrors.ErrNotFound.Wrapf("no such a class in contract %s: %s", contractID, classID)
	}

	var class collection.TokenClass
	if err := k.cdc.UnmarshalInterface(bz, &class); err != nil {
		panic(err)
	}
	return class, nil
}

func (k Keeper) setTokenClass(ctx sdk.Context, contractID string, class collection.TokenClass) {
	store := ctx.KVStore(k.storeKey)
	key := classKey(contractID, class.GetId())

	bz, err := k.cdc.MarshalInterface(class)
	if err != nil {
		panic(err)
	}
	store.Set(key, bz)
}

func (k Keeper) getNextClassIDs(ctx sdk.Context, contractID string) collection.NextClassIDs {
	store := ctx.KVStore(k.storeKey)
	key := nextClassIDKey(contractID)
	bz := store.Get(key)
	if bz == nil {
		panic(sdkerrors.ErrNotFound.Wrapf("no next class ids of contract %s", contractID))
	}

	var class collection.NextClassIDs
	if err := class.Unmarshal(bz); err != nil {
		panic(err)
	}
	return class
}

func (k Keeper) setNextClassIDs(ctx sdk.Context, ids collection.NextClassIDs) {
	store := ctx.KVStore(k.storeKey)
	key := nextClassIDKey(ids.ContractId)

	bz, err := ids.Marshal()
	if err != nil {
		panic(err)
	}
	store.Set(key, bz)
}

func (k Keeper) MintFT(ctx sdk.Context, contractID string, to sdk.AccAddress, amount []collection.Coin) error {
	for _, coin := range amount {
		if err := collection.ValidateFTID(coin.TokenId); err != nil {
			return err
		}

		classID := collection.SplitTokenID(coin.TokenId)
		class, err := k.GetTokenClass(ctx, contractID, classID)
		if err != nil {
			return err
		}

		ftClass, ok := class.(*collection.FTClass)
		if !ok {
			return sdkerrors.ErrInvalidType.Wrapf("not a class of fungible token: %s", classID)
		}

		if !ftClass.Mintable {
			return sdkerrors.ErrInvalidRequest.Wrapf("class is not mintable")
		}

		k.mintFT(ctx, contractID, to, classID, coin.Amount)
	}

	return nil
}

func (k Keeper) mintFT(ctx sdk.Context, contractID string, to sdk.AccAddress, classID string, amount sdk.Int) {
	tokenID := collection.NewFTID(classID)
	k.setBalance(ctx, contractID, to, tokenID, amount)

	// update statistics
	supply := k.GetSupply(ctx, contractID, classID)
	k.setSupply(ctx, contractID, classID, supply.Add(amount))

	minted := k.GetMinted(ctx, contractID, classID)
	k.setMinted(ctx, contractID, classID, minted.Add(amount))
}

func (k Keeper) MintNFT(ctx sdk.Context, contractID string, to sdk.AccAddress, params []collection.MintNFTParam) ([]collection.NFT, error) {
	tokens := make([]collection.NFT, 0, len(params))
	for _, param := range params {
		classID := param.TokenType
		class, err := k.GetTokenClass(ctx, contractID, classID)
		if err != nil {
			return nil, err
		}

		if _, ok := class.(*collection.NFTClass); !ok {
			return nil, sdkerrors.ErrInvalidType.Wrapf("not a class of non-fungible token: %s", classID)
		}

		nextTokenID := k.getNextTokenID(ctx, contractID, classID)
		k.setNextTokenID(ctx, contractID, classID, nextTokenID.Incr())
		tokenID := collection.NewNFTID(classID, int(nextTokenID.Uint64()))

		amount := sdk.OneInt()

		k.setBalance(ctx, contractID, to, tokenID, amount)
		k.setOwner(ctx, contractID, tokenID, to)

		token := collection.NFT{
			Id:   tokenID,
			Name: param.Name,
			Meta: param.Meta,
		}
		k.setNFT(ctx, contractID, token)

		// update statistics
		supply := k.GetSupply(ctx, contractID, classID)
		k.setSupply(ctx, contractID, classID, supply.Add(amount))

		minted := k.GetMinted(ctx, contractID, classID)
		k.setMinted(ctx, contractID, classID, minted.Add(amount))

		tokens = append(tokens, token)

		// legacy
		k.setLegacyToken(ctx, contractID, tokenID)
	}

	return tokens, nil
}

func (k Keeper) BurnCoins(ctx sdk.Context, contractID string, from sdk.AccAddress, amount []collection.Coin) ([]collection.Coin, error) {
	if err := k.subtractCoins(ctx, contractID, from, amount); err != nil {
		return nil, err
	}

	burntAmount := []collection.Coin{}
	for _, coin := range amount {
		burntAmount = append(burntAmount, coin)
		if err := collection.ValidateNFTID(coin.TokenId); err == nil {
			// legacy
			k.iterateDescendants(ctx, contractID, coin.TokenId, func(descendantID string, _ int) (stop bool) {
				ctx.EventManager().EmitEvent(collection.NewEventOperationBurnNFT(contractID, descendantID))
				return false
			})

			k.deleteOwner(ctx, contractID, coin.TokenId)
			k.deleteNFT(ctx, contractID, coin.TokenId)
			pruned := k.pruneNFT(ctx, contractID, coin.TokenId)

			for _, id := range pruned {
				burntAmount = append(burntAmount, collection.NewCoin(id, sdk.OneInt()))
			}

			// legacy
			k.deleteLegacyToken(ctx, contractID, coin.TokenId)
		}
	}

	// update statistics
	for _, coin := range burntAmount {
		classID := collection.SplitTokenID(coin.TokenId)
		supply := k.GetSupply(ctx, contractID, classID)
		k.setSupply(ctx, contractID, classID, supply.Sub(coin.Amount))

		burnt := k.GetBurnt(ctx, contractID, classID)
		k.setBurnt(ctx, contractID, classID, burnt.Add(coin.Amount))
	}

	return burntAmount, nil
}

func (k Keeper) getNextTokenID(ctx sdk.Context, contractID string, classID string) sdk.Uint {
	store := ctx.KVStore(k.storeKey)
	key := nextTokenIDKey(contractID, classID)
	bz := store.Get(key)
	if bz == nil {
		panic(sdkerrors.ErrNotFound.Wrapf("no next token id of token class %s", classID))
	}

	var id sdk.Uint
	if err := id.Unmarshal(bz); err != nil {
		panic(err)
	}
	return id
}

func (k Keeper) setNextTokenID(ctx sdk.Context, contractID string, classID string, tokenID sdk.Uint) {
	store := ctx.KVStore(k.storeKey)
	key := nextTokenIDKey(contractID, classID)

	bz, err := tokenID.Marshal()
	if err != nil {
		panic(err)
	}
	store.Set(key, bz)
}

func (k Keeper) ModifyContract(ctx sdk.Context, contractID string, operator sdk.AccAddress, changes []collection.Attribute) error {
	contract, err := k.GetContract(ctx, contractID)
	if err != nil {
		return err
	}

	modifiers := map[string]func(string){
		collection.AttributeKeyName.String(): func(name string) {
			contract.Name = name
		},
		collection.AttributeKeyBaseImgURI.String(): func(uri string) {
			contract.Name = uri
		},
		collection.AttributeKeyMeta.String(): func(meta string) {
			contract.Meta = meta
		},
	}
	for _, change := range changes {
		modifiers[change.Key](change.Value)
	}

	k.setContract(ctx, *contract)

	event := collection.EventModifiedContract{
		ContractId: contractID,
		Operator:   operator.String(),
		Changes:    changes,
	}
	if err := ctx.EventManager().EmitTypedEvent(&event); err != nil {
		panic(err)
	}
	return nil
}

func (k Keeper) ModifyTokenClass(ctx sdk.Context, contractID string, classID string, operator sdk.AccAddress, changes []collection.Attribute) error {
	class, err := k.GetTokenClass(ctx, contractID, classID)
	if err != nil {
		return err
	}

	modifiers := map[string]func(string){
		collection.AttributeKeyName.String(): func(name string) {
			class.SetName(name)
		},
		collection.AttributeKeyMeta.String(): func(meta string) {
			class.SetMeta(meta)
		},
	}
	for _, change := range changes {
		modifiers[change.Key](change.Value)
	}

	k.setTokenClass(ctx, contractID, class)

	event := collection.EventModifiedTokenClass{
		ContractId: contractID,
		ClassId:    class.GetId(),
		Operator:   operator.String(),
		Changes:    changes,
	}
	if err := ctx.EventManager().EmitTypedEvent(&event); err != nil {
		panic(err)
	}
	return nil
}

func (k Keeper) ModifyNFT(ctx sdk.Context, contractID string, tokenID string, operator sdk.AccAddress, changes []collection.Attribute) error {
	token, err := k.GetNFT(ctx, contractID, tokenID)
	if err != nil {
		return err
	}

	modifiers := map[string]func(string){
		collection.AttributeKeyName.String(): func(name string) {
			token.Name = name
		},
		collection.AttributeKeyMeta.String(): func(meta string) {
			token.Meta = meta
		},
	}
	for _, change := range changes {
		modifiers[change.Key](change.Value)
	}

	k.setNFT(ctx, contractID, *token)

	event := collection.EventModifiedNFT{
		ContractId: contractID,
		TokenId:    tokenID,
		Operator:   operator.String(),
		Changes:    changes,
	}
	if err := ctx.EventManager().EmitTypedEvent(&event); err != nil {
		panic(err)
	}
	return nil
}

func (k Keeper) Grant(ctx sdk.Context, contractID string, granter, grantee sdk.AccAddress, permission collection.Permission) {
	k.grant(ctx, contractID, grantee, permission)

	event := collection.EventGranted{
		ContractId: contractID,
		Granter:    granter.String(),
		Grantee:    grantee.String(),
		Permission: permission,
	}
	if err := ctx.EventManager().EmitTypedEvent(&event); err != nil {
		panic(err)
	}
}

func (k Keeper) grant(ctx sdk.Context, contractID string, grantee sdk.AccAddress, permission collection.Permission) {
	k.setGrant(ctx, contractID, grantee, permission)

	// create account if grantee does not exist.
	k.createAccountOnAbsence(ctx, grantee)
}

func (k Keeper) Abandon(ctx sdk.Context, contractID string, grantee sdk.AccAddress, permission collection.Permission) {
	k.deleteGrant(ctx, contractID, grantee, permission)

	event := collection.EventRenounced{
		ContractId: contractID,
		Grantee:    grantee.String(),
		Permission: permission,
	}
	if err := ctx.EventManager().EmitTypedEvent(&event); err != nil {
		panic(err)
	}
}

func (k Keeper) GetGrant(ctx sdk.Context, contractID string, grantee sdk.AccAddress, permission collection.Permission) (*collection.Grant, error) {
	store := ctx.KVStore(k.storeKey)
	if store.Has(grantKey(contractID, grantee, permission)) {
		return &collection.Grant{
			Grantee:    grantee.String(),
			Permission: permission,
		}, nil
	}
	return nil, sdkerrors.ErrNotFound.Wrapf("no %s permission granted on %s", permission, grantee)
}

func (k Keeper) setGrant(ctx sdk.Context, contractID string, grantee sdk.AccAddress, permission collection.Permission) {
	store := ctx.KVStore(k.storeKey)
	key := grantKey(contractID, grantee, permission)
	store.Set(key, []byte{})
}

func (k Keeper) deleteGrant(ctx sdk.Context, contractID string, grantee sdk.AccAddress, permission collection.Permission) {
	store := ctx.KVStore(k.storeKey)
	key := grantKey(contractID, grantee, permission)
	store.Delete(key)
}

func (k Keeper) getStatistic(ctx sdk.Context, keyPrefix []byte, contractID string, classID string) sdk.Int {
	store := ctx.KVStore(k.storeKey)
	amount := sdk.ZeroInt()
	bz := store.Get(statisticKey(keyPrefix, contractID, classID))
	if bz != nil {
		if err := amount.Unmarshal(bz); err != nil {
			panic(err)
		}
	}

	return amount
}

func (k Keeper) setStatistic(ctx sdk.Context, keyPrefix []byte, contractID string, classID string, amount sdk.Int) {
	store := ctx.KVStore(k.storeKey)
	key := statisticKey(keyPrefix, contractID, classID)
	if amount.IsZero() {
		store.Delete(key)
	} else {
		bz, err := amount.Marshal()
		if err != nil {
			panic(err)
		}
		store.Set(key, bz)
	}
}

func (k Keeper) GetSupply(ctx sdk.Context, contractID string, classID string) sdk.Int {
	return k.getStatistic(ctx, supplyKeyPrefix, contractID, classID)
}

func (k Keeper) GetMinted(ctx sdk.Context, contractID string, classID string) sdk.Int {
	return k.getStatistic(ctx, mintedKeyPrefix, contractID, classID)
}

func (k Keeper) GetBurnt(ctx sdk.Context, contractID string, classID string) sdk.Int {
	return k.getStatistic(ctx, burntKeyPrefix, contractID, classID)
}

func (k Keeper) setSupply(ctx sdk.Context, contractID string, classID string, amount sdk.Int) {
	k.setStatistic(ctx, supplyKeyPrefix, contractID, classID, amount)
}

func (k Keeper) setMinted(ctx sdk.Context, contractID string, classID string, amount sdk.Int) {
	k.setStatistic(ctx, mintedKeyPrefix, contractID, classID, amount)
}

func (k Keeper) setBurnt(ctx sdk.Context, contractID string, classID string, amount sdk.Int) {
	k.setStatistic(ctx, burntKeyPrefix, contractID, classID, amount)
}
