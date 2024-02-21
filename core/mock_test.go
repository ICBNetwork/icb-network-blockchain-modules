package core

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ICBNetwork/icb-network-blockchain-modules/v2/messages"
	"github.com/ICBNetwork/icb-network-blockchain-modules/v2/messages/proto"
)

const (
	testRoundTimeout = time.Second
)

var (
	correctRoundMessage = newCorrectRoundMessage(0)

	badRoundMessage = roundMessage{
		proposal: &proto.Proposal{
			RawProposal: []byte("bad"),
			Round:       100,
		},
		hash: []byte("bad proposal hash"),
		seal: []byte("bad seal"),
	}
)

func newCorrectRoundMessage(round uint64) roundMessage {
	return roundMessage{
		proposal: &proto.Proposal{
			RawProposal: validEthereumBlock,
			Round:       round,
		},
		hash: []byte("proposal hash"),
		seal: []byte("seal"),
	}
}

// Define delegation methods
type isValidBlockDelegate func([]byte) bool
type IsValidValidatorDelegate func(*proto.Message) bool
type isProposerDelegate func([]byte, uint64, uint64) bool
type buildEthereumBlockDelegate func(uint64) []byte
type isValidProposalHashDelegate func(*proto.Proposal, []byte) bool
type isValidCommittedSealDelegate func([]byte, *messages.CommittedSeal) bool

type buildPrePrepareMessageDelegate func(
	[]byte,
	*proto.RoundChangeCertificate,
	*proto.View,
) *proto.Message
type buildPrepareMessageDelegate func([]byte, *proto.View) *proto.Message
type buildCommitMessageDelegate func([]byte, *proto.View) *proto.Message
type buildRoundChangeMessageDelegate func(
	*proto.Proposal,
	*proto.PreparedCertificate,
	*proto.View,
) *proto.Message

type insertProposalDelegate func(*proto.Proposal, []*messages.CommittedSeal)
type idDelegate func() []byte
type getVotingPowerDelegate func(uint64) (map[string]*big.Int, error)

var _ Backend = &mockBackend{}

// mockBackend is the mock backend structure that is configurable
type mockBackend struct {
	isValidProposalFn      isValidBlockDelegate
	IsValidValidatorFn     IsValidValidatorDelegate
	isProposerFn           isProposerDelegate
	buildProposalFn        buildEthereumBlockDelegate
	isValidProposalHashFn  isValidProposalHashDelegate
	isValidCommittedSealFn isValidCommittedSealDelegate

	buildPrePrepareMessageFn  buildPrePrepareMessageDelegate
	buildPrepareMessageFn     buildPrepareMessageDelegate
	buildCommitMessageFn      buildCommitMessageDelegate
	buildRoundChangeMessageFn buildRoundChangeMessageDelegate
	insertProposalFn          insertProposalDelegate
	idFn                      idDelegate
	getVotingPowerFn          getVotingPowerDelegate
}

func (m mockBackend) ID() []byte {
	if m.idFn != nil {
		return m.idFn()
	}

	return nil
}

func (m mockBackend) InsertProposal(proposal *proto.Proposal, committedSeals []*messages.CommittedSeal) {
	if m.insertProposalFn != nil {
		m.insertProposalFn(proposal, committedSeals)
	}
}

func (m mockBackend) IsValidProposal(proposal []byte) bool {
	if m.isValidProposalFn != nil {
		return m.isValidProposalFn(proposal)
	}

	return true
}

func (m mockBackend) IsValidValidator(msg *proto.Message) bool {
	if m.IsValidValidatorFn != nil {
		return m.IsValidValidatorFn(msg)
	}

	return true
}

func (m mockBackend) IsProposer(id []byte, sequence, round uint64) bool {
	if m.isProposerFn != nil {
		return m.isProposerFn(id, sequence, round)
	}

	return false
}

func (m mockBackend) BuildProposal(view *proto.View) []byte {
	if m.buildProposalFn != nil {
		return m.buildProposalFn(view.Height)
	}

	return nil
}

func (m mockBackend) IsValidProposalHash(proposal *proto.Proposal, hash []byte) bool {
	if m.isValidProposalHashFn != nil {
		return m.isValidProposalHashFn(proposal, hash)
	}

	return true
}

func (m mockBackend) IsValidCommittedSeal(proposal []byte, committedSeal *messages.CommittedSeal) bool {
	if m.isValidCommittedSealFn != nil {
		return m.isValidCommittedSealFn(proposal, committedSeal)
	}

	return true
}

func (m mockBackend) BuildPrePrepareMessage(
	rawProposal []byte,
	certificate *proto.RoundChangeCertificate,
	view *proto.View,
) *proto.Message {
	if m.buildPrePrepareMessageFn != nil {
		return m.buildPrePrepareMessageFn(rawProposal, certificate, view)
	}

	return nil
}

func (m mockBackend) BuildPrepareMessage(proposal []byte, view *proto.View) *proto.Message {
	if m.buildPrepareMessageFn != nil {
		return m.buildPrepareMessageFn(proposal, view)
	}

	return nil
}

