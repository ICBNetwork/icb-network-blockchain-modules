package core

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ICBNetwork/icb-network-blockchain-modules/messages"
	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"
)

// generateNodeAddresses generates dummy node addresses
func generateNodeAddresses(count uint64) [][]byte {
	addresses := make([][]byte, count)

	for index := range addresses {
		addresses[index] = []byte(fmt.Sprintf("node %d", index))
	}

	return addresses
}

// buildBasicPreprepareMessage builds a simple preprepare message
func buildBasicPreprepareMessage(
	rawProposal []byte,
	proposalHash []byte,
	certificate *proto.RoundChangeCertificate,
	from []byte,
	view *proto.View,
) *proto.Message {
	return &proto.Message{
		View: view,
		From: from,
		Type: proto.MessageType_PREPREPARE,
		Payload: &proto.Message_PreprepareData{
			PreprepareData: &proto.PrePrepareMessage{
				Proposal: &proto.Proposal{
					RawProposal: rawProposal,
					Round:       view.Round,
				},
				Certificate:  certificate,
				ProposalHash: proposalHash,
			},
		},
	}
}

// buildBasicPrepareMessage builds a simple prepare message
func buildBasicPrepareMessage(
	proposalHash,
	from []byte,
	view *proto.View,
) *proto.Message {
	return &proto.Message{
		View: view,
		From: from,
		Type: proto.MessageType_PREPARE,
		Payload: &proto.Message_PrepareData{
			PrepareData: &proto.PrepareMessage{
				ProposalHash: proposalHash,
			},
		},
	}
}

// buildBasicCommitMessage builds a simple commit message
func buildBasicCommitMessage(
	proposalHash,
	committedSeal,
	from []byte,
	view *proto.View,
) *proto.Message {
	return &proto.Message{
		View: view,
		From: from,
		Type: proto.MessageType_COMMIT,
		Payload: &proto.Message_CommitData{
			CommitData: &proto.CommitMessage{
				ProposalHash:  proposalHash,
				CommittedSeal: committedSeal,
			},
		},
	}
}

// buildBasicRoundChangeMessage builds a simple round change message
func buildBasicRoundChangeMessage(
	proposal *proto.Proposal,
	certificate *proto.PreparedCertificate,
	view *proto.View,
	from []byte,
) *proto.Message {
	return &proto.Message{
		View: view,
		From: from,
		Type: proto.MessageType_ROUND_CHANGE,
		Payload: &proto.Message_RoundChangeData{
			RoundChangeData: &proto.RoundChangeMessage{
				LastPreparedProposal:      proposal,
				LatestPreparedCertificate: certificate,
			},
		},
	}
}

// maxFaulty returns the maximum number of allowed
// faulty nodes
func maxFaulty(nodeCount uint64) uint64 {
	return (nodeCount - 1) / 3
}

// quorum returns the minimum number of
// required nodes to reach quorum
func quorum(numNodes uint64) uint64 {
	switch maxFaulty(numNodes) {
	case 0:
		return numNodes
	default:
		return uint64(math.Ceil(2 * float64(numNodes) / 3))
	}
}

