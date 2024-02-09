package core

import (
	"context"
	"sync"
	"time"

	"github.com/ICBNetwork/icb-network-blockchain-modules/messages"
	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"
)

// Define delegation methods
type isValidBlockDelegate func([]byte) bool
type isValidSenderDelegate func(*proto.Message) bool
type isProposerDelegate func([]byte, uint64, uint64) bool
type buildProposalDelegate func(uint64) []byte
type isValidProposalHashDelegate func([]byte, []byte) bool
type isValidCommittedSealDelegate func([]byte, *messages.CommittedSeal) bool

type buildPrePrepareMessageDelegate func(
	[]byte,
	*proto.RoundChangeCertificate,
	*proto.View,
) *proto.Message
type buildPrepareMessageDelegate func([]byte, *proto.View) *proto.Message
type buildCommitMessageDelegate func([]byte, *proto.View) *proto.Message
type buildRoundChangeMessageDelegate func(
	[]byte,
	*proto.PreparedCertificate,
	*proto.View,
) *proto.Message

type quorumDelegate func(blockHeight uint64) uint64
type insertBlockDelegate func([]byte, []*messages.CommittedSeal)
type idDelegate func() []byte
type maximumFaultyNodesDelegate func() uint64

// mockBackend is the mock backend structure that is configurable
type mockBackend struct {
	isValidBlockFn         isValidBlockDelegate
	isValidSenderFn        isValidSenderDelegate
	isProposerFn           isProposerDelegate
	buildProposalFn        buildProposalDelegate
	isValidProposalHashFn  isValidProposalHashDelegate
	isValidCommittedSealFn isValidCommittedSealDelegate

	buildPrePrepareMessageFn  buildPrePrepareMessageDelegate
	buildPrepareMessageFn     buildPrepareMessageDelegate
	buildCommitMessageFn      buildCommitMessageDelegate
	buildRoundChangeMessageFn buildRoundChangeMessageDelegate
	quorumFn                  quorumDelegate
	insertBlockFn             insertBlockDelegate
	idFn                      idDelegate
	maximumFaultyNodesFn      maximumFaultyNodesDelegate
}

func (m mockBackend) ID() []byte {
	if m.idFn != nil {
		return m.idFn()
	}

	return nil
}

func (m mockBackend) InsertBlock(proposal []byte, committedSeals []*messages.CommittedSeal) {
	if m.insertBlockFn != nil {
		m.insertBlockFn(proposal, committedSeals)
	}
}

func (m mockBackend) Quorum(blockNumber uint64) uint64 {
	if m.quorumFn != nil {
		return m.quorumFn(blockNumber)
	}

	return 0
}

func (m mockBackend) IsValidBlock(block []byte) bool {
	if m.isValidBlockFn != nil {
		return m.isValidBlockFn(block)
	}

	return true
}

func (m mockBackend) IsValidSender(msg *proto.Message) bool {
	if m.isValidSenderFn != nil {
		return m.isValidSenderFn(msg)
	}

	return true
}

func (m mockBackend) IsProposer(id []byte, sequence, round uint64) bool {
	if m.isProposerFn != nil {
		return m.isProposerFn(id, sequence, round)
	}

	return false
}

func (m mockBackend) BuildProposal(blockNumber uint64) []byte {
	if m.buildProposalFn != nil {
		return m.buildProposalFn(blockNumber)
	}

	return nil
}

func (m mockBackend) IsValidProposalHash(proposal, hash []byte) bool {
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

func (m mockBackend) MaximumFaultyNodes() uint64 {
	if m.maximumFaultyNodesFn != nil {
		return m.maximumFaultyNodesFn()
	}

	return 0
}

func (m mockBackend) BuildPrePrepareMessage(
	proposal []byte,
	certificate *proto.RoundChangeCertificate,
	view *proto.View,
) *proto.Message {
	if m.buildPrePrepareMessageFn != nil {
		return m.buildPrePrepareMessageFn(proposal, certificate, view)
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
	proposal []byte,
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

	getValidMessagesFn func(
		view *proto.View,
		messageType proto.MessageType,
		isValid func(message *proto.Message) bool,
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
	numNodes int,
	backendCallbackMap map[int]backendConfigCallback,
	loggerCallbackMap map[int]loggerConfigCallback,
	transportCallbackMap map[int]transportConfigCallback,
) *mockCluster {
	if numNodes < 1 {
		return nil
	}

	nodes := make([]*IBFT, numNodes)
	quitChannels := make([]chan struct{}, numNodes)
	messageHandlersQuit := make([]chan struct{}, numNodes)

	for index := 0; index < numNodes; index++ {
		var (
			logger    = &mockLogger{}
			transport = &mockTransport{}
			backend   = &mockBackend{}
		)

		// Execute set callbacks, if any
		if backendCallbackMap != nil {
			if backendCallback, isSet := backendCallbackMap[index]; isSet {
				backendCallback(backend)
			}
		}

		if loggerCallbackMap != nil {
			if loggerCallback, isSet := loggerCallbackMap[index]; isSet {
				loggerCallback(logger)
			}
		}

		if transportCallbackMap != nil {
			if transportCallback, isSet := transportCallbackMap[index]; isSet {
				transportCallback(transport)
			}
		}

		// Create a new instance of the IBFT node
		i := NewIBFT(logger, backend, transport)

		// Instantiate quit channels for node routines
		quitChannels[index] = make(chan struct{})
		messageHandlersQuit[index] = make(chan struct{})

		nodes[index] = i
	}

	return &mockCluster{
		nodes: nodes,
	}
}

// mockCluster represents a mock IBFT cluster
type mockCluster struct {
	nodes []*IBFT // references to the nodes in the cluster

	wg sync.WaitGroup
}

func (m *mockCluster) runSequence(height uint64) {
	for _, node := range m.nodes {
		m.wg.Add(1)

		go func(node *IBFT, height uint64) {
			defer func() {
				m.wg.Done()
			}()

			// Start the main run loop for the node
			node.RunSequence(context.Background(), height)
		}(node, height)
	}
}

// stop sends a quit signal to all nodes
// in the cluster
func (m *mockCluster) stop() {
	// Wait for all main run loops to signalize
	// that they're finished
	m.wg.Wait()
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
