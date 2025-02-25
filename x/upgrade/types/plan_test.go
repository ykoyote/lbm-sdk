package types_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/line/ostracon/libs/log"
	ocproto "github.com/line/ostracon/proto/ostracon/types"

	codectypes "github.com/line/lbm-sdk/codec/types"
	sdk "github.com/line/lbm-sdk/types"
	"github.com/line/lbm-sdk/x/upgrade/types"
)

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestPlanString(t *testing.T) {
	cases := map[string]struct {
		p      types.Plan
		expect string
	}{
		"with height": {
			p: types.Plan{
				Name:   "by height",
				Info:   "https://foo.bar/baz",
				Height: 7890,
			},
			expect: "Upgrade Plan\n  Name: by height\n  Height: 7890\n  Info: https://foo.bar/baz.",
		},
		"neither": {
			p: types.Plan{
				Name: "almost-empty",
			},
			expect: "Upgrade Plan\n  Name: almost-empty\n  Height: 0\n  Info: .",
		},
	}

	for name, tc := range cases {
		tc := tc // copy to local variable for scopelint
		t.Run(name, func(t *testing.T) {
			s := tc.p.String()
			require.Equal(t, tc.expect, s)
		})
	}
}

func TestPlanValid(t *testing.T) {
	cases := map[string]struct {
		p     types.Plan
		valid bool
	}{
		"proper by height": {
			p: types.Plan{
				Name:   "all-good",
				Height: 123450000,
			},
			valid: true,
		},
		"no name": {
			p: types.Plan{
				Height: 123450000,
			},
		},
		"time-base upgrade": {
			p: types.Plan{
				Time: time.Now(),
			},
		},
		"IBC upgrade": {
			p: types.Plan{
				Height:              123450000,
				UpgradedClientState: &codectypes.Any{},
			},
		},
		"no due at": {
			p: types.Plan{
				Name: "missing",
				Info: "important",
			},
		},
		"negative height": {
			p: types.Plan{
				Name:   "minus",
				Height: -12345,
			},
		},
	}

	for name, tc := range cases {
		tc := tc // copy to local variable for scopelint
		t.Run(name, func(t *testing.T) {
			err := tc.p.ValidateBasic()
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}

}

func TestShouldExecute(t *testing.T) {
	cases := map[string]struct {
		p         types.Plan
		ctxTime   time.Time
		ctxHeight int64
		expected  bool
	}{
		"past time": {
			p: types.Plan{
				Name: "do-good",
				Info: "some text here",
				Time: mustParseTime("2019-07-08T11:33:55Z"),
			},
			ctxTime:   mustParseTime("2019-07-08T11:32:00Z"),
			ctxHeight: 100000,
			expected:  false,
		},
		"on time": {
			p: types.Plan{
				Name: "do-good",
				Time: mustParseTime("2019-07-08T11:33:55Z"),
			},
			ctxTime:   mustParseTime("2019-07-08T11:33:55Z"),
			ctxHeight: 100000,
			expected:  true,
		},
		"future time": {
			p: types.Plan{
				Name: "do-good",
				Time: mustParseTime("2019-07-08T11:33:55Z"),
			},
			ctxTime:   mustParseTime("2019-07-08T11:33:57Z"),
			ctxHeight: 100000,
			expected:  true,
		},
		"past height": {
			p: types.Plan{
				Name:   "do-good",
				Height: 1234,
			},
			ctxTime:   mustParseTime("2019-07-08T11:32:00Z"),
			ctxHeight: 1000,
			expected:  false,
		},
		"on height": {
			p: types.Plan{
				Name:   "do-good",
				Height: 1234,
			},
			ctxTime:   mustParseTime("2019-07-08T11:32:00Z"),
			ctxHeight: 1234,
			expected:  true,
		},
		"future height": {
			p: types.Plan{
				Name:   "do-good",
				Height: 1234,
			},
			ctxTime:   mustParseTime("2019-07-08T11:32:00Z"),
			ctxHeight: 1235,
			expected:  true,
		},
	}

	for name, tc := range cases {
		tc := tc // copy to local variable for scopelint
		t.Run(name, func(t *testing.T) {
			ctx := sdk.NewContext(nil, ocproto.Header{Height: tc.ctxHeight, Time: tc.ctxTime}, false, log.NewNopLogger())
			should := tc.p.ShouldExecute(ctx)
			assert.Equal(t, tc.expected, should)
		})
	}
}