// TestConsensus_ValidFlow tests the following scenario:
// N = 4
//
// - Node 0 is the proposer for height 1, round 0
// - Node 0 proposes a valid block B
// - All nodes go through the consensus states to insert the valid block B
func TestConsensus_ValidFlow(t *testing.T) {
	t.Parallel()

	var (
		multicastFn func(message *proto.Message)

		numNodes       = uint64(4)
		nodes          = generateNodeAddresses(numNodes)
		insertedBlocks = make([][]byte, numNodes)
	)

	// commonTransportCallback is the common method modification
	// required for Transport, for all nodes
	commonTransportCallback := func(transport *mockTransport, _ int) {
		transport.multicastFn = func(message *proto.Message) {
			multicastFn(message)
		}
	}

	// commonBackendCallback is the common method modification required
	// for the Backend, for all nodes
	commonBackendCallback := func(backend *mockBackend, nodeIndex int) {
		// Make sure the quorum function requires all nodes
		backend.getVotingPowerFn = testCommonGetVotingPowertFn(nodes)

		// Make sure the node ID is properly relayed
		backend.idFn = func() []byte {
			return nodes[nodeIndex]
		}

		// Make sure the only proposer is node 0
		backend.isProposerFn = func(from []byte, _ uint64, _ uint64) bool {
			return bytes.Equal(from, nodes[0])
		}

		// Make sure the proposal is valid if it matches what node 0 proposed
		backend.isValidProposalFn = func(rawProposal []byte) bool {
			return bytes.Equal(rawProposal, correctRoundMessage.proposal.GetRawProposal())
		}

		// Make sure the proposal hash matches
		backend.isValidProposalHashFn = func(proposal *proto.Proposal, proposalHash []byte) bool {
			return bytes.Equal(proposal.GetRawProposal(), correctRoundMessage.proposal.GetRawProposal()) &&
				proposal.Round == correctRoundMessage.proposal.Round &&
				bytes.Equal(proposalHash, correctRoundMessage.hash)
		}

		// Make sure the preprepare message is built correctly
		backend.buildPrePrepareMessageFn = func(
			rawProposal []byte,
			certificate *proto.RoundChangeCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicPreprepareMessage(
				rawProposal,
				correctRoundMessage.hash,
				certificate,
				nodes[nodeIndex],
				view)
		}

		// Make sure the prepare message is built correctly
		backend.buildPrepareMessageFn = func(proposal []byte, view *proto.View) *proto.Message {
			return buildBasicPrepareMessage(correctRoundMessage.hash, nodes[nodeIndex], view)
		}

		// Make sure the commit message is built correctly
		backend.buildCommitMessageFn = func(proposal []byte, view *proto.View) *proto.Message {
			return buildBasicCommitMessage(correctRoundMessage.hash, correctRoundMessage.seal, nodes[nodeIndex], view)
		}

		// Make sure the round change message is built correctly
		backend.buildRoundChangeMessageFn = func(
			proposal *proto.Proposal,
			certificate *proto.PreparedCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicRoundChangeMessage(proposal, certificate, view, nodes[nodeIndex])
		}

		// Make sure the inserted proposal is noted
		backend.insertProposalFn = func(proposal *proto.Proposal, _ []*messages.CommittedSeal) {
			insertedBlocks[nodeIndex] = proposal.RawProposal
		}

		// Set the proposal creation method
		backend.buildProposalFn = func(_ uint64) []byte {
			return correctRoundMessage.proposal.GetRawProposal()
		}
	}

	// Create the mock cluster
	cluster := newMockCluster(
		numNodes,
		commonBackendCallback,
		nil,
		commonTransportCallback,
	)

	// Set the multicast callback to relay the message
	// to the entire cluster
	multicastFn = func(message *proto.Message) {
		cluster.pushMessage(message)
	}

	// Start the main run loops
	cluster.runSequence(0)

	// Wait until the main run loops finish
	cluster.awaitCompletion()

	// Make sure the inserted blocks match what node 0 proposed
	for _, block := range insertedBlocks {
		assert.True(t, bytes.Equal(block, correctRoundMessage.proposal.GetRawProposal()))
	}
}

