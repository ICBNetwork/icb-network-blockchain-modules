// Package core implements IBFT consensus
// backend.go defines interfaces of backend, that performs detailed procedure rather than consensus
package core

import (
	"github.com/ICBNetwork/icb-network-blockchain-modules/messages"
	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"
)

// MessageConstructor defines a message constructor interface
// All constructed messages must be signed by a validator for the whole message
type MessageConstructor interface {
	// BuildPrePrepareMessage builds a PREPREPARE message based on the passed in view and proposal
	BuildPrePrepareMessage(
		rawProposal []byte,
		certificate *proto.RoundChangeCertificate,
		view *proto.View,
	) *proto.Message

	// BuildPrepareMessage builds a PREPARE message based on the passed in view and proposal hash
	BuildPrepareMessage(proposalHash []byte, view *proto.View) *proto.Message

	// BuildCommitMessage builds a COMMIT message based on the passed in view and proposal hash
	// Must create a committed seal for proposal hash and include it into the message
	BuildCommitMessage(proposalHash []byte, view *proto.View) *proto.Message

	// BuildRoundChangeMessage builds a ROUND_CHANGE message based on the passed in view,
	// latest prepared proposal, and latest prepared certificate
	BuildRoundChangeMessage(
		proposal *proto.Proposal,
		certificate *proto.PreparedCertificate,
		view *proto.View,
	) *proto.Message
}

// Verifier defines the verifier interface
type Verifier interface {
	// IsValidProposal checks if the proposal is valid
	IsValidProposal(rawProposal []byte) bool

	// IsValidValidator checks if a signature in message is signed by sender
	// Must check the following things:
	// (1) recover the signature and the signer matches from address in message
	// (2) the signer address is one of the validators at the height in message
	IsValidValidator(msg *proto.Message) bool

	// IsProposer checks if the passed in ID is the Proposer for current view (sequence, round)
	IsProposer(id []byte, height, round uint64) bool

	// IsValidProposalHash checks if the hash matches the proposal
	IsValidProposalHash(proposal *proto.Proposal, hash []byte) bool

	// IsValidCommittedSeal checks
	// if signature for proposal hash in committed seal is signed by a validator
	IsValidCommittedSeal(proposalHash []byte, committedSeal *messages.CommittedSeal) bool
}

// Backend defines an interface all backend implementations
// need to implement
type Backend interface {
	MessageConstructor
	Verifier
	ValidatorBackend

	// BuildProposal builds a new proposal for the given view (height and round)
	BuildProposal(view *proto.View) []byte

	// InsertProposal inserts a proposal with the specified committed seals
	// the reason why we are including round here is because a single committedSeal
	// has signed the tuple of (rawProposal, round)
	InsertProposal(proposal *proto.Proposal, committedSeals []*messages.CommittedSeal)

	// ID returns the validator's ID
	ID() []byte
}