func (m mockBackend) BuildCommitMessage(proposalHash []byte, view *proto.View) *proto.Message {
	if m.buildCommitMessageFn != nil {
		return m.buildCommitMessageFn(proposalHash, view)
	}

	return nil
}

func (m mockBackend) BuildRoundChangeMessage(
	proposal *proto.Proposal,
	certificate *proto.PreparedCertificate,
	view *proto.View,
) *proto.Message {
	if m.buildRoundChangeMessageFn != nil {
		return m.buildRoundChangeMessageFn(proposal, certificate, view)
	}

	return &proto.Message{
		View: &proto.View{
			Height: view.Height,
			Round:  view.Round,
		},
		Type:    proto.MessageType_ROUND_CHANGE,
		Payload: nil,
	}
}

func (m mockBackend) GetVotingPowers(height uint64) (map[string]*big.Int, error) {
	if m.getVotingPowerFn != nil {
		return m.getVotingPowerFn(height)
	}

	return map[string]*big.Int{}, nil
}

// Define delegation methods
type multicastFnDelegate func(*proto.Message)

// mockTransport is the mock transport structure that is configurable
type mockTransport struct {
	multicastFn multicastFnDelegate
}

func (t mockTransport) Multicast(msg *proto.Message) {
	if t.multicastFn != nil {
		t.multicastFn(msg)
	}
}

// Define delegation methods
type opLogDelegate func(string, ...interface{})

// mockLogger is the mock logging structure that is configurable
type mockLogger struct {
	infoFn,
	debugFn,
	errorFn opLogDelegate
}

func (l mockLogger) Info(msg string, args ...interface{}) {
	if l.infoFn != nil {
		l.infoFn(msg, args)
	}
}

func (l mockLogger) Debug(msg string, args ...interface{}) {
	if l.debugFn != nil {
		l.debugFn(msg, args)
	}
}

func (l mockLogger) Error(msg string, args ...interface{}) {
	if l.errorFn != nil {
		l.errorFn(msg, args)
	}
}

type mockMessages struct {
	addMessageFn    func(message *proto.Message)
	pruneByHeightFn func(height uint64)
	signalEventFn   func(messageType proto.MessageType, messageView *proto.View)

	getValidMessagesFn func(
		view *proto.View,
		messageType proto.MessageType,
		isValid func(message *proto.Message) bool,
	) []*proto.Message
	getExtendedRCCFn func(
		height uint64,
		isValidMessage func(message *proto.Message) bool,
		isValidRCC func(round uint64, messages []*proto.Message) bool,
	) []*proto.Message
	getMostRoundChangeMessagesFn func(uint64, uint64) []*proto.Message

	subscribeFn   func(details messages.SubscriptionDetails) *messages.Subscription
	unsubscribeFn func(id messages.SubscriptionID)
}

func (m mockMessages) GetValidMessages(
	view *proto.View,
	messageType proto.MessageType,
	isValid func(*proto.Message) bool,
) []*proto.Message {
	if m.getValidMessagesFn != nil {
		return m.getValidMessagesFn(view, messageType, isValid)
	}

	return nil
}

func (m mockMessages) Subscribe(details messages.SubscriptionDetails) *messages.Subscription {
	if m.subscribeFn != nil {
		return m.subscribeFn(details)
	}

	return nil
}

func (m mockMessages) Unsubscribe(id messages.SubscriptionID) {
	if m.unsubscribeFn != nil {
		m.unsubscribeFn(id)
	}
}

func (m mockMessages) AddMessage(msg *proto.Message) {
	if m.addMessageFn != nil {
		m.addMessageFn(msg)
	}
}

func (m mockMessages) PruneByHeight(height uint64) {
	if m.pruneByHeightFn != nil {
		m.pruneByHeightFn(height)
	}
}

func (m mockMessages) SignalEvent(msgType proto.MessageType, view *proto.View) {
	if m.signalEventFn != nil {
		m.signalEventFn(msgType, view)
	}
}

func (m mockMessages) GetExtendedRCC(
	height uint64,
	isValidMessage func(message *proto.Message) bool,
	isValidRCC func(round uint64, messages []*proto.Message) bool,
) []*proto.Message {
	if m.getExtendedRCCFn != nil {
		return m.getExtendedRCCFn(height, isValidMessage, isValidRCC)
	}

	return nil
}

func (m mockMessages) GetMostRoundChangeMessages(round, height uint64) []*proto.Message {
	if m.getMostRoundChangeMessagesFn != nil {
		return m.getMostRoundChangeMessagesFn(round, height)
	}

	return nil
}

type backendConfigCallback func(*mockBackend)
type loggerConfigCallback func(*mockLogger)
type transportConfigCallback func(*mockTransport)