// TestConsensus_InvalidBlock tests the following scenario:
// N = 4
//
// - Node 0 is the proposer for block 1, round 0
// - Node 0 proposes an invalid block B
// - Other nodes should verify that the block is invalid
// - All nodes should move to round 1, and start a new consensus round
// - Node 1 is the proposer for block 1, round 1
// - Node 1 proposes a valid block B'
// - All nodes go through the consensus states to insert the valid block B'
func TestConsensus_InvalidBlock(t *testing.T) {
	t.Parallel()

	var multicastFn func(message *proto.Message)

	proposals := [][]byte{
		[]byte("proposal 1"), // proposed by node 0
		[]byte("proposal 2"), // proposed by node 1
	}

	proposalHashes := [][]byte{
		[]byte("proposal hash 1"), // for proposal 1
		[]byte("proposal hash 2"), // for proposal 2
	}
	committedSeal := []byte("seal")
	numNodes := uint64(4)
	nodes := generateNodeAddresses(numNodes)
	insertedBlocks := make([][]byte, numNodes)

	// commonTransportCallback is the common method modification
	// required for Transport, for all nodes
	commonTransportCallback := func(transport *mockTransport, _ int) {
		transport.multicastFn = func(message *proto.Message) {
			multicastFn(message)
		}
	}

	// commonBackendCallback is the common method modification required
	// for the Backend, for all nodes
	commonBackendCallback := func(backend *mockBackend, nodeIndex int) {
		// Make sure the quorum function requires all nodes
		backend.getVotingPowerFn = testCommonGetVotingPowertFn(nodes)

		// Make sure the node ID is properly relayed
		backend.idFn = func() []byte {
			return nodes[nodeIndex]
		}

		// Make sure the only proposer is node 0
		backend.isProposerFn = func(from []byte, _ uint64, round uint64) bool {
			// Node 0 is the proposer for round 0
			// Node 1 is the proposer for round 1
			return bytes.Equal(from, nodes[round])
		}

		// Make sure the proposal is valid if it matches what node 0 proposed
		backend.isValidProposalFn = func(newProposal []byte) bool {
			// Node 1 is the proposer for round 1,
			// and their proposal is the only one that's valid
			return bytes.Equal(newProposal, proposals[1])
		}

		// Make sure the proposal hash matches
		backend.isValidProposalHashFn = func(proposal *proto.Proposal, proposalHash []byte) bool {
			if bytes.Equal(proposal.RawProposal, proposals[0]) {
				return bytes.Equal(proposalHash, proposalHashes[0])
			}

			return bytes.Equal(proposalHash, proposalHashes[1])
		}

		// Make sure the preprepare message is built correctly
		backend.buildPrePrepareMessageFn = func(
			rawProposal []byte,
			certificate *proto.RoundChangeCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicPreprepareMessage(
				rawProposal,
				proposalHashes[view.Round],
				certificate,
				nodes[nodeIndex],
				view,
			)
		}

		// Make sure the prepare message is built correctly
		backend.buildPrepareMessageFn = func(proposal []byte, view *proto.View) *proto.Message {
			return buildBasicPrepareMessage(proposalHashes[view.Round], nodes[nodeIndex], view)
		}

		// Make sure the commit message is built correctly
		backend.buildCommitMessageFn = func(proposal []byte, view *proto.View) *proto.Message {
			return buildBasicCommitMessage(proposalHashes[view.Round], committedSeal, nodes[nodeIndex], view)
		}

		// Make sure the round change message is built correctly
		backend.buildRoundChangeMessageFn = func(
			proposal *proto.Proposal,
			certificate *proto.PreparedCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicRoundChangeMessage(proposal, certificate, view, nodes[nodeIndex])
		}

		// Make sure the inserted proposal is noted
		backend.insertProposalFn = func(proposal *proto.Proposal, _ []*messages.CommittedSeal) {
			insertedBlocks[nodeIndex] = proposal.RawProposal
		}

		// Build proposal function
		backend.buildProposalFn = func(_ uint64) []byte {
			return proposals[nodeIndex]
		}
	}

	// Create the mock cluster
	cluster := newMockCluster(
		numNodes,
		commonBackendCallback,
		nil,
		commonTransportCallback,
	)

	// Set the base timeout to be lower than usual
	cluster.setBaseTimeout(2 * time.Second)

	// Set the multicast callback to relay the message
	// to the entire cluster
	multicastFn = cluster.pushMessage

	// Start the main run loops
	cluster.runSequence(1)

	// Wait until the main run loops finish
	cluster.awaitCompletion()

	// Make sure the nodes switched to the new round
	assert.True(t, cluster.areAllNodesOnRound(1))

	// Make sure the inserted blocks match what node 1 proposed
	for _, block := range insertedBlocks {
		assert.True(t, bytes.Equal(block, proposals[1]))
	}
}
