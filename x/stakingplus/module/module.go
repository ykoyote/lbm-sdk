package module

import (
	"encoding/json"

	abci "github.com/line/ostracon/abci/types"

	"github.com/line/lbm-sdk/codec"
	"github.com/line/lbm-sdk/types/module"

	codectypes "github.com/line/lbm-sdk/codec/types"
	sdk "github.com/line/lbm-sdk/types"

	"github.com/line/lbm-sdk/x/stakingplus"
	"github.com/line/lbm-sdk/x/stakingplus/keeper"

	"github.com/line/lbm-sdk/x/staking"
	stakingkeeper "github.com/line/lbm-sdk/x/staking/keeper"
	stakingtypes "github.com/line/lbm-sdk/x/staking/types"
)

var _ module.AppModuleBasic = AppModuleBasic{}

// AppModuleBasic defines the basic application module used by the stakingplus module.
type AppModuleBasic struct {
	staking.AppModuleBasic
}

func (b AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	b.AppModuleBasic.RegisterInterfaces(registry)
	stakingplus.RegisterInterfaces(registry)
}

//____________________________________________________________________________

var _ module.AppModule = AppModule{}

// AppModule implements an application module for the stakingplus module.
type AppModule struct {
	AppModuleBasic
	impl staking.AppModule

	keeper stakingkeeper.Keeper
	ak     stakingtypes.AccountKeeper
	bk     stakingtypes.BankKeeper
	fk     stakingplus.FoundationKeeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(cdc codec.Codec, keeper stakingkeeper.Keeper, ak stakingtypes.AccountKeeper, bk stakingtypes.BankKeeper, fk stakingplus.FoundationKeeper) AppModule {
	impl := staking.NewAppModule(cdc, keeper, ak, bk)
	return AppModule{
		AppModuleBasic: AppModuleBasic{
			impl.AppModuleBasic,
		},
		impl:   impl,
		keeper: keeper,
		ak:     ak,
		bk:     bk,
		fk:     fk,
	}
}

// RegisterInvariants does nothing, there are no invariants to enforce
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	am.impl.RegisterInvariants(ir)
}

// Route is empty, as we do not handle Messages (just proposals)
func (am AppModule) Route() sdk.Route {
	return am.impl.Route()
}

// QuerierRoute returns the route we respond to for abci queries
func (am AppModule) QuerierRoute() string {
	return am.impl.QuerierRoute()
}

// LegacyQuerierHandler registers a query handler to respond to the module-specific queries
func (am AppModule) LegacyQuerierHandler(legacyQuerierCdc *codec.LegacyAmino) sdk.Querier {
	return am.impl.LegacyQuerierHandler(legacyQuerierCdc)
}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	stakingtypes.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper, am.fk))
	querier := stakingkeeper.Querier{Keeper: am.keeper}
	stakingtypes.RegisterQueryServer(cfg.QueryServer(), querier)

	// m := keeper.NewMigrator(am.keeper)
	// migrations := map[uint64]func(sdk.Context) error{}
	// for ver, handler := range migrations {
	// 	if err := cfg.RegisterMigration(foundation.ModuleName, ver, handler); err != nil {
	// 		panic(fmt.Sprintf("failed to migrate x/%s from version %d to %d: %v", stakingplus.ModuleName, ver, ver+1, err))
	// 	}
	// }
}

// InitGenesis performs genesis initialization for the stakingplus module. It returns
// no validator updates.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) []abci.ValidatorUpdate {
	return am.impl.InitGenesis(ctx, cdc, data)
}

// ExportGenesis returns the exported genesis state as raw bytes for the stakingplus
// module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	return am.impl.ExportGenesis(ctx, cdc)
}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (am AppModule) ConsensusVersion() uint64 {
	return am.impl.ConsensusVersion()
}

// BeginBlock returns the begin blocker for the stakingplus module.
func (am AppModule) BeginBlock(ctx sdk.Context, req abci.RequestBeginBlock) {
	am.impl.BeginBlock(ctx, req)
}

// EndBlock returns the end blocker for the stakingplus module. It returns no validator
// updates.
func (am AppModule) EndBlock(ctx sdk.Context, req abci.RequestEndBlock) []abci.ValidatorUpdate {
	return am.impl.EndBlock(ctx, req)
}
