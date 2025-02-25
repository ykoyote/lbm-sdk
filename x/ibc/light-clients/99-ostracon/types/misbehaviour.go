package types

import (
	"bytes"
	"time"

	ocproto "github.com/line/ostracon/proto/ostracon/types"
	octypes "github.com/line/ostracon/types"

	sdkerrors "github.com/line/lbm-sdk/types/errors"
	clienttypes "github.com/line/lbm-sdk/x/ibc/core/02-client/types"
	host "github.com/line/lbm-sdk/x/ibc/core/24-host"
	"github.com/line/lbm-sdk/x/ibc/core/exported"
)

var _ exported.Misbehaviour = &Misbehaviour{}

// NewMisbehaviour creates a new Misbehaviour instance.
func NewMisbehaviour(clientID string, header1, header2 *Header) *Misbehaviour {
	return &Misbehaviour{
		ClientId: clientID,
		Header1:  header1,
		Header2:  header2,
	}
}

// ClientType is Ostracon light client
func (misbehaviour Misbehaviour) ClientType() string {
	return exported.Ostracon
}

// GetClientID returns the ID of the client that committed a misbehaviour.
func (misbehaviour Misbehaviour) GetClientID() string {
	return misbehaviour.ClientId
}

// GetHeight returns the height at which misbehaviour occurred
//
// NOTE: assumes that misbehaviour headers have the same height
func (misbehaviour Misbehaviour) GetHeight() exported.Height {
	return misbehaviour.Header1.GetHeight()
}

// GetTime returns the timestamp at which misbehaviour occurred. It uses the
// maximum value from both headers to prevent producing an invalid header outside
// of the misbehaviour age range.
func (misbehaviour Misbehaviour) GetTime() time.Time {
	t1, t2 := misbehaviour.Header1.GetTime(), misbehaviour.Header2.GetTime()
	if t1.After(t2) {
		return t1
	}
	return t2
}

// ValidateBasic implements Misbehaviour interface
func (misbehaviour Misbehaviour) ValidateBasic() error {
	if misbehaviour.Header1 == nil {
		return sdkerrors.Wrap(ErrInvalidHeader, "misbehaviour Header1 cannot be nil")
	}
	if misbehaviour.Header2 == nil {
		return sdkerrors.Wrap(ErrInvalidHeader, "misbehaviour Header2 cannot be nil")
	}
	if misbehaviour.Header1.TrustedHeight.RevisionHeight == 0 {
		return sdkerrors.Wrapf(ErrInvalidHeaderHeight, "misbehaviour Header1 cannot have zero revision height")
	}
	if misbehaviour.Header2.TrustedHeight.RevisionHeight == 0 {
		return sdkerrors.Wrapf(ErrInvalidHeaderHeight, "misbehaviour Header2 cannot have zero revision height")
	}
	if misbehaviour.Header1.TrustedValidators == nil {
		return sdkerrors.Wrap(ErrInvalidValidatorSet, "trusted validator set in Header1 cannot be empty")
	}
	if misbehaviour.Header1.TrustedVoters == nil {
		return sdkerrors.Wrap(ErrInvalidVoterSet, "trusted voter set in Header1 cannot be empty")
	}
	if misbehaviour.Header2.TrustedValidators == nil {
		return sdkerrors.Wrap(ErrInvalidValidatorSet, "trusted validator set in Header2 cannot be empty")
	}
	if misbehaviour.Header2.TrustedVoters == nil {
		return sdkerrors.Wrap(ErrInvalidVoterSet, "trusted voter set in Header2 cannot be empty")
	}
	if misbehaviour.Header1.Header.ChainID != misbehaviour.Header2.Header.ChainID {
		return sdkerrors.Wrap(clienttypes.ErrInvalidMisbehaviour, "headers must have identical chainIDs")
	}

	if err := host.ClientIdentifierValidator(misbehaviour.ClientId); err != nil {
		return sdkerrors.Wrap(err, "misbehaviour client ID is invalid")
	}

	// ValidateBasic on both validators
	if err := misbehaviour.Header1.ValidateBasic(); err != nil {
		return sdkerrors.Wrap(
			clienttypes.ErrInvalidMisbehaviour,
			sdkerrors.Wrap(err, "header 1 failed validation").Error(),
		)
	}
	if err := misbehaviour.Header2.ValidateBasic(); err != nil {
		return sdkerrors.Wrap(
			clienttypes.ErrInvalidMisbehaviour,
			sdkerrors.Wrap(err, "header 2 failed validation").Error(),
		)
	}
	// Ensure that Heights are the same
	if misbehaviour.Header1.GetHeight() != misbehaviour.Header2.GetHeight() {
		return sdkerrors.Wrapf(clienttypes.ErrInvalidMisbehaviour, "headers in misbehaviour are on different heights (%d ≠ %d)", misbehaviour.Header1.GetHeight(), misbehaviour.Header2.GetHeight())
	}

	blockID1, err := octypes.BlockIDFromProto(&misbehaviour.Header1.SignedHeader.Commit.BlockID)
	if err != nil {
		return sdkerrors.Wrap(err, "invalid block ID from header 1 in misbehaviour")
	}
	blockID2, err := octypes.BlockIDFromProto(&misbehaviour.Header2.SignedHeader.Commit.BlockID)
	if err != nil {
		return sdkerrors.Wrap(err, "invalid block ID from header 2 in misbehaviour")
	}

	// Ensure that Commit Hashes are different
	if bytes.Equal(blockID1.Hash, blockID2.Hash) {
		return sdkerrors.Wrap(clienttypes.ErrInvalidMisbehaviour, "headers block hashes are equal")
	}
	if err := validCommit(misbehaviour.Header1.Header.ChainID, *blockID1,
		misbehaviour.Header1.Commit, misbehaviour.Header1.VoterSet); err != nil {
		return err
	}
	if err := validCommit(misbehaviour.Header2.Header.ChainID, *blockID2,
		misbehaviour.Header2.Commit, misbehaviour.Header2.VoterSet); err != nil {
		return err
	}
	return nil
}

// validCommit checks if the given commit is a valid commit from the passed-in validatorset
func validCommit(chainID string, blockID octypes.BlockID, commit *ocproto.Commit, voterSet *ocproto.VoterSet) (err error) {
	ocCommit, err := octypes.CommitFromProto(commit)
	if err != nil {
		return sdkerrors.Wrap(err, "commit is not ostracon commit type")
	}
	ocVoterSet, err := octypes.VoterSetFromProto(voterSet)
	if err != nil {
		return sdkerrors.Wrap(err, "validator set is not tendermint voter set type")
	}

	if err := ocVoterSet.VerifyCommit(chainID, blockID, ocCommit.Height, ocCommit); err != nil {
		return sdkerrors.Wrap(clienttypes.ErrInvalidMisbehaviour, "voter set did not commit to header")
	}

	return nil
}
