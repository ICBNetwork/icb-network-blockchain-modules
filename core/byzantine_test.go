package core

import (
	"bytes"
	"testing"
	"time"

	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"

	"github.com/stretchr/testify/assert"
)

func TestByzantineBehaviour(t *testing.T) {
	t.Parallel()

	//nolint:dupl
	t.Run("malicious hash in proposal", func(t *testing.T) {
		t.Parallel()

		cluster := newCluster(
			6,
			func(c *cluster) {
				for _, node := range c.nodes {
					currentNode := node

					backendBuilder := mockBackendBuilder{}
					backendBuilder.withProposerFn(createForcedRCProposerFn(c))
					backendBuilder.withIDFn(currentNode.addr)
					backendBuilder.withBuildPrePrepareMessageFn(createBadHashPrePrepareMessageFn(currentNode))
					backendBuilder.withGetVotingPowerFn(testCommonGetVotingPowertFnForNodes(c.nodes))

					node.core = NewIBFT(
						mockLogger{},
						backendBuilder.build(currentNode),
						&mockTransport{multicastFn: c.gossip},
					)
				}
			},
		)

		err := cluster.progressToHeight(20*time.Second, 1)
		assert.NoError(t, err, "unable to reach height: %w", err)
		assert.Equal(t, uint64(1), cluster.latestHeight)

		cluster.makeNByzantine(int(cluster.maxFaulty()))
		assert.NoError(t, cluster.progressToHeight(20*time.Second, 2))
		assert.Equal(t, uint64(2), cluster.latestHeight)
	})

	t.Run("malicious hash in prepare", func(t *testing.T) {
		t.Parallel()

		cluster := newCluster(
			6,
			func(c *cluster) {
				for _, node := range c.nodes {
					currentNode := node

					backendBuilder := mockBackendBuilder{}
					backendBuilder.withProposerFn(c.isProposer)
					backendBuilder.withIDFn(currentNode.addr)
					backendBuilder.withBuildPrepareMessageFn(createBadHashPrepareMessageFn(currentNode))
					backendBuilder.withGetVotingPowerFn(testCommonGetVotingPowertFnForNodes(c.nodes))

					node.core = NewIBFT(
						mockLogger{},
						backendBuilder.build(currentNode),
						&mockTransport{multicastFn: c.gossip},
					)
				}
			},
		)

		err := cluster.progressToHeight(10*time.Second, 1)
		assert.NoError(t, err, "unable to reach height: %w", err)
		assert.Equal(t, uint64(1), cluster.latestHeight)

		cluster.makeNByzantine(int(cluster.maxFaulty()))
		assert.NoError(t, cluster.progressToHeight(10*time.Second, 2))
		assert.Equal(t, uint64(2), cluster.latestHeight)
	})

	//nolint:dupl
	t.Run("malicious +1 round in proposal", func(t *testing.T) {
		t.Parallel()

		cluster := newCluster(
			6,
			func(c *cluster) {
				for _, node := range c.nodes {
					currentNode := node

					backendBuilder := mockBackendBuilder{}
					backendBuilder.withProposerFn(createForcedRCProposerFn(c))
					backendBuilder.withIDFn(currentNode.addr)
					backendBuilder.withBuildPrePrepareMessageFn(createBadRoundPrePrepareMessageFn(currentNode))
					backendBuilder.withGetVotingPowerFn(testCommonGetVotingPowertFnForNodes(c.nodes))

					node.core = NewIBFT(
						mockLogger{},
						backendBuilder.build(currentNode),
						&mockTransport{multicastFn: c.gossip},
					)
				}
			},
		)

		err := cluster.progressToHeight(20*time.Second, 1)
		assert.NoError(t, err, "unable to reach height: %w", err)
		assert.Equal(t, uint64(1), cluster.latestHeight)

		// Max tolerant byzantine
		cluster.makeNByzantine(int(cluster.maxFaulty()))
		assert.NoError(t, cluster.progressToHeight(40*time.Second, 2))
		assert.Equal(t, uint64(2), cluster.latestHeight)
	})

	//nolint:dupl
	t.Run("malicious +1 round in rcc", func(t *testing.T) {
		t.Parallel()

		cluster := newCluster(
			6,
			func(c *cluster) {
				for _, node := range c.nodes {
					currentNode := node

					backendBuilder := mockBackendBuilder{}
					backendBuilder.withProposerFn(createForcedRCProposerFn(c))
					backendBuilder.withIDFn(currentNode.addr)
					backendBuilder.withBuildRoundChangeMessageFn(createBadRoundRoundChangeFn(currentNode))
					backendBuilder.withGetVotingPowerFn(testCommonGetVotingPowertFnForNodes(c.nodes))

					node.core = NewIBFT(
						mockLogger{},
						backendBuilder.build(currentNode),
						&mockTransport{multicastFn: c.gossip},
					)
				}
			},
		)

		err := cluster.progressToHeight(20*time.Second, 1)
		assert.NoError(t, err, "unable to reach height: %w", err)
		assert.Equal(t, uint64(1), cluster.latestHeight)

		cluster.makeNByzantine(int(cluster.maxFaulty()))
		assert.NoError(t, cluster.progressToHeight(30*time.Second, 2))
		assert.Equal(t, uint64(2), cluster.latestHeight)
	})

	//nolint:dupl
	t.Run("malicious +1 round in rcc and in proposal", func(t *testing.T) {
		t.Parallel()

		cluster := newCluster(
			6,
			func(c *cluster) {
				for _, node := range c.nodes {
					currentNode := node

					backendBuilder := mockBackendBuilder{}
					backendBuilder.withProposerFn(createForcedRCProposerFn(c))
					backendBuilder.withIDFn(currentNode.addr)
					backendBuilder.withBuildPrePrepareMessageFn(createBadRoundPrePrepareMessageFn(currentNode))
					backendBuilder.withBuildRoundChangeMessageFn(createBadRoundRoundChangeFn(currentNode))
					backendBuilder.withGetVotingPowerFn(testCommonGetVotingPowertFnForNodes(c.nodes))

					node.core = NewIBFT(
						mockLogger{},
						backendBuilder.build(currentNode),
						&mockTransport{multicastFn: c.gossip},
					)
				}
			},
		)

		err := cluster.progressToHeight(20*time.Second, 1)
		assert.NoError(t, err, "unable to reach height: %w", err)
		assert.Equal(t, uint64(1), cluster.latestHeight)

		cluster.makeNByzantine(int(cluster.maxFaulty()))
		assert.NoError(t, cluster.progressToHeight(30*time.Second, 2))
		assert.Equal(t, uint64(2), cluster.latestHeight)
	})

	//nolint:dupl
	t.Run("malicious +1 round in rcc and bad hash in proposal", func(t *testing.T) {
		t.Parallel()

		cluster := newCluster(
			6,
			func(c *cluster) {
				for _, node := range c.nodes {
					currentNode := node

					backendBuilder := mockBackendBuilder{}
					backendBuilder.withProposerFn(createForcedRCProposerFn(c))
					backendBuilder.withIDFn(currentNode.addr)
					backendBuilder.withBuildPrePrepareMessageFn(createBadHashPrePrepareMessageFn(currentNode))
					backendBuilder.withBuildRoundChangeMessageFn(createBadRoundRoundChangeFn(currentNode))
					backendBuilder.withGetVotingPowerFn(testCommonGetVotingPowertFnForNodes(c.nodes))

					node.core = NewIBFT(
						mockLogger{},
						backendBuilder.build(currentNode),
						&mockTransport{multicastFn: c.gossip},
					)
				}
			},
		)

		err := cluster.progressToHeight(20*time.Second, 1)
		assert.NoError(t, err, "unable to reach height: %w", err)
		assert.Equal(t, uint64(1), cluster.latestHeight)

		cluster.makeNByzantine(int(cluster.maxFaulty()))
		assert.NoError(t, cluster.progressToHeight(30*time.Second, 2))
		assert.Equal(t, uint64(2), cluster.latestHeight)
	})

	//nolint:dupl
	t.Run("malicious +1 round in rcc and bad hash in prepare", func(t *testing.T) {
		t.Parallel()

		cluster := newCluster(
			6,
			func(c *cluster) {
				for _, node := range c.nodes {
					currentNode := node

					backendBuilder := mockBackendBuilder{}
					backendBuilder.withProposerFn(createForcedRCProposerFn(c))
					backendBuilder.withIDFn(currentNode.addr)
					backendBuilder.withBuildPrepareMessageFn(createBadHashPrepareMessageFn(currentNode))
					backendBuilder.withBuildRoundChangeMessageFn(createBadRoundRoundChangeFn(currentNode))
					backendBuilder.withGetVotingPowerFn(testCommonGetVotingPowertFnForNodes(c.nodes))

					node.core = NewIBFT(
						mockLogger{},
						backendBuilder.build(currentNode),
						&mockTransport{multicastFn: c.gossip},
					)
				}
			},
		)

		err := cluster.progressToHeight(20*time.Second, 1)
		assert.NoError(t, err, "unable to reach height: %w", err)
		assert.Equal(t, uint64(1), cluster.latestHeight)

		cluster.makeNByzantine(int(cluster.maxFaulty()))
		assert.NoError(t, cluster.progressToHeight(30*time.Second, 2))
		assert.Equal(t, uint64(2), cluster.latestHeight)
	})

	//nolint:dupl
	t.Run("malicious +1 round in rcc and bad commit seal", func(t *testing.T) {
		t.Parallel()

		cluster := newCluster(
			6,
			func(c *cluster) {
				for _, node := range c.nodes {
					currentNode := node

					backendBuilder := mockBackendBuilder{}
					backendBuilder.withProposerFn(createForcedRCProposerFn(c))
					backendBuilder.withIDFn(currentNode.addr)
					backendBuilder.withBuildCommitMessageFn(createBadCommitMessageFn(currentNode))
					backendBuilder.withBuildRoundChangeMessageFn(createBadRoundRoundChangeFn(currentNode))
					backendBuilder.withGetVotingPowerFn(testCommonGetVotingPowertFnForNodes(c.nodes))

					node.core = NewIBFT(
						mockLogger{},
						backendBuilder.build(currentNode),
						&mockTransport{multicastFn: c.gossip},
					)
				}
			},
		)

		err := cluster.progressToHeight(20*time.Second, 1)
		assert.NoError(t, err, "unable to reach height: %w", err)
		assert.Equal(t, uint64(1), cluster.latestHeight)

		cluster.makeNByzantine(int(cluster.maxFaulty()))
		assert.NoError(t, cluster.progressToHeight(30*time.Second, 2))
		assert.Equal(t, uint64(2), cluster.latestHeight)
	})
}

