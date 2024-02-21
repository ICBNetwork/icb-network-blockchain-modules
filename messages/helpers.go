package messages

import (
	"bytes"
	"errors"

	"github.com/ICBNetwork/icb-network-blockchain-modules/v2/messages/proto"
)

var (
	// ErrWrongCommitMessageType is an error indicating wrong type in commit messages
	ErrWrongCommitMessageType = errors.New("wrong type message is included in COMMIT messages")
)

// CommittedSeal Validator proof of signing a committed proposal
type CommittedSeal struct {
	Signer    []byte
	Signature []byte
}

// ExtractCommittedSeals extracts the committed seals from the passed in messages
func ExtractCommittedSeals(commitMessages []*proto.Message) ([]*CommittedSeal, error) {
	committedSeals := make([]*CommittedSeal, 0)

	for _, commitMessage := range commitMessages {
		if commitMessage.Type != proto.MessageType_COMMIT {
			// safe check
			return nil, ErrWrongCommitMessageType
		}

		committedSeals = append(committedSeals, ExtractCommittedSeal(commitMessage))
	}

	return committedSeals, nil
}

// ExtractCommittedSeal extracts the committed seal from the passed in message
func ExtractCommittedSeal(commitMessage *proto.Message) *CommittedSeal {
	commitData, _ := commitMessage.Payload.(*proto.Message_CommitData)

	return &CommittedSeal{
		Signer:    commitMessage.From,
		Signature: commitData.CommitData.CommittedSeal,
	}
}

// ExtractCommitHash extracts the commit proposal hash from the passed in message
func ExtractCommitHash(commitMessage *proto.Message) []byte {
	if commitMessage.Type != proto.MessageType_COMMIT {
		return nil
	}

	commitData, _ := commitMessage.Payload.(*proto.Message_CommitData)

	return commitData.CommitData.ProposalHash
}

// ExtractProposal extracts the (rawData,r) proposal from the passed in message
func ExtractProposal(proposalMessage *proto.Message) *proto.Proposal {
	if proposalMessage.Type != proto.MessageType_PREPREPARE {
		return nil
	}

	preprepareData, _ := proposalMessage.Payload.(*proto.Message_PreprepareData)

	return preprepareData.PreprepareData.Proposal
}

// ExtractProposalHash extracts the proposal hash from the passed in message
func ExtractProposalHash(proposalMessage *proto.Message) []byte {
	if proposalMessage.Type != proto.MessageType_PREPREPARE {
		return nil
	}

	preprepareData, _ := proposalMessage.Payload.(*proto.Message_PreprepareData)

	return preprepareData.PreprepareData.ProposalHash
}

// ExtractRoundChangeCertificate extracts the RCC from the passed in message
func ExtractRoundChangeCertificate(proposalMessage *proto.Message) *proto.RoundChangeCertificate {
	if proposalMessage.Type != proto.MessageType_PREPREPARE {
		return nil
	}

	preprepareData, _ := proposalMessage.Payload.(*proto.Message_PreprepareData)

	return preprepareData.PreprepareData.Certificate
}

// ExtractPrepareHash extracts the prepare proposal hash from the passed in message
func ExtractPrepareHash(prepareMessage *proto.Message) []byte {
	if prepareMessage.Type != proto.MessageType_PREPARE {
		return nil
	}

	prepareData, _ := prepareMessage.Payload.(*proto.Message_PrepareData)

	return prepareData.PrepareData.ProposalHash
}

// ExtractLatestPC extracts the latest PC from the passed in message
func ExtractLatestPC(roundChangeMessage *proto.Message) *proto.PreparedCertificate {
	if roundChangeMessage.Type != proto.MessageType_ROUND_CHANGE {
		return nil
	}

	rcData, _ := roundChangeMessage.Payload.(*proto.Message_RoundChangeData)

	return rcData.RoundChangeData.LatestPreparedCertificate
}

// ExtractLastPreparedProposal extracts the latest prepared proposal from the passed in message
func ExtractLastPreparedProposal(roundChangeMessage *proto.Message) *proto.Proposal {
	if roundChangeMessage.Type != proto.MessageType_ROUND_CHANGE {
		return nil
	}

	rcData, _ := roundChangeMessage.Payload.(*proto.Message_RoundChangeData)

	return rcData.RoundChangeData.LastPreparedProposal
}

// HasUniqueSenders checks if the messages have unique senders
func HasUniqueSenders(messages []*proto.Message) bool {
	if len(messages) < 1 {
		return false
	}

	senderMap := make(map[string]struct{})

	for _, message := range messages {
		key := string(message.From)
		if _, exists := senderMap[key]; exists {
			return false
		}

		senderMap[key] = struct{}{}
	}

	return true
}

// HaveSameProposalHash checks if the messages have the same proposal hash
func HaveSameProposalHash(messages []*proto.Message) bool {
	if len(messages) < 1 {
		return false
	}

	var hash []byte

	for _, message := range messages {
		var extractedHash []byte

		switch message.Type {
		case proto.MessageType_PREPREPARE:
			extractedHash = ExtractProposalHash(message)
		case proto.MessageType_PREPARE:
			extractedHash = ExtractPrepareHash(message)
		case proto.MessageType_COMMIT, proto.MessageType_ROUND_CHANGE:
			return false
		default:
			return false
		}

		if hash == nil {
			// No previous hash for comparison,
			// set the first one as the reference, as
			// all of them need to be the same anyway
			hash = extractedHash
		}

		if !bytes.Equal(hash, extractedHash) {
			return false
		}
	}

	return true
}

// AllHaveLowerRound checks if all messages have the same round
func AllHaveLowerRound(messages []*proto.Message, round uint64) bool {
	if len(messages) < 1 {
		return false
	}

	for _, message := range messages {
		if message.View.Round >= round {
			return false
		}
	}

	return true
}

// AllHaveSameRound checks if all messages have the same round
func AllHaveSameRound(messages []*proto.Message) bool {
	if len(messages) < 1 {
		return false
	}

	var round = messages[0].View.Round

	for _, message := range messages {
		if message.View.Round != round {
			return false
		}
	}

	return true
}

// AllHaveSameHeight checks if all messages have the same height
func AllHaveSameHeight(messages []*proto.Message, height uint64) bool {
	if len(messages) < 1 {
		return false
	}

	for _, message := range messages {
		if message.View.Height != height {
			return false
		}
	}

	return true
}
