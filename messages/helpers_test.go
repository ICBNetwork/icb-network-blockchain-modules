package messages

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"
)

func TestMessages_ExtractCommittedSeals(t *testing.T) {
	t.Parallel()

	var (
		committedSeal = []byte("committed seal")
	)

	createCommitMessage := func(signer string) *proto.Message {
		return &proto.Message{
			Type: proto.MessageType_COMMIT,
			Payload: &proto.Message_CommitData{
				CommitData: &proto.CommitMessage{
					CommittedSeal: committedSeal,
				},
			},
			From: []byte(signer),
		}
	}

	createWrongMessage := func(signer string, msgType proto.MessageType) *proto.Message {
		return &proto.Message{
			Type: msgType,
		}
	}

	createCommittedSeal := func(from string) *CommittedSeal {
		return &CommittedSeal{
			Signer:    []byte(from),
			Signature: committedSeal,
		}
	}

	tests := []struct {
		name     string
		messages []*proto.Message
		expected []*CommittedSeal
		err      error
	}{
		{
			name: "contains only valid COMMIT messages",
			messages: []*proto.Message{
				createCommitMessage("signer1"),
				createCommitMessage("signer2"),
			},
			expected: []*CommittedSeal{
				createCommittedSeal("signer1"),
				createCommittedSeal("signer2"),
			},
			err: nil,
		},
		{
			name: "contains wrong type messages",
			messages: []*proto.Message{
				createCommitMessage("signer1"),
				createWrongMessage("signer2", proto.MessageType_PREPREPARE),
			},
			expected: nil,
			err:      ErrWrongCommitMessageType,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			seals, err := ExtractCommittedSeals(test.messages)

			assert.Equal(t, test.expected, seals)
			assert.Equal(t, test.err, err)
		})
	}
}

func TestMessages_ExtractCommitHash(t *testing.T) {
	t.Parallel()

	commitHash := []byte("commit hash")

	testTable := []struct {
		name               string
		expectedCommitHash []byte
		message            *proto.Message
	}{
		{
			"valid message",
			commitHash,
			&proto.Message{
				Type: proto.MessageType_COMMIT,
				Payload: &proto.Message_CommitData{
					CommitData: &proto.CommitMessage{
						ProposalHash: commitHash,
					},
				},
			},
		},
		{
			"invalid message",
			nil,
			&proto.Message{
				Type: proto.MessageType_PREPARE,
			},
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.expectedCommitHash,
				ExtractCommitHash(testCase.message),
			)
		})
	}
}

func TestMessages_ExtractProposal(t *testing.T) {
	t.Parallel()

	proposal := &proto.Proposal{}

	testTable := []struct {
		name             string
		expectedProposal *proto.Proposal
		message          *proto.Message
	}{
		{
			"valid message",
			proposal,
			&proto.Message{
				Type: proto.MessageType_PREPREPARE,
				Payload: &proto.Message_PreprepareData{
					PreprepareData: &proto.PrePrepareMessage{
						Proposal: proposal,
					},
				},
			},
		},
		{
			"invalid message",
			nil,
			&proto.Message{
				Type: proto.MessageType_PREPARE,
			},
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.expectedProposal,
				ExtractProposal(testCase.message),
			)
		})
	}
}

func TestMessages_ExtractProposalHash(t *testing.T) {
	t.Parallel()

	proposalHash := []byte("proposal hash")

	testTable := []struct {
		name                 string
		expectedProposalHash []byte
		message              *proto.Message
	}{
		{
			"valid message",
			proposalHash,
			&proto.Message{
				Type: proto.MessageType_PREPREPARE,
				Payload: &proto.Message_PreprepareData{
					PreprepareData: &proto.PrePrepareMessage{
						ProposalHash: proposalHash,
					},
				},
			},
		},
		{
			"invalid message",
			nil,
			&proto.Message{
				Type: proto.MessageType_PREPARE,
			},
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.expectedProposalHash,
				ExtractProposalHash(testCase.message),
			)
		})
	}
}