func createBadRoundRoundChangeFn(node *node) buildRoundChangeMessageDelegate {
	return func(proposal *proto.Proposal,
		rcc *proto.PreparedCertificate,
		view *proto.View) *proto.Message {
		if node.byzantine {
			view.Round++
		}

		return buildBasicRoundChangeMessage(
			proposal,
			rcc,
			view,
			node.address,
		)
	}
}

func createBadRoundPrePrepareMessageFn(node *node) buildPrePrepareMessageDelegate {
	return func(
		proposal []byte,
		certificate *proto.RoundChangeCertificate,
		view *proto.View,
	) *proto.Message {
		if node.byzantine {
			view.Round++
		}

		return buildBasicPreprepareMessage(
			proposal,
			validProposalHash,
			certificate,
			node.address,
			view,
		)
	}
}

func createBadHashPrePrepareMessageFn(node *node) buildPrePrepareMessageDelegate {
	return func(proposal []byte,
		rcc *proto.RoundChangeCertificate,
		view *proto.View) *proto.Message {
		proposalHash := validProposalHash
		if node.byzantine {
			proposalHash = []byte("invalid proposal hash")
		}

		return buildBasicPreprepareMessage(
			proposal,
			proposalHash,
			rcc,
			node.address,
			view,
		)
	}
}

