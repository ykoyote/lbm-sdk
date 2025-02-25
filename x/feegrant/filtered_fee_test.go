package feegrant_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ocproto "github.com/line/ostracon/proto/ostracon/types"

	"github.com/line/lbm-sdk/simapp"
	sdk "github.com/line/lbm-sdk/types"
	banktypes "github.com/line/lbm-sdk/x/bank/types"
	"github.com/line/lbm-sdk/x/feegrant"
)

func TestFilteredFeeValidAllow(t *testing.T) {
	app := simapp.Setup(false)

	ctx := app.BaseApp.NewContext(false, ocproto.Header{
		Time: time.Now(),
	})
	eth := sdk.NewCoins(sdk.NewInt64Coin("eth", 10))
	atom := sdk.NewCoins(sdk.NewInt64Coin("atom", 555))
	smallAtom := sdk.NewCoins(sdk.NewInt64Coin("atom", 43))
	bigAtom := sdk.NewCoins(sdk.NewInt64Coin("atom", 1000))
	leftAtom := sdk.NewCoins(sdk.NewInt64Coin("atom", 512))
	now := ctx.BlockTime()
	oneHour := now.Add(1 * time.Hour)

	// msg we will call in the all cases
	call := banktypes.MsgSend{}
	cases := map[string]struct {
		allowance *feegrant.BasicAllowance
		msgs      []string
		fee       sdk.Coins
		blockTime time.Time
		accept    bool
		remove    bool
		remains   sdk.Coins
	}{
		"msg contained": {
			allowance: &feegrant.BasicAllowance{},
			msgs:      []string{sdk.MsgTypeURL(&call)},
			accept:    true,
		},
		"msg not contained": {
			allowance: &feegrant.BasicAllowance{},
			msgs:      []string{"/cosmos.gov.v1beta1.MsgVote"},
			accept:    false,
		},
		"small fee without expire": {
			allowance: &feegrant.BasicAllowance{
				SpendLimit: atom,
			},
			msgs:    []string{sdk.MsgTypeURL(&call)},
			fee:     smallAtom,
			accept:  true,
			remove:  false,
			remains: leftAtom,
		},
		"all fee without expire": {
			allowance: &feegrant.BasicAllowance{
				SpendLimit: smallAtom,
			},
			msgs:   []string{sdk.MsgTypeURL(&call)},
			fee:    smallAtom,
			accept: true,
			remove: true,
		},
		"wrong fee": {
			allowance: &feegrant.BasicAllowance{
				SpendLimit: smallAtom,
			},
			msgs:   []string{sdk.MsgTypeURL(&call)},
			fee:    eth,
			accept: false,
		},
		"non-expired": {
			allowance: &feegrant.BasicAllowance{
				SpendLimit: atom,
				Expiration: &oneHour,
			},
			msgs:      []string{sdk.MsgTypeURL(&call)},
			fee:       smallAtom,
			blockTime: now,
			accept:    true,
			remove:    false,
			remains:   leftAtom,
		},
		"expired": {
			allowance: &feegrant.BasicAllowance{
				SpendLimit: atom,
				Expiration: &now,
			},
			msgs:      []string{sdk.MsgTypeURL(&call)},
			fee:       smallAtom,
			blockTime: oneHour,
			accept:    false,
			remove:    true,
		},
		"fee more than allowed": {
			allowance: &feegrant.BasicAllowance{
				SpendLimit: atom,
				Expiration: &oneHour,
			},
			msgs:      []string{sdk.MsgTypeURL(&call)},
			fee:       bigAtom,
			blockTime: now,
			accept:    false,
		},
		"with out spend limit": {
			allowance: &feegrant.BasicAllowance{
				Expiration: &oneHour,
			},
			msgs:      []string{sdk.MsgTypeURL(&call)},
			fee:       bigAtom,
			blockTime: now,
			accept:    true,
		},
		"expired no spend limit": {
			allowance: &feegrant.BasicAllowance{
				Expiration: &now,
			},
			msgs:      []string{sdk.MsgTypeURL(&call)},
			fee:       bigAtom,
			blockTime: oneHour,
			accept:    false,
		},
	}

	for name, stc := range cases {
		tc := stc // to make scopelint happy
		t.Run(name, func(t *testing.T) {
			err := tc.allowance.ValidateBasic()
			require.NoError(t, err)

			ctx := app.BaseApp.NewContext(false, ocproto.Header{}).WithBlockTime(tc.blockTime)

			// create grant
			var granter, grantee sdk.AccAddress
			allowance, err := feegrant.NewAllowedMsgAllowance(tc.allowance, tc.msgs)
			require.NoError(t, err)
			grant, err := feegrant.NewGrant(granter, grantee, allowance)
			require.NoError(t, err)

			// now try to deduct
			removed, err := allowance.Accept(ctx, tc.fee, []sdk.Msg{&call})
			if !tc.accept {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tc.remove, removed)
			if !removed {
				// mimic save & load process (#10564)
				// the cached allowance was correct even before the fix,
				// however, the saved value was not.
				// so we need this to catch the bug.

				newGranter, _ := sdk.AccAddressFromBech32(grant.Granter)
				newGrantee, _ := sdk.AccAddressFromBech32(grant.Grantee)
				// create a new updated grant
				newGrant, err := feegrant.NewGrant(
					newGranter,
					newGrantee,
					allowance)
				require.NoError(t, err)

				// save the grant
				cdc := simapp.MakeTestEncodingConfig().Marshaler
				bz, err := cdc.Marshal(&newGrant)
				require.NoError(t, err)

				// load the grant
				var loadedGrant feegrant.Grant
				err = cdc.Unmarshal(bz, &loadedGrant)
				require.NoError(t, err)

				newAllowance, err := loadedGrant.GetGrant()
				require.NoError(t, err)
				feeAllowance, err := newAllowance.(*feegrant.AllowedMsgAllowance).GetAllowance()
				require.NoError(t, err)
				assert.Equal(t, tc.remains, feeAllowance.(*feegrant.BasicAllowance).SpendLimit)
			}
		})
	}
}