func TestMessages_ExtractRCC(t *testing.T) {
	t.Parallel()

	rcc := &proto.RoundChangeCertificate{
		RoundChangeMessages: nil,
	}

	testTable := []struct {
		name        string
		expectedRCC *proto.RoundChangeCertificate
		message     *proto.Message
	}{
		{
			"valid message",
			rcc,
			&proto.Message{
				Type: proto.MessageType_PREPREPARE,
				Payload: &proto.Message_PreprepareData{
					PreprepareData: &proto.PrePrepareMessage{
						Certificate: rcc,
					},
				},
			},
		},
		{
			"invalid message",
			nil,
			&proto.Message{
				Type: proto.MessageType_PREPARE,
			},
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.expectedRCC,
				ExtractRoundChangeCertificate(testCase.message),
			)
		})
	}
}

func TestMessages_ExtractPrepareHash(t *testing.T) {
	t.Parallel()

	prepareHash := []byte("prepare hash")

	testTable := []struct {
		name                string
		expectedPrepareHash []byte
		message             *proto.Message
	}{
		{
			"valid message",
			prepareHash,
			&proto.Message{
				Type: proto.MessageType_PREPARE,
				Payload: &proto.Message_PrepareData{
					PrepareData: &proto.PrepareMessage{
						ProposalHash: prepareHash,
					},
				},
			},
		},
		{
			"invalid message",
			nil,
			&proto.Message{
				Type: proto.MessageType_PREPREPARE,
			},
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.expectedPrepareHash,
				ExtractPrepareHash(testCase.message),
			)
		})
	}
}

func TestMessages_ExtractLatestPC(t *testing.T) {
	t.Parallel()

	latestPC := &proto.PreparedCertificate{
		ProposalMessage: nil,
		PrepareMessages: nil,
	}

	testTable := []struct {
		name       string
		expectedPC *proto.PreparedCertificate
		message    *proto.Message
	}{
		{
			"valid message",
			latestPC,
			&proto.Message{
				Type: proto.MessageType_ROUND_CHANGE,
				Payload: &proto.Message_RoundChangeData{
					RoundChangeData: &proto.RoundChangeMessage{
						LatestPreparedCertificate: latestPC,
					},
				},
			},
		},
		{
			"invalid message",
			nil,
			&proto.Message{
				Type: proto.MessageType_PREPREPARE,
			},
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.expectedPC,
				ExtractLatestPC(testCase.message),
			)
		})
	}
}

func TestMessages_ExtractLPPB(t *testing.T) {
	t.Parallel()

	lastPreparedProposal := &proto.Proposal{}

	testTable := []struct {
		name         string
		expectedLPPB *proto.Proposal
		message      *proto.Message
	}{
		{
			"valid message",
			lastPreparedProposal,
			&proto.Message{
				Type: proto.MessageType_ROUND_CHANGE,
				Payload: &proto.Message_RoundChangeData{
					RoundChangeData: &proto.RoundChangeMessage{
						LastPreparedProposal: lastPreparedProposal,
					},
				},
			},
		},
		{
			"invalid message",
			nil,
			&proto.Message{
				Type: proto.MessageType_PREPREPARE,
			},
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.expectedLPPB,
				ExtractLastPreparedProposal(testCase.message),
			)
		})
	}
}

func TestMessages_HasUniqueSenders(t *testing.T) {
	t.Parallel()

	testTable := []struct {
		name      string
		messages  []*proto.Message
		hasUnique bool
	}{
		{
			"empty messages",
			nil,
			false,
		},
		{
			"non unique senders",
			[]*proto.Message{
				{
					From: []byte("node 1"),
				},
				{
					From: []byte("node 1"),
				},
			},
			false,
		},
		{
			"unique senders",
			[]*proto.Message{
				{
					From: []byte("node 1"),
				},
				{
					From: []byte("node 2"),
				},
			},
			true,
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.hasUnique,
				HasUniqueSenders(testCase.messages),
			)
		})
	}
}