func createBadHashPrepareMessageFn(node *node) buildPrepareMessageDelegate {
	return func(_ []byte, view *proto.View) *proto.Message {
		proposalHash := validProposalHash
		if node.byzantine {
			proposalHash = []byte("invalid proposal hash")
		}

		return buildBasicPrepareMessage(
			proposalHash,
			node.address,
			view,
		)
	}
}

func createForcedRCProposerFn(c *cluster) isProposerDelegate {
	return func(from []byte, height uint64, round uint64) bool {
		if round == 0 {
			return false
		}

		return bytes.Equal(
			from,
			c.addresses()[int(round)%len(c.addresses())],
		)
	}
}

func createBadCommitMessageFn(node *node) buildCommitMessageDelegate {
	return func(_ []byte, view *proto.View) *proto.Message {
		committedSeal := validCommittedSeal
		if node.byzantine {
			committedSeal = []byte("invalid committed seal")
		}

		return buildBasicCommitMessage(
			validProposalHash,
			committedSeal,
			node.address,
			view,
		)
	}
}

type mockBackendBuilder struct {
	isProposerFn isProposerDelegate

	idFn idDelegate

	buildPrePrepareMessageFn  buildPrePrepareMessageDelegate
	buildPrepareMessageFn     buildPrepareMessageDelegate
	buildCommitMessageFn      buildCommitMessageDelegate
	buildRoundChangeMessageFn buildRoundChangeMessageDelegate

	getVotingPowerFn getVotingPowerDelegate
}

