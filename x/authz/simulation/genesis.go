package simulation

import (
	"math/rand"

	codectypes "github.com/line/lbm-sdk/codec/types"
	sdk "github.com/line/lbm-sdk/types"
	"github.com/line/lbm-sdk/types/module"
	simtypes "github.com/line/lbm-sdk/types/simulation"
	"github.com/line/lbm-sdk/x/authz"
	banktypes "github.com/line/lbm-sdk/x/bank/types"
	govtypes "github.com/line/lbm-sdk/x/gov/types"
)

// genGrant returns a slice of authorization grants.
func genGrant(r *rand.Rand, accounts []simtypes.Account) []authz.GrantAuthorization {
	authorizations := make([]authz.GrantAuthorization, len(accounts)-1)
	for i := 0; i < len(accounts)-1; i++ {
		granter := accounts[i]
		grantee := accounts[i+1]
		authorizations[i] = authz.GrantAuthorization{
			Granter:       granter.Address.String(),
			Grantee:       grantee.Address.String(),
			Authorization: generateRandomGrant(r),
		}
	}

	return authorizations
}

func generateRandomGrant(r *rand.Rand) *codectypes.Any {
	authorizations := make([]*codectypes.Any, 2)
	authorizations[0] = newAnyAuthorization(banktypes.NewSendAuthorization(sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(1000)))))
	authorizations[1] = newAnyAuthorization(authz.NewGenericAuthorization(sdk.MsgTypeURL(&govtypes.MsgSubmitProposal{})))

	return authorizations[r.Intn(len(authorizations))]
}

func newAnyAuthorization(a authz.Authorization) *codectypes.Any {
	any, err := codectypes.NewAnyWithValue(a)
	if err != nil {
		panic(err)
	}

	return any
}

// RandomizedGenState generates a random GenesisState for authz.
func RandomizedGenState(simState *module.SimulationState) {
	var grants []authz.GrantAuthorization
	simState.AppParams.GetOrGenerate(
		simState.Cdc, "authz", &grants, simState.Rand,
		func(r *rand.Rand) { grants = genGrant(r, simState.Accounts) },
	)

	authzGrantsGenesis := authz.NewGenesisState(grants)

	simState.GenState[authz.ModuleName] = simState.Cdc.MustMarshalJSON(authzGrantsGenesis)
}
