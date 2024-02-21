package messages

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"
)

func TestEventManager_SubscribeCancel(t *testing.T) {
	t.Parallel()

	numSubscriptions := 10
	subscriptions := make([]*Subscription, numSubscriptions)
	baseDetails := SubscriptionDetails{
		MessageType: proto.MessageType_PREPARE,
		View: &proto.View{
			Height: 0,
			Round:  0,
		},
		MinNumMessages: 1,
	}

	IDMap := make(map[SubscriptionID]bool)

	em := newEventManager()
	defer em.close()

	// Create the subscriptions
	for i := 0; i < numSubscriptions; i++ {
		subscriptions[i] = em.subscribe(baseDetails)

		// Check that the number is up-to-date
		assert.Equal(t, int64(i+1), em.numSubscriptions)

		// Check if a duplicate ID has been issued
		if _, ok := IDMap[subscriptions[i].ID]; ok {
			t.Fatalf("Duplicate ID entry")
		} else {
			IDMap[subscriptions[i].ID] = true
		}
	}

	quitCh := make(chan struct{}, 1)
	defer func() {
		quitCh <- struct{}{}
	}()

	go func() {
		for {
			em.signalEvent(baseDetails.MessageType, baseDetails.View)

			select {
			case <-quitCh:
				return
			default:
			}
		}
	}()

	// Cancel them concurrently
	for _, subscription := range subscriptions {
		em.cancelSubscription(subscription.ID)
	}

	// Check that the number is up-to-date
	assert.Equal(t, int64(0), em.numSubscriptions)
}

func TestEventManager_SubscribeClose(t *testing.T) {
	t.Parallel()

	numSubscriptions := 10
	subscriptions := make([]*Subscription, numSubscriptions)
	baseDetails := SubscriptionDetails{
		MessageType: proto.MessageType_PREPARE,
		View: &proto.View{
			Height: 0,
			Round:  0,
		},
		MinNumMessages: 1,
	}

	em := newEventManager()

	// Create the subscriptions
	for i := 0; i < numSubscriptions; i++ {
		subscriptions[i] = em.subscribe(baseDetails)

		// Check that the number is up-to-date
		assert.Equal(t, int64(i+1), em.numSubscriptions)
	}

	// Close off the event manager
	em.close()
	assert.Equal(t, int64(0), em.numSubscriptions)

	// Check if the subscription channels are closed
	for indx, subscription := range subscriptions {
		if _, more := <-subscription.SubCh; more {
			t.Fatalf("SubscriptionDetails channel not closed for index %d", indx)
		}
	}
}
