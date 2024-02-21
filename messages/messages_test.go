package messages

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"
)

// generateRandomMessages generates random messages for the
func generateRandomMessages(
	count int,
	view *proto.View,
	messageTypes ...proto.MessageType,
) []*proto.Message {
	messages := make([]*proto.Message, 0)

	for index := 0; index < count; index++ {
		for _, messageType := range messageTypes {
			message := &proto.Message{
				From: []byte(strconv.Itoa(index)),
				View: view,
				Type: messageType,
			}

			switch messageType {
			case proto.MessageType_PREPREPARE:
				message.Payload = &proto.Message_PreprepareData{
					PreprepareData: &proto.PrePrepareMessage{
						Proposal: nil,
					},
				}
			case proto.MessageType_PREPARE:
				message.Payload = &proto.Message_PrepareData{
					PrepareData: &proto.PrepareMessage{
						ProposalHash: nil,
					},
				}
			case proto.MessageType_COMMIT:
				message.Payload = &proto.Message_CommitData{
					CommitData: &proto.CommitMessage{
						ProposalHash:  nil,
						CommittedSeal: nil,
					},
				}
			}

			messages = append(messages, message)
		}
	}

	return messages
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestMessages_AddMessage tests if the message addition
// of different types works
func TestMessages_AddMessage(t *testing.T) {
	t.Parallel()

	numMessages := 5
	initialView := &proto.View{
		Height: 1,
		Round:  1,
	}

	messages := NewMessages()
	defer messages.Close()

	// Append random message types
	randomMessages := generateRandomMessages(
		numMessages,
		initialView,
		proto.MessageType_PREPARE,
		proto.MessageType_COMMIT,
		proto.MessageType_ROUND_CHANGE,
	)

	for _, message := range randomMessages {
		messages.AddMessage(message)
	}

	// Make sure that the messages are present
	assert.Equal(t, numMessages, messages.numMessages(initialView, proto.MessageType_PREPARE))
	assert.Equal(t, numMessages, messages.numMessages(initialView, proto.MessageType_COMMIT))
	assert.Equal(t, numMessages, messages.numMessages(initialView, proto.MessageType_ROUND_CHANGE))
}

// TestMessages_AddDuplicates tests that no duplicates
// can be added to the height -> round -> message queue,
// meaning a sender cannot fill the message queue with duplicate messages
// of the same view for the same message type
func TestMessages_AddDuplicates(t *testing.T) {
	t.Parallel()

	numMessages := 5
	commonSender := strconv.Itoa(1)
	commonType := proto.MessageType_PREPARE
	initialView := &proto.View{
		Height: 1,
		Round:  1,
	}

	messages := NewMessages()
	defer messages.Close()

	// Append random message types
	randomMessages := generateRandomMessages(
		numMessages,
		initialView,
		commonType,
	)

	for _, message := range randomMessages {
		message.From = []byte(commonSender)
		messages.AddMessage(message)
	}

	// Check that only 1 message has been added
	assert.Equal(t, 1, messages.numMessages(initialView, commonType))
}

// TestMessages_Prune tests if pruning of certain messages works
func TestMessages_Prune(t *testing.T) {
	t.Parallel()

	numMessages := 5
	messageType := proto.MessageType_PREPARE
	messages := NewMessages()

	t.Cleanup(func() {
		messages.Close()
	})

	views := make([]*proto.View, 0)
	for index := uint64(1); index <= 3; index++ {
		views = append(views, &proto.View{
			Height: 1,
			Round:  index,
		})
	}

	// Append random message types
	randomMessages := make([]*proto.Message, 0)
	for _, view := range views {
		randomMessages = append(
			randomMessages,
			generateRandomMessages(
				numMessages,
				view,
				messageType,
			)...,
		)
	}

	for _, message := range randomMessages {
		messages.AddMessage(message)
	}

	// Prune out the messages from this view
	messages.PruneByHeight(views[1].Height + 1)

	// Make sure the round 1 messages are pruned out
	assert.Equal(t, 0, messages.numMessages(views[0], messageType))

	// Make sure the round 2 messages are pruned out
	assert.Equal(t, 0, messages.numMessages(views[1], messageType))

	// Make sure the round 3 messages are pruned out
	assert.Equal(t, 0, messages.numMessages(views[2], messageType))
}

// TestMessages_GetMessage makes sure
// that messages are fetched correctly for the
// corresponding message type
func TestMessages_GetValidMessagesMessage(t *testing.T) {
	t.Parallel()

	var (
		defaultView = &proto.View{
			Height: 1,
			Round:  0,
		}
		numMessages = 5
	)

	testTable := []struct {
		name        string
		messageType proto.MessageType
	}{
		{
			"Fetch PREPREPAREs",
			proto.MessageType_PREPREPARE,
		},
		{
			"Fetch PREPAREs",
			proto.MessageType_PREPARE,
		},
		{
			"Fetch COMMITs",
			proto.MessageType_COMMIT,
		},
		{
			"Fetch ROUND_CHANGEs",
			proto.MessageType_ROUND_CHANGE,
		},
	}

	alwaysInvalidFn := func(_ *proto.Message) bool {
		return false
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Add the initial message set
			messages := NewMessages()
			defer messages.Close()

			// Generate random messages
			randomMessages := generateRandomMessages(
				numMessages,
				defaultView,
				testCase.messageType,
			)

			// Add the messages to the corresponding queue
			for _, message := range randomMessages {
				messages.AddMessage(message)
			}

			// Make sure the messages are there
			assert.Equal(
				t,
				numMessages,
				messages.numMessages(defaultView, testCase.messageType),
			)

			// Start fetching messages and making sure they're not cleared
			switch testCase.messageType {
			case proto.MessageType_PREPREPARE:
				messages.GetValidMessages(defaultView, proto.MessageType_PREPREPARE, alwaysInvalidFn)
			case proto.MessageType_PREPARE:
				messages.GetValidMessages(defaultView, proto.MessageType_PREPARE, alwaysInvalidFn)
			case proto.MessageType_COMMIT:
				messages.GetValidMessages(defaultView, proto.MessageType_COMMIT, alwaysInvalidFn)
			case proto.MessageType_ROUND_CHANGE:
				messages.GetValidMessages(defaultView, proto.MessageType_ROUND_CHANGE, alwaysInvalidFn)
			}

			assert.Equal(
				t,
				0,
				messages.numMessages(defaultView, testCase.messageType),
			)
		})
	}
}

