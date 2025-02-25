package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/spf13/pflag"

	govutils "github.com/line/lbm-sdk/x/gov/client/utils"
)

func (p proposal) validate() error {
	if p.Type == "" {
		return fmt.Errorf("proposal type is required")
	}

	if p.Title == "" {
		return fmt.Errorf("proposal title is required")
	}

	if p.Description == "" {
		return fmt.Errorf("proposal description is required")
	}
	return nil
}

func parseSubmitProposalFlags(fs *pflag.FlagSet) (*proposal, error) {
	proposal := &proposal{}
	proposalFile, _ := fs.GetString(FlagProposal)

	if proposalFile == "" {
		proposalType, _ := fs.GetString(FlagProposalType)
		title, _ := fs.GetString(FlagTitle)
		description, _ := fs.GetString(FlagDescription)
		if proposalType == "" && title == "" && description == "" {
			return nil, fmt.Errorf("one of the --proposal or (--title, --description and --type) flags are required")
		}

		proposal.Title, _ = fs.GetString(FlagTitle)
		proposal.Description, _ = fs.GetString(FlagDescription)
		proposal.Type = govutils.NormalizeProposalType(proposalType)
		proposal.Deposit, _ = fs.GetString(FlagDeposit)
		if err := proposal.validate(); err != nil {
			return nil, err
		}

		return proposal, nil
	}

	for _, flag := range ProposalFlags {
		if v, _ := fs.GetString(flag); v != "" {
			return nil, fmt.Errorf("--%s flag provided alongside --proposal, which is a noop", flag)
		}
	}

	contents, err := ioutil.ReadFile(proposalFile)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(contents, proposal)
	if err != nil {
		return nil, err
	}

	return proposal, nil
}
