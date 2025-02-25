package testutil

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/suite"

	ostcli "github.com/line/ostracon/libs/cli"

	clitestutil "github.com/line/lbm-sdk/testutil/cli"
	"github.com/line/lbm-sdk/testutil/network"
	sdk "github.com/line/lbm-sdk/types"
	"github.com/line/lbm-sdk/x/gov/client/cli"
	"github.com/line/lbm-sdk/x/gov/types"
)

type DepositTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network
	fees    string
}

func NewDepositTestSuite(cfg network.Config) *DepositTestSuite {
	return &DepositTestSuite{cfg: cfg}
}

func (s *DepositTestSuite) SetupSuite() {
	s.T().Log("setting up test suite")

	s.network = network.New(s.T(), s.cfg)

	_, err := s.network.WaitForHeight(1)
	s.Require().NoError(err)
	s.fees = sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(20))).String()

}

func (s *DepositTestSuite) TearDownSuite() {
	s.T().Log("tearing down test suite")
	s.network.Cleanup()
}

func (s *DepositTestSuite) TestQueryDepositsInitialDeposit() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx
	initialDeposit := sdk.NewCoin(s.cfg.BondDenom, types.DefaultMinDepositTokens.Sub(sdk.NewInt(20))).String()

	// create a proposal with deposit
	_, err := MsgSubmitProposal(val.ClientCtx, val.Address.String(),
		"Text Proposal 1", "Where is the title!?", types.ProposalTypeText,
		fmt.Sprintf("--%s=%s", cli.FlagDeposit, initialDeposit))
	s.Require().NoError(err)

	// deposit more amount
	_, err = MsgDeposit(clientCtx, val.Address.String(), "1", sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(50)).String())
	s.Require().NoError(err)

	// waiting for voting period to end
	time.Sleep(20 * time.Second)

	// query deposit & verify initial deposit
	deposit := s.queryDeposit(val, "1", false)
	s.Require().Equal(deposit.Amount.String(), initialDeposit)

	// query deposits
	deposits := s.queryDeposits(val, "1", false)
	s.Require().Equal(len(deposits), 2)
	// verify initial deposit
	s.Require().Equal(deposits[0].Amount.String(), initialDeposit)
}

func (s *DepositTestSuite) TestQueryDepositsWithoutInitialDeposit() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx

	// create a proposal without deposit
	_, err := MsgSubmitProposal(val.ClientCtx, val.Address.String(),
		"Text Proposal 2", "Where is the title!?", types.ProposalTypeText)
	s.Require().NoError(err)

	// deposit amount
	_, err = MsgDeposit(clientCtx, val.Address.String(), "2", sdk.NewCoin(s.cfg.BondDenom, types.DefaultMinDepositTokens.Add(sdk.NewInt(50))).String())
	s.Require().NoError(err)

	// waiting for voting period to end
	time.Sleep(20 * time.Second)

	// query deposit
	deposit := s.queryDeposit(val, "2", false)
	s.Require().Equal(deposit.Amount.String(), sdk.NewCoin(s.cfg.BondDenom, types.DefaultMinDepositTokens.Add(sdk.NewInt(50))).String())

	// query deposits
	deposits := s.queryDeposits(val, "2", false)
	s.Require().Equal(len(deposits), 1)
	// verify initial deposit
	s.Require().Equal(deposits[0].Amount.String(), sdk.NewCoin(s.cfg.BondDenom, types.DefaultMinDepositTokens.Add(sdk.NewInt(50))).String())
}

func (s *DepositTestSuite) TestQueryProposalNotEnoughDeposits() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx
	initialDeposit := sdk.NewCoin(s.cfg.BondDenom, types.DefaultMinDepositTokens.Sub(sdk.NewInt(2000))).String()

	// create a proposal with deposit
	_, err := MsgSubmitProposal(val.ClientCtx, val.Address.String(),
		"Text Proposal 3", "Where is the title!?", types.ProposalTypeText,
		fmt.Sprintf("--%s=%s", cli.FlagDeposit, initialDeposit))
	s.Require().NoError(err)

	// query proposal
	args := []string{"3", fmt.Sprintf("--%s=json", ostcli.OutputFlag)}
	cmd := cli.GetCmdQueryProposal()
	_, err = clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	// waiting for deposit period to end
	time.Sleep(20 * time.Second)

	// query proposal
	_, err = clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "proposal 3 doesn't exist")
}

func (s *DepositTestSuite) TestRejectedProposalDeposits() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx
	initialDeposit := sdk.NewCoin(s.cfg.BondDenom, types.DefaultMinDepositTokens)

	// create a proposal with deposit
	_, err := MsgSubmitProposal(clientCtx, val.Address.String(),
		"Text Proposal 4", "Where is the title!?", types.ProposalTypeText,
		fmt.Sprintf("--%s=%s", cli.FlagDeposit, initialDeposit))
	s.Require().NoError(err)

	// query deposits
	var deposits types.QueryDepositsResponse
	args := []string{"4", fmt.Sprintf("--%s=json", ostcli.OutputFlag)}
	cmd := cli.GetCmdQueryDeposits()
	out, err := clitestutil.ExecTestCLICmd(val.ClientCtx, cmd, args)
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.LegacyAmino.UnmarshalJSON(out.Bytes(), &deposits))
	s.Require().Equal(len(deposits.Deposits), 1)
	// verify initial deposit
	s.Require().Equal(deposits.Deposits[0].Amount.String(), sdk.NewCoin(s.cfg.BondDenom, types.DefaultMinDepositTokens).String())

	// vote
	_, err = MsgVote(clientCtx, val.Address.String(), "4", "no")
	s.Require().NoError(err)

	time.Sleep(20 * time.Second)

	args = []string{"4", fmt.Sprintf("--%s=json", ostcli.OutputFlag)}
	cmd = cli.GetCmdQueryProposal()
	_, err = clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	// query deposits
	depositsRes := s.queryDeposits(val, "4", false)
	s.Require().Equal(len(depositsRes), 1)
	// verify initial deposit
	s.Require().Equal(depositsRes[0].Amount.String(), initialDeposit.String())

}

func (s *DepositTestSuite) queryDeposits(val *network.Validator, proposalID string, exceptErr bool) types.Deposits {
	args := []string{proposalID, fmt.Sprintf("--%s=json", ostcli.OutputFlag)}
	var depositsRes types.Deposits
	cmd := cli.GetCmdQueryDeposits()
	out, err := clitestutil.ExecTestCLICmd(val.ClientCtx, cmd, args)
	if exceptErr {
		s.Require().Error(err)
		return nil
	}
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.LegacyAmino.UnmarshalJSON(out.Bytes(), &depositsRes))
	return depositsRes
}

func (s *DepositTestSuite) queryDeposit(val *network.Validator, proposalID string, exceptErr bool) *types.Deposit {
	args := []string{proposalID, val.Address.String(), fmt.Sprintf("--%s=json", ostcli.OutputFlag)}
	var depositRes types.Deposit
	cmd := cli.GetCmdQueryDeposit()
	out, err := clitestutil.ExecTestCLICmd(val.ClientCtx, cmd, args)
	if exceptErr {
		s.Require().Error(err)
		return nil
	}
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.LegacyAmino.UnmarshalJSON(out.Bytes(), &depositRes))
	return &depositRes
}