// TestMessages_GetExtendedRCC makes sure
// Messages returns the ROUND-CHANGE messages for the highest round
// where all messages are valid
func TestMessages_GetExtendedRCC(t *testing.T) {
	t.Parallel()

	var (
		height uint64 = 0
		quorum        = 5
	)

	messages := NewMessages()
	defer messages.Close()

	// Generate round messages
	randomMessages := map[uint64][]*proto.Message{
		0: generateRandomMessages(quorum-1, &proto.View{
			Height: height,
			Round:  0,
		}, proto.MessageType_ROUND_CHANGE),

		1: generateRandomMessages(quorum, &proto.View{
			Height: height,
			Round:  1,
		}, proto.MessageType_ROUND_CHANGE),

		2: generateRandomMessages(quorum, &proto.View{
			Height: height,
			Round:  2,
		}, proto.MessageType_ROUND_CHANGE),

		3: generateRandomMessages(quorum-1, &proto.View{
			Height: height,
			Round:  3,
		}, proto.MessageType_ROUND_CHANGE),
	}

	// Add the messages
	for _, roundMessages := range randomMessages {
		for _, message := range roundMessages {
			messages.AddMessage(message)
		}
	}

	extendedRCC := messages.GetExtendedRCC(
		height,
		func(message *proto.Message) bool {
			return true
		},
		func(round uint64, messages []*proto.Message) bool {
			return len(messages) >= quorum
		},
	)

	assert.ElementsMatch(
		t,
		randomMessages[2],
		extendedRCC,
	)
}

// TestMessages_GetMostRoundChangeMessages makes sure
// the round messages for the round with the most round change
// messages are fetched
func TestMessages_GetMostRoundChangeMessages(t *testing.T) {
	t.Parallel()

	messages := NewMessages()
	defer messages.Close()

	mostMessageCount := 3
	mostMessagesRound := uint64(2)

	// Generate round messages
	randomMessages := map[uint64][]*proto.Message{
		0: generateRandomMessages(mostMessageCount-2, &proto.View{
			Height: 0,
			Round:  0,
		}, proto.MessageType_ROUND_CHANGE),
		1: generateRandomMessages(mostMessageCount-1, &proto.View{
			Height: 0,
			Round:  1,
		}, proto.MessageType_ROUND_CHANGE),
		mostMessagesRound: generateRandomMessages(mostMessageCount, &proto.View{
			Height: 0,
			Round:  mostMessagesRound,
		}, proto.MessageType_ROUND_CHANGE),
	}

	// Add the messages
	for _, roundMessages := range randomMessages {
		for _, message := range roundMessages {
			messages.AddMessage(message)
		}
	}

	roundChangeMessages := messages.GetMostRoundChangeMessages(0, 0)

	if len(roundChangeMessages) != mostMessageCount {
		t.Fatalf("Invalid number of round change messages, %d", len(roundChangeMessages))
	}

	assert.Equal(t, mostMessagesRound, roundChangeMessages[0].View.Round)
}

// TestMessages_EventManager checks that the event manager
// behaves correctly when new messages appear
func TestMessages_EventManager(t *testing.T) {
	t.Parallel()

	messages := NewMessages()
	defer messages.Close()

	numMessages := 10
	messageType := proto.MessageType_PREPARE
	baseView := &proto.View{
		Height: 0,
		Round:  0,
	}

	// Create the subscription
	subscription := messages.Subscribe(SubscriptionDetails{
		MessageType: messageType,
		View:        baseView,
	})

	defer messages.Unsubscribe(subscription.ID)

	// Push random messages
	randomMessages := generateRandomMessages(numMessages, baseView, messageType)
	for _, message := range randomMessages {
		messages.AddMessage(message)
		messages.SignalEvent(message.Type, message.View)
	}

	// Wait for the subscription event to happen
	select {
	case <-subscription.SubCh:
	case <-time.After(5 * time.Second):
	}

	// Make sure the number of messages is actually accurate
	assert.Equal(t, numMessages, messages.numMessages(baseView, messageType))
}
