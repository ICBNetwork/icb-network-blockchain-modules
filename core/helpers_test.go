package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"
)

/*	HELPERS */

var (
	validEthereumBlock = []byte("valid ethereum block")
	validProposalHash  = []byte("valid proposal hash")
	validCommittedSeal = []byte("valid committed seal")
)

func isValidProposal(newProposal []byte) bool {
	return bytes.Equal(newProposal, validEthereumBlock)
}

func buildValidEthereumBlock(_ uint64) []byte {
	return validEthereumBlock
}

func isValidProposalHash(proposal *proto.Proposal, proposalHash []byte) bool {
	return bytes.Equal(
		proposalHash,
		validProposalHash,
	)
}

type node struct {
	core      *IBFT
	address   []byte
	offline   bool
	faulty    bool
	byzantine bool
}

func (n *node) addr() []byte {
	return n.address
}

func (n *node) buildPrePrepare(
	rawProposal []byte,
	certificate *proto.RoundChangeCertificate,
	view *proto.View,
) *proto.Message {
	return buildBasicPreprepareMessage(
		rawProposal,
		validProposalHash,
		certificate,
		n.address,
		view,
	)
}

func (n *node) buildPrepare(
	_ []byte,
	view *proto.View,
) *proto.Message {
	return buildBasicPrepareMessage(
		validProposalHash,
		n.address,
		view,
	)
}

func (n *node) buildCommit(
	_ []byte,
	view *proto.View,
) *proto.Message {
	return buildBasicCommitMessage(
		validProposalHash,
		validCommittedSeal,
		n.address,
		view,
	)
}

func (n *node) buildRoundChange(
	proposal *proto.Proposal,
	certificate *proto.PreparedCertificate,
	view *proto.View,
) *proto.Message {
	return buildBasicRoundChangeMessage(
		proposal,
		certificate,
		view,
		n.address,
	)
}

func (n *node) runSequence(ctx context.Context, height uint64) {
	if n.offline {
		return
	}

	ctxSequence, cancel := context.WithCancel(ctx)
	defer cancel()

	n.core.RunSequence(ctxSequence, height)
}

type cluster struct {
	nodes []*node
	wg    sync.WaitGroup

	latestHeight uint64
}

func newCluster(num uint64, init func(*cluster)) *cluster {
	c := &cluster{
		nodes:        make([]*node, num),
		wg:           sync.WaitGroup{},
		latestHeight: 0,
	}

	for i, addr := range generateNodeAddresses(num) {
		c.nodes[i] = &node{address: addr}
	}

	init(c)

	return c
}

func (c *cluster) runGradualSequence(ctx context.Context, height uint64) {
	for nodeIndex, n := range c.nodes {
		c.wg.Add(1)

		go func(ctx context.Context, ordinal int, node *node) {
			// Start the main run loop for the node
			runDelay := ordinal * rand.Intn(1000)

			select {
			case <-ctx.Done():
			case <-time.After(time.Duration(runDelay) * time.Millisecond):
				node.core.RunSequence(ctx, height)
			}

			c.wg.Done()
		}(ctx, nodeIndex+1, n)
	}
}

func (c *cluster) runSequence(ctx context.Context, height uint64) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		for _, n := range c.nodes {
			c.wg.Add(1)

			go func(n *node) {
				defer c.wg.Done()

				n.runSequence(ctx, height)
			}(n)
		}

		c.wg.Wait()
	}()

	return done
}

func (c *cluster) runSequences(ctx context.Context, height uint64) error {
	for current := c.latestHeight + 1; current <= height; current++ {
		sequenceDone := c.runSequence(ctx, current)

		select {
		case <-sequenceDone:
			c.latestHeight = current
		case <-ctx.Done():
			// wait for the worker threads to return
			<-sequenceDone

			return errors.New("timeout")
		}
	}

	return nil
}

func (c *cluster) progressToHeight(timeout time.Duration, height uint64) error {
	if c.latestHeight >= height {
		panic("height already reached")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return c.runSequences(ctx, height)
}

func (c *cluster) addresses() [][]byte {
	addresses := make([][]byte, len(c.nodes))
	for i, node := range c.nodes {
		addresses[i] = node.address
	}

	return addresses
}

func (c *cluster) isProposer(
	from []byte,
	height,
	round uint64,
) bool {
	addrs := c.addresses()

	return bytes.Equal(
		from,
		addrs[int(height+round)%len(addrs)],
	)
}

func (c *cluster) gossip(msg *proto.Message) {
	for _, node := range c.nodes {
		node.core.AddMessage(msg)
	}
}

func (c *cluster) maxFaulty() uint64 {
	return (uint64(len(c.nodes)) - 1) / 3
}

func (c *cluster) makeNByzantine(num int) {
	for i := 0; i < num; i++ {
		c.nodes[i].byzantine = true
	}
}

func (c *cluster) makeNFaulty(num int) {
	for i := 0; i < num; i++ {
		c.nodes[i].faulty = true
	}
}

func (c *cluster) stopN(num int) {
	for i := 0; i < num; i++ {
		c.nodes[i].offline = true
	}
}

func (c *cluster) startN(num int) {
	for i := 0; i < num; i++ {
		c.nodes[i].offline = false
	}
}

func testCommonGetVotingPowertFn(nodes [][]byte) func(u uint64) (map[string]*big.Int, error) {
	return func(u uint64) (map[string]*big.Int, error) {
		result := map[string]*big.Int{}

		for _, x := range nodes {
			result[string(x)] = big.NewInt(1)
		}

		return result, nil
	}
}

func testCommonGetVotingPowertFnForCnt(nodesCnt uint64) func(u uint64) (map[string]*big.Int, error) {
	return func(u uint64) (map[string]*big.Int, error) {
		result := map[string]*big.Int{}

		for i := 0; i < int(nodesCnt); i++ {
			result[fmt.Sprintf("node %d", i)] = big.NewInt(1)
		}

		return result, nil
	}
}

func testCommonGetVotingPowertFnForNodes(nodes []*node) func(u uint64) (map[string]*big.Int, error) {
	return func(u uint64) (map[string]*big.Int, error) {
		result := map[string]*big.Int{}

		for _, x := range nodes {
			result[string(x.address)] = big.NewInt(1)
		}

		return result, nil
	}
}