func (b *mockBackendBuilder) withProposerFn(f isProposerDelegate) {
	b.isProposerFn = f
}

func (b *mockBackendBuilder) withBuildPrePrepareMessageFn(f buildPrePrepareMessageDelegate) {
	b.buildPrePrepareMessageFn = f
}

func (b *mockBackendBuilder) withBuildPrepareMessageFn(f buildPrepareMessageDelegate) {
	b.buildPrepareMessageFn = f
}

func (b *mockBackendBuilder) withBuildCommitMessageFn(f buildCommitMessageDelegate) {
	b.buildCommitMessageFn = f
}

func (b *mockBackendBuilder) withBuildRoundChangeMessageFn(f buildRoundChangeMessageDelegate) {
	b.buildRoundChangeMessageFn = f
}

func (b *mockBackendBuilder) withIDFn(f idDelegate) {
	b.idFn = f
}

func (b *mockBackendBuilder) withGetVotingPowerFn(f getVotingPowerDelegate) {
	b.getVotingPowerFn = f
}

func (b *mockBackendBuilder) build(node *node) *mockBackend {
	if b.buildPrePrepareMessageFn == nil {
		b.buildPrePrepareMessageFn = node.buildPrePrepare
	}

	if b.buildPrepareMessageFn == nil {
		b.buildPrepareMessageFn = node.buildPrepare
	}

	if b.buildCommitMessageFn == nil {
		b.buildCommitMessageFn = node.buildCommit
	}

	if b.buildRoundChangeMessageFn == nil {
		b.buildRoundChangeMessageFn = node.buildRoundChange
	}

	return &mockBackend{
		isValidProposalFn:      isValidProposal,
		isValidProposalHashFn:  isValidProposalHash,
		IsValidValidatorFn:     nil,
		isValidCommittedSealFn: nil,
		isProposerFn:           b.isProposerFn,
		idFn:                   b.idFn,

		buildProposalFn:           buildValidEthereumBlock,
		buildPrePrepareMessageFn:  b.buildPrePrepareMessageFn,
		buildPrepareMessageFn:     b.buildPrepareMessageFn,
		buildCommitMessageFn:      b.buildCommitMessageFn,
		buildRoundChangeMessageFn: b.buildRoundChangeMessageFn,
		insertProposalFn:          nil,
		getVotingPowerFn:          b.getVotingPowerFn,
	}
}
