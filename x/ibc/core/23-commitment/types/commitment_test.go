package types_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	dbm "github.com/tendermint/tm-db"

	"github.com/line/lbm-sdk/store/iavl"
	"github.com/line/lbm-sdk/store/rootmulti"
	storetypes "github.com/line/lbm-sdk/store/types"
)

type MerkleTestSuite struct {
	suite.Suite

	store     *rootmulti.Store
	storeKey  *storetypes.KVStoreKey
	iavlStore *iavl.Store
}

func (suite *MerkleTestSuite) SetupTest() {
	db := dbm.NewMemDB()
	suite.store = rootmulti.NewStore(db)

	suite.storeKey = storetypes.NewKVStoreKey("iavlStoreKey")

	suite.store.MountStoreWithDB(suite.storeKey, storetypes.StoreTypeIAVL, nil)
	suite.store.LoadVersion(0)

	suite.iavlStore = suite.store.GetCommitStore(suite.storeKey).(*iavl.Store)
}

func TestMerkleTestSuite(t *testing.T) {
	suite.Run(t, new(MerkleTestSuite))
}