func TestMessages_HaveSameProposalHash(t *testing.T) {
	t.Parallel()

	proposalHash := []byte("proposal hash")

	testTable := []struct {
		name     string
		messages []*proto.Message
		haveSame bool
	}{
		{
			"empty messages",
			nil,
			false,
		},
		{
			"invalid message type",
			[]*proto.Message{
				{
					Type: proto.MessageType_ROUND_CHANGE,
				},
			},
			false,
		},
		{
			"hash mismatch",
			[]*proto.Message{
				{
					Type: proto.MessageType_PREPREPARE,
					Payload: &proto.Message_PreprepareData{
						PreprepareData: &proto.PrePrepareMessage{
							ProposalHash: proposalHash,
						},
					},
				},
				{
					Type: proto.MessageType_PREPARE,
					Payload: &proto.Message_PrepareData{
						PrepareData: &proto.PrepareMessage{
							ProposalHash: []byte("differing hash"),
						},
					},
				},
			},
			false,
		},
		{
			"hash match",
			[]*proto.Message{
				{
					Type: proto.MessageType_PREPREPARE,
					Payload: &proto.Message_PreprepareData{
						PreprepareData: &proto.PrePrepareMessage{
							ProposalHash: proposalHash,
						},
					},
				},
				{
					Type: proto.MessageType_PREPARE,
					Payload: &proto.Message_PrepareData{
						PrepareData: &proto.PrepareMessage{
							ProposalHash: proposalHash,
						},
					},
				},
			},
			true,
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.haveSame,
				HaveSameProposalHash(testCase.messages),
			)
		})
	}
}

func TestMessages_AllHaveLowerRond(t *testing.T) {
	t.Parallel()

	round := uint64(1)

	testTable := []struct {
		name      string
		messages  []*proto.Message
		round     uint64
		haveLower bool
	}{
		{
			"empty messages",
			nil,
			0,
			false,
		},
		{
			"not same lower round",
			[]*proto.Message{
				{
					View: &proto.View{
						Height: 0,
						Round:  round,
					},
				},
				{
					View: &proto.View{
						Height: 0,
						Round:  round + 1,
					},
				},
			},
			round,
			false,
		},
		{
			"same higher round",
			[]*proto.Message{
				{
					View: &proto.View{
						Height: 0,
						Round:  round + 1,
					},
				},
				{
					View: &proto.View{
						Height: 0,
						Round:  round + 1,
					},
				},
			},
			round,
			false,
		},
		{
			"lower round match",
			[]*proto.Message{
				{
					View: &proto.View{
						Height: 0,
						Round:  round,
					},
				},
				{
					View: &proto.View{
						Height: 0,
						Round:  round,
					},
				},
			},
			2,
			true,
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.haveLower,
				AllHaveLowerRound(
					testCase.messages,
					testCase.round,
				),
			)
		})
	}
}

func TestMessages_AllHaveSameHeight(t *testing.T) {
	t.Parallel()

	height := uint64(1)

	testTable := []struct {
		name     string
		messages []*proto.Message
		haveSame bool
	}{
		{
			"empty messages",
			nil,
			false,
		},
		{
			"not same height",
			[]*proto.Message{
				{
					View: &proto.View{
						Height: height - 1,
					},
				},
				{
					View: &proto.View{
						Height: height,
					},
				},
			},
			false,
		},
		{
			"same height",
			[]*proto.Message{
				{
					View: &proto.View{
						Height: height,
					},
				},
				{
					View: &proto.View{
						Height: height,
					},
				},
			},
			true,
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				testCase.haveSame,
				AllHaveSameHeight(
					testCase.messages,
					height,
				),
			)
		})
	}
}
