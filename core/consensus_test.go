package core

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/ICBNetwork/icb-network-blockchain-modules/messages"
	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"
	"github.com/stretchr/testify/assert"
)

// generateNodeAddresses generates dummy node addresses
func generateNodeAddresses(count int) [][]byte {
	addresses := make([][]byte, count)

	for index := range addresses {
		addresses[index] = []byte(fmt.Sprintf("node %d", index))
	}

	return addresses
}

// buildBasicPreprepareMessage builds a simple preprepare message
func buildBasicPreprepareMessage(
	proposal []byte,
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
				Proposal:     proposal,
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
	proposal []byte,
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
				LastPreparedProposedBlock: proposal,
				LatestPreparedCertificate: certificate,
			},
		},
	}
}

// TestConsensus_ValidFlow tests the following scenario:
// N = 4
//
// - Node 0 is the proposer for block 1, round 0
// - Node 0 proposes a valid block B
// - All nodes go through the consensus states to insert the valid block B
func TestConsensus_ValidFlow(t *testing.T) {
	t.Parallel()

	var multicastFn func(message *proto.Message)

	proposal := []byte("proposal")
	proposalHash := []byte("proposal hash")
	committedSeal := []byte("seal")
	numNodes := 4
	nodes := generateNodeAddresses(numNodes)
	insertedBlocks := make([][]byte, numNodes)

	// commonTransportCallback is the common method modification
	// required for Transport, for all nodes
	commonTransportCallback := func(transport *mockTransport) {
		transport.multicastFn = func(message *proto.Message) {
			multicastFn(message)
		}
	}

	// commonBackendCallback is the common method modification required
	// for the Backend, for all nodes
	commonBackendCallback := func(backend *mockBackend, nodeIndex int) {
		// Make sure the quorum function requires all nodes
		backend.quorumFn = func(_ uint64) uint64 {
			return uint64(numNodes)
		}

		// Make sure the node ID is properly relayed
		backend.idFn = func() []byte {
			return nodes[nodeIndex]
		}

		// Make sure the only proposer is node 0
		backend.isProposerFn = func(from []byte, _ uint64, _ uint64) bool {
			return bytes.Equal(from, nodes[0])
		}

		// Make sure the proposal is valid if it matches what node 0 proposed
		backend.isValidBlockFn = func(newProposal []byte) bool {
			return bytes.Equal(newProposal, proposal)
		}

		// Make sure the proposal hash matches
		backend.isValidProposalHashFn = func(p []byte, ph []byte) bool {
			return bytes.Equal(p, proposal) && bytes.Equal(ph, proposalHash)
		}

		// Make sure the preprepare message is built correctly
		backend.buildPrePrepareMessageFn = func(
			proposal []byte,
			certificate *proto.RoundChangeCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicPreprepareMessage(
				proposal,
				proposalHash,
				certificate,
				nodes[nodeIndex],
				view)
		}

		// Make sure the prepare message is built correctly
		backend.buildPrepareMessageFn = func(proposal []byte, view *proto.View) *proto.Message {
			return buildBasicPrepareMessage(proposalHash, nodes[nodeIndex], view)
		}

		// Make sure the commit message is built correctly
		backend.buildCommitMessageFn = func(proposal []byte, view *proto.View) *proto.Message {
			return buildBasicCommitMessage(proposalHash, committedSeal, nodes[nodeIndex], view)
		}

		// Make sure the round change message is built correctly
		backend.buildRoundChangeMessageFn = func(
			proposal []byte,
			certificate *proto.PreparedCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicRoundChangeMessage(proposal, certificate, view, nodes[nodeIndex])
		}

		// Make sure the inserted proposal is noted
		backend.insertBlockFn = func(proposal []byte, _ []*messages.CommittedSeal) {
			insertedBlocks[nodeIndex] = proposal
		}
	}

	var (
		backendCallbackMap = map[int]backendConfigCallback{
			0: func(backend *mockBackend) {
				// Execute the common backend setup
				commonBackendCallback(backend, 0)

				// Set the proposal creation method for node 0, since
				// they are the proposer
				backend.buildProposalFn = func(u uint64) []byte {
					return proposal
				}
			},
			1: func(backend *mockBackend) {
				commonBackendCallback(backend, 1)
			},
			2: func(backend *mockBackend) {
				commonBackendCallback(backend, 2)
			},
			3: func(backend *mockBackend) {
				commonBackendCallback(backend, 3)
			},
		}
		transportCallbackMap = map[int]transportConfigCallback{
			0: commonTransportCallback,
			1: commonTransportCallback,
			2: commonTransportCallback,
			3: commonTransportCallback,
		}
	)

	// Create the mock cluster
	cluster := newMockCluster(
		numNodes,
		backendCallbackMap,
		nil,
		transportCallbackMap,
	)

	// Set the multicast callback to relay the message
	// to the entire cluster
	multicastFn = func(message *proto.Message) {
		cluster.pushMessage(message)
	}

	// Start the main run loops
	cluster.runSequence(0)

	// Wait until the main run loops finish
	cluster.stop()

	// Make sure the inserted blocks match what node 0 proposed
	for _, block := range insertedBlocks {
		assert.True(t, bytes.Equal(block, proposal))
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
	numNodes := 4
	nodes := generateNodeAddresses(numNodes)
	insertedBlocks := make([][]byte, numNodes)

	// commonTransportCallback is the common method modification
	// required for Transport, for all nodes
	commonTransportCallback := func(transport *mockTransport) {
		transport.multicastFn = func(message *proto.Message) {
			multicastFn(message)
		}
	}

	maxFaulty := func(nodeCount int) uint64 {
		return uint64((nodeCount - 1) / 3)
	}

	quorumOptimal := func(numNodes int) uint64 {
		if maxFaulty(numNodes) == 0 {
			return uint64(numNodes)
		}

		return uint64(math.Ceil(2 * float64(numNodes) / 3))
	}

	// commonBackendCallback is the common method modification required
	// for the Backend, for all nodes
	commonBackendCallback := func(backend *mockBackend, nodeIndex int) {
		// Make sure the quorum function is Quorum optimal
		backend.quorumFn = func(_ uint64) uint64 {
			return quorumOptimal(numNodes)
		}

		// Make sure the allowed faulty nodes function is accurate
		backend.maximumFaultyNodesFn = func() uint64 {
			return maxFaulty(numNodes)
		}

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
		backend.isValidBlockFn = func(newProposal []byte) bool {
			// Node 1 is the proposer for round 1,
			// and their proposal is the only one that's valid
			return bytes.Equal(newProposal, proposals[1])
		}

		// Make sure the proposal hash matches
		backend.isValidProposalHashFn = func(proposal []byte, proposalHash []byte) bool {
			if bytes.Equal(proposal, proposals[0]) {
				return bytes.Equal(proposalHash, proposalHashes[0])
			}

			return bytes.Equal(proposalHash, proposalHashes[1])
		}

		// Make sure the preprepare message is built correctly
		backend.buildPrePrepareMessageFn = func(
			proposal []byte,
			certificate *proto.RoundChangeCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicPreprepareMessage(
				proposal,
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
			proposal []byte,
			certificate *proto.PreparedCertificate,
			view *proto.View,
		) *proto.Message {
			return buildBasicRoundChangeMessage(proposal, certificate, view, nodes[nodeIndex])
		}

		// Make sure the inserted proposal is noted
		backend.insertBlockFn = func(proposal []byte, _ []*messages.CommittedSeal) {
			insertedBlocks[nodeIndex] = proposal
		}
	}

	var (
		backendCallbackMap = map[int]backendConfigCallback{
			0: func(backend *mockBackend) {
				commonBackendCallback(backend, 0)

				backend.buildProposalFn = func(_ uint64) []byte {
					return proposals[0]
				}
			},
			1: func(backend *mockBackend) {
				commonBackendCallback(backend, 1)

				backend.buildProposalFn = func(_ uint64) []byte {
					return proposals[1]
				}
			},
			2: func(backend *mockBackend) {
				commonBackendCallback(backend, 2)
			},
			3: func(backend *mockBackend) {
				commonBackendCallback(backend, 3)
			},
		}
		transportCallbackMap = map[int]transportConfigCallback{
			0: commonTransportCallback,
			1: commonTransportCallback,
			2: commonTransportCallback,
			3: commonTransportCallback,
		}
	)

	// Create the mock cluster
	cluster := newMockCluster(
		numNodes,
		backendCallbackMap,
		nil,
		transportCallbackMap,
	)

	// Set the base timeout to be lower than usual
	cluster.setBaseTimeout(2 * time.Second)

	// Set the multicast callback to relay the message
	// to the entire cluster
	multicastFn = func(message *proto.Message) {
		cluster.pushMessage(message)
	}

	// Start the main run loops
	cluster.runSequence(1)

	// Wait until the main run loops finish
	cluster.stop()

	// Make sure the nodes switched to the new round
	assert.True(t, cluster.areAllNodesOnRound(1))

	// Make sure the inserted blocks match what node 1 proposed
	for _, block := range insertedBlocks {
		assert.True(t, bytes.Equal(block, proposals[1]))
	}
}