// newMockCluster creates a new IBFT cluster
func newMockCluster(
	numNodes uint64,
	backendCallback func(*mockBackend, int),
	loggerCallback func(*mockLogger, int),
	transportCallback func(*mockTransport, int),
) *mockCluster {
	if numNodes < 1 {
		return nil
	}

	// Initialize the backend and transport callbacks for
	// each node in the arbitrary cluster
	backendCallbackMap := make(map[int]backendConfigCallback)
	loggerCallbackMap := make(map[int]loggerConfigCallback)
	transportCallbackMap := make(map[int]transportConfigCallback)

	for i := 0; i < int(numNodes); i++ {
		i := i

		if backendCallback != nil {
			backendCallbackMap[i] = func(backend *mockBackend) {
				backendCallback(backend, i)
			}
		}

		if transportCallback != nil {
			transportCallbackMap[i] = func(backend *mockTransport) {
				transportCallback(backend, i)
			}
		}

		if loggerCallback != nil {
			loggerCallbackMap[i] = func(backend *mockLogger) {
				loggerCallback(backend, i)
			}
		}
	}

	nodes := make([]*IBFT, numNodes)
	nodeCtxs := make([]mockNodeContext, numNodes)

	for index := 0; index < int(numNodes); index++ {
		var (
			logger    = &mockLogger{}
			transport = &mockTransport{}
			backend   = &mockBackend{}
		)

		// Execute set callbacks, if any
		if len(backendCallbackMap) > 0 {
			if bc, isSet := backendCallbackMap[index]; isSet {
				bc(backend)
			}
		}

		if len(loggerCallbackMap) > 0 {
			if lc, isSet := loggerCallbackMap[index]; isSet {
				lc(logger)
			}
		}

		if len(transportCallbackMap) > 0 {
			if tc, isSet := transportCallbackMap[index]; isSet {
				tc(transport)
			}
		}

		// Create a new instance of the IBFT node
		nodes[index] = NewIBFT(logger, backend, transport)

		// Instantiate context for the nodes
		nodeCtxs[index] = newMockNodeContext()
	}

	cr := &mockCluster{
		nodes: nodes,
		ctxs:  nodeCtxs,
	}

	// Set a small timeout, because of situations
	// where the byzantine node is the proposer
	cr.setBaseTimeout(testRoundTimeout)

	return cr
}

// mockNodeContext keeps track of the node runtime context
type mockNodeContext struct {
	ctx      context.Context
	cancelFn context.CancelFunc
}

// newMockNodeContext is the constructor of mockNodeContext
func newMockNodeContext() mockNodeContext {
	ctx, cancelFn := context.WithCancel(context.Background())

	return mockNodeContext{
		ctx:      ctx,
		cancelFn: cancelFn,
	}
}

// mockNodeWg is the WaitGroup wrapper for the cluster nodes
type mockNodeWg struct {
	sync.WaitGroup
	count int64
}

func (wg *mockNodeWg) Add(delta int) {
	wg.WaitGroup.Add(delta)
}

func (wg *mockNodeWg) Done() {
	atomic.AddInt64(&wg.count, 1)
	wg.WaitGroup.Done()
}

func (wg *mockNodeWg) getDone() int64 {
	return atomic.LoadInt64(&wg.count)
}

func (wg *mockNodeWg) resetDone() {
	atomic.StoreInt64(&wg.count, 0)
}

// mockCluster represents a mock IBFT cluster
type mockCluster struct {
	nodes []*IBFT           // references to the nodes in the cluster
	ctxs  []mockNodeContext // context handlers for the nodes in the cluster

	wg mockNodeWg
}

func (m *mockCluster) runSequence(height uint64) {
	m.wg.resetDone()

	for nodeIndex, node := range m.nodes {
		m.wg.Add(1)

		go func(ctx context.Context, node *IBFT) {
			// Start the main run loop for the node
			node.RunSequence(ctx, height)

			m.wg.Done()
		}(m.ctxs[nodeIndex].ctx, node)
	}
}

// awaitCompletion waits for completion of all
// nodes in the cluster
func (m *mockCluster) awaitCompletion() {
	// Wait for all main run loops to signalize
	// that they're finished
	m.wg.Wait()
}

// forceShutdown sends a stop signal to all running nodes
// in the cluster, and awaits their completion
func (m *mockCluster) forceShutdown() {
	// Send a stop signal to all the nodes
	for i, ctx := range m.ctxs {
		ctx.cancelFn()

		m.ctxs[i] = newMockNodeContext()
	}

	// Wait for all the nodes to finish
	m.awaitCompletion()
}

// awaitNCompletions awaits completion of the current sequence
// for N nodes in the cluster
func (m *mockCluster) awaitNCompletions(
	ctx context.Context,
	count int64,
) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"await exceeded timeout for %d nodes",
				count,
			)
		default:
			if m.wg.getDone() >= count {
				return nil
			}
		}
	}
}

// pushMessage imitates a message passing service,
// it relays a message to all nodes in the network
func (m *mockCluster) pushMessage(message *proto.Message) {
	for _, node := range m.nodes {
		node.AddMessage(message)
	}
}

// areAllNodesOnRound checks to make sure all nodes
// are on the same specified round
func (m *mockCluster) areAllNodesOnRound(round uint64) bool {
	for _, node := range m.nodes {
		if node.state.getRound() != round {
			return false
		}
	}

	return true
}

// setBaseTimeout sets the base timeout for rounds
func (m *mockCluster) setBaseTimeout(timeout time.Duration) {
	for _, node := range m.nodes {
		node.baseRoundTimeout = timeout
	}
}
