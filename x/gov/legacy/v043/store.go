package v043

import (
	"fmt"

	"github.com/line/lbm-sdk/codec"
	"github.com/line/lbm-sdk/store/prefix"
	sdk "github.com/line/lbm-sdk/types"
	"github.com/line/lbm-sdk/types/address"
	"github.com/line/lbm-sdk/x/gov/types"
)

const proposalIDLen = 8

// migratePrefixProposalAddress is a helper function that migrates all keys of format:
// <prefix_bytes><proposal_id (8 bytes)><address_bytes>
// into format:
// <prefix_bytes><proposal_id (8 bytes)><address_len (1 byte)><address_bytes>
func migratePrefixProposalAddress(store sdk.KVStore, prefixBz []byte) {
	oldStore := prefix.NewStore(store, prefixBz)

	oldStoreIter := oldStore.Iterator(nil, nil)
	defer oldStoreIter.Close()

	for ; oldStoreIter.Valid(); oldStoreIter.Next() {
		proposalID := oldStoreIter.Key()[:proposalIDLen]
		addr := oldStoreIter.Key()[proposalIDLen:]
		newStoreKey := append(append(prefixBz, proposalID...), address.MustLengthPrefix(addr)...)

		// Set new key on store. Values don't change.
		store.Set(newStoreKey, oldStoreIter.Value())
		oldStore.Delete(oldStoreIter.Key())
	}
}

// migrateStoreWeightedVotes migrates a legacy vote to an ADR-037 weighted vote.
// Important: the `oldVote` has its `Option` field set, whereas the new weighted
// vote has its `Options` field set.
func migrateVote(oldVote types.Vote) types.Vote {
	return types.Vote{
		ProposalId: oldVote.ProposalId,
		Voter:      oldVote.Voter,
		Options:    types.NewNonSplitVoteOption(oldVote.Option), // nolint: staticcheck
	}
}

// migrateStoreWeightedVotes migrates in-place all legacy votes to ADR-037 weighted votes.
func migrateStoreWeightedVotes(store sdk.KVStore, cdc codec.BinaryCodec) error {
	iterator := sdk.KVStorePrefixIterator(store, types.VotesKeyPrefix)

	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var oldVote types.Vote
		err := cdc.Unmarshal(iterator.Value(), &oldVote)
		if err != nil {
			return err
		}

		newVote := migrateVote(oldVote)
		fmt.Println("migrateStoreWeightedVotes newVote=", newVote)
		bz, err := cdc.Marshal(&newVote)
		if err != nil {
			return err
		}

		store.Set(iterator.Key(), bz)
	}

	return nil
}

// MigrateStore performs in-place store migrations from v0.40 to v0.43. The
// migration includes:
//
// - Change addresses to be length-prefixed.
func MigrateStore(ctx sdk.Context, storeKey sdk.StoreKey, cdc codec.BinaryCodec) error {
	store := ctx.KVStore(storeKey)
	migratePrefixProposalAddress(store, types.DepositsKeyPrefix)
	migratePrefixProposalAddress(store, types.VotesKeyPrefix)
	return migrateStoreWeightedVotes(store, cdc)
}
