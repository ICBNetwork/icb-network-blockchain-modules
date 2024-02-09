package core

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ICBNetwork/icb-network-blockchain-modules/messages"
	"github.com/ICBNetwork/icb-network-blockchain-modules/messages/proto"
	"github.com/stretchr/testify/assert"
)

func proposalMatches(proposal []byte, message *proto.Message) bool {
	if message == nil || message.Type != proto.MessageType_PREPREPARE {
		return false
	}

	preprepareData, _ := message.Payload.(*proto.Message_PreprepareData)
	extractedProposal := preprepareData.PreprepareData.Proposal

	return bytes.Equal(proposal, extractedProposal)
}

func prepareHashMatches(prepareHash []byte, message *proto.Message) bool {
	if message == nil || message.Type != proto.MessageType_PREPARE {
		return false
	}

	prepareData, _ := message.Payload.(*proto.Message_PrepareData)
	extractedPrepareHash := prepareData.PrepareData.ProposalHash

	return bytes.Equal(prepareHash, extractedPrepareHash)
}

func commitHashMatches(commitHash []byte, message *proto.Message) bool {
	if message == nil || message.Type != proto.MessageType_COMMIT {
		return false
	}

	commitData, _ := message.Payload.(*proto.Message_CommitData)
	extractedCommitHash := commitData.CommitData.ProposalHash

	return bytes.Equal(commitHash, extractedCommitHash)
}

func generateMessages(count uint64, messageType proto.MessageType) []*proto.Message {
	messages := make([]*proto.Message, count)

	for index := uint64(0); index < count; index++ {
		message := &proto.Message{
			View: &proto.View{
				Height: 0,
				Round:  0,
			},
			Type: messageType,
		}

		switch message.Type {
		case proto.MessageType_PREPREPARE:
			message.Payload = &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{},
			}
		case proto.MessageType_PREPARE:
			message.Payload = &proto.Message_PrepareData{
				PrepareData: &proto.PrepareMessage{},
			}
		case proto.MessageType_COMMIT:
			message.Payload = &proto.Message_CommitData{
				CommitData: &proto.CommitMessage{},
			}
		case proto.MessageType_ROUND_CHANGE:
			message.Payload = &proto.Message_RoundChangeData{
				RoundChangeData: &proto.RoundChangeMessage{},
			}
		}

		messages[index] = message
	}

	return messages
}

func generateMessagesWithSender(count uint64, messageType proto.MessageType, sender []byte) []*proto.Message {
	messages := generateMessages(count, messageType)

	for _, message := range messages {
		message.From = sender
	}

	return messages
}

func generateMessagesWithUniqueSender(count uint64, messageType proto.MessageType) []*proto.Message {
	messages := generateMessages(count, messageType)

	for index, message := range messages {
		message.From = []byte(fmt.Sprintf("node %d", index))
	}

	return messages
}

func appendProposalHash(messages []*proto.Message, proposalHash []byte) {
	for _, message := range messages {
		switch message.Type {
		case proto.MessageType_PREPREPARE:
			ppData, _ := message.Payload.(*proto.Message_PreprepareData)
			payload := ppData.PreprepareData

			payload.ProposalHash = proposalHash
		case proto.MessageType_PREPARE:
			pData, _ := message.Payload.(*proto.Message_PrepareData)
			payload := pData.PrepareData

			payload.ProposalHash = proposalHash
		default:
		}
	}
}

func setRoundForMessages(messages []*proto.Message, round uint64) {
	for _, message := range messages {
		message.View.Round = round
	}
}

func generateSeals(count int) [][]byte {
	seals := make([][]byte, count)

	for i := 0; i < count; i++ {
		seals[i] = []byte("committed seal")
	}

	return seals
}

func filterMessages(messages []*proto.Message, isValid func(message *proto.Message) bool) []*proto.Message {
	newMessages := make([]*proto.Message, 0)

	for _, message := range messages {
		if isValid(message) {
			newMessages = append(newMessages, message)
		}
	}

	return newMessages
}

func generateFilledRCMessages(
	quorum uint64,
	proposal,
	proposalHash []byte) []*proto.Message {
	// Generate random RC messages
	roundChangeMessages := generateMessages(quorum, proto.MessageType_ROUND_CHANGE)
	prepareMessages := generateMessages(quorum-1, proto.MessageType_PREPARE)

	// Fill up the prepare message hashes
	for index, message := range prepareMessages {
		message.Payload = &proto.Message_PrepareData{
			PrepareData: &proto.PrepareMessage{
				ProposalHash: proposalHash,
			},
		}
		message.View = &proto.View{
			Height: 0,
			Round:  1,
		}
		message.From = []byte(fmt.Sprintf("node %d", index+1))
	}

	lastPreparedCertificate := &proto.PreparedCertificate{
		ProposalMessage: &proto.Message{
			View: &proto.View{
				Height: 0,
				Round:  1,
			},
			From: []byte("unique node"),
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{
					Proposal:     proposal,
					ProposalHash: proposalHash,
					Certificate:  nil,
				},
			},
		},
		PrepareMessages: prepareMessages,
	}

	// Fill up their certificates
	for _, message := range roundChangeMessages {
		message.Payload = &proto.Message_RoundChangeData{
			RoundChangeData: &proto.RoundChangeMessage{
				LastPreparedProposedBlock: proposal,
				LatestPreparedCertificate: lastPreparedCertificate,
			},
		}
		message.View = &proto.View{
			Height: 0,
			Round:  1,
		}
	}

	return roundChangeMessages
}

// TestRunNewRound_Proposer checks that the node functions
// correctly as the proposer for a block
func TestRunNewRound_Proposer(t *testing.T) {
	t.Parallel()

	t.Run(
		"proposer builds fresh block",
		func(t *testing.T) {
			t.Parallel()

			ctx, cancelFn := context.WithCancel(context.Background())

			var (
				newProposal                        = []byte("new block")
				multicastedProposal *proto.Message = nil

				log       = mockLogger{}
				transport = mockTransport{func(message *proto.Message) {
					if message != nil && message.Type == proto.MessageType_PREPREPARE {
						multicastedProposal = message
					}
				}}
				backend = mockBackend{
					idFn: func() []byte { return nil },
					isProposerFn: func(_ []byte, _ uint64, _ uint64) bool {
						return true
					},
					buildProposalFn: func(_ uint64) []byte {
						return newProposal
					},
					buildPrePrepareMessageFn: func(
						proposal []byte,
						certificate *proto.RoundChangeCertificate,
						view *proto.View,
					) *proto.Message {
						return &proto.Message{
							View: view,
							Type: proto.MessageType_PREPREPARE,
							Payload: &proto.Message_PreprepareData{
								PreprepareData: &proto.PrePrepareMessage{
									Proposal:    proposal,
									Certificate: certificate,
								},
							},
						}
					},
				}
				messages = mockMessages{
					subscribeFn: func(_ messages.SubscriptionDetails) *messages.Subscription {
						cancelFn()

						return &messages.Subscription{
							ID:    messages.SubscriptionID(1),
							SubCh: make(chan uint64),
						}
					},
				}
			)

			i := NewIBFT(log, backend, transport)
			i.messages = messages

			i.wg.Add(1)
			i.startRound(ctx)

			i.wg.Wait()

			// Make sure the node is in prepare state
			assert.Equal(t, prepare, i.state.name)

			// Make sure the accepted proposal is the one proposed to other nodes
			assert.Equal(t, multicastedProposal, i.state.proposalMessage)

			// Make sure the accepted proposal matches what was built
			assert.True(t, proposalMatches(newProposal, multicastedProposal))
		},
	)

	t.Run(
		"proposer builds proposal for round > 0 (create new)",
		func(t *testing.T) {
			t.Parallel()

			quorum := uint64(4)
			ctx, cancelFn := context.WithCancel(context.Background())

			roundChangeMessages := generateMessages(quorum, proto.MessageType_ROUND_CHANGE)
			setRoundForMessages(roundChangeMessages, 1)

			var (
				multicastedPreprepare *proto.Message = nil
				multicastedPrepare    *proto.Message = nil
				proposalHash                         = []byte("proposal hash")
				proposal                             = []byte("proposal")
				notifyCh                             = make(chan uint64, 1)

				log       = mockLogger{}
				transport = mockTransport{func(message *proto.Message) {
					switch message.Type {
					case proto.MessageType_PREPREPARE:
						multicastedPreprepare = message
					case proto.MessageType_PREPARE:
						multicastedPrepare = message
					default:
					}
				}}
				backend = mockBackend{
					idFn: func() []byte { return nil },
					isProposerFn: func(_ []byte, _ uint64, _ uint64) bool {
						return true
					},
					quorumFn: func(_ uint64) uint64 {
						return quorum
					},
					buildProposalFn: func(_ uint64) []byte {
						return proposal
					},
					buildPrepareMessageFn: func(_ []byte, view *proto.View) *proto.Message {
						return &proto.Message{
							View: view,
							Type: proto.MessageType_PREPARE,
							Payload: &proto.Message_PrepareData{
								PrepareData: &proto.PrepareMessage{
									ProposalHash: proposalHash,
								},
							},
						}
					},
					buildPrePrepareMessageFn: func(
						_ []byte,
						_ *proto.RoundChangeCertificate,
						view *proto.View,
					) *proto.Message {
						return &proto.Message{
							View: view,
							Type: proto.MessageType_PREPREPARE,
							Payload: &proto.Message_PreprepareData{
								PreprepareData: &proto.PrePrepareMessage{
									Proposal:     proposal,
									ProposalHash: proposalHash,
								},
							},
						}
					},
				}
				messages = mockMessages{
					subscribeFn: func(_ messages.SubscriptionDetails) *messages.Subscription {
						return &messages.Subscription{
							ID:    messages.SubscriptionID(1),
							SubCh: notifyCh,
						}
					},
					unsubscribeFn: func(_ messages.SubscriptionID) {
						cancelFn()
					},
					getValidMessagesFn: func(
						view *proto.View,
						messageType proto.MessageType,
						isValid func(message *proto.Message) bool,
					) []*proto.Message {
						return filterMessages(
							roundChangeMessages,
							isValid,
						)
					},
				}
			)

			i := NewIBFT(log, backend, transport)
			i.messages = messages
			i.state.setView(&proto.View{
				Height: 0,
				Round:  1,
			})

			notifyCh <- 1

			i.wg.Add(1)
			i.startRound(ctx)

			i.wg.Wait()

			// Make sure the node changed the state to prepare
			assert.Equal(t, prepare, i.state.name)

			// Make sure the multicasted proposal is the accepted proposal
			assert.Equal(t, multicastedPreprepare, i.state.proposalMessage)

			// Make sure the correct proposal value was multicasted
			assert.True(t, proposalMatches(proposal, multicastedPreprepare))

			// Make sure the prepare message was not multicasted
			assert.Nil(t, multicastedPrepare)
		},
	)

	t.Run(
		"proposer builds proposal for round > 0 (resend last prepared proposal)",
		func(t *testing.T) {
			t.Parallel()

			lastPreparedProposedBlock := []byte("last prepared block")
			proposalHash := []byte("proposal hash")

			quorum := uint64(4)
			ctx, cancelFn := context.WithCancel(context.Background())

			roundChangeMessages := generateMessages(quorum, proto.MessageType_ROUND_CHANGE)
			prepareMessages := generateMessages(quorum-1, proto.MessageType_PREPARE)

			for index, message := range prepareMessages {
				message.Payload = &proto.Message_PrepareData{
					PrepareData: &proto.PrepareMessage{
						ProposalHash: proposalHash,
					},
				}

				message.From = []byte(fmt.Sprintf("node %d", index+1))
			}

			setRoundForMessages(roundChangeMessages, 1)

			// Make sure at least one RC message has a PC
			payload, _ := roundChangeMessages[1].Payload.(*proto.Message_RoundChangeData)
			rcData := payload.RoundChangeData

			rcData.LastPreparedProposedBlock = lastPreparedProposedBlock
			rcData.LatestPreparedCertificate = &proto.PreparedCertificate{
				ProposalMessage: &proto.Message{
					View: &proto.View{
						Height: 0,
						Round:  0,
					},
					From: []byte("unique node"),
					Type: proto.MessageType_PREPREPARE,
					Payload: &proto.Message_PreprepareData{
						PreprepareData: &proto.PrePrepareMessage{
							Proposal:     lastPreparedProposedBlock,
							ProposalHash: proposalHash,
							Certificate:  nil,
						},
					},
				},
				PrepareMessages: prepareMessages,
			}

			var (
				multicastedPreprepare *proto.Message = nil
				multicastedPrepare    *proto.Message = nil
				proposal                             = []byte("proposal")
				notifyCh                             = make(chan uint64, 1)

				log       = mockLogger{}
				transport = mockTransport{func(message *proto.Message) {
					switch message.Type {
					case proto.MessageType_PREPREPARE:
						multicastedPreprepare = message
					case proto.MessageType_PREPARE:
						multicastedPrepare = message
					default:
					}
				}}
				backend = mockBackend{
					idFn: func() []byte { return nil },
					isProposerFn: func(_ []byte, _ uint64, _ uint64) bool {
						return true
					},
					quorumFn: func(_ uint64) uint64 {
						return quorum
					},
					buildProposalFn: func(_ uint64) []byte {
						return proposal
					},
					buildPrepareMessageFn: func(_ []byte, view *proto.View) *proto.Message {
						return &proto.Message{
							View: view,
							Type: proto.MessageType_PREPARE,
							Payload: &proto.Message_PrepareData{
								PrepareData: &proto.PrepareMessage{
									ProposalHash: proposalHash,
								},
							},
						}
					},
					buildPrePrepareMessageFn: func(
						proposal []byte,
						certificate *proto.RoundChangeCertificate,
						view *proto.View,
					) *proto.Message {
						return &proto.Message{
							View: view,
							Type: proto.MessageType_PREPREPARE,
							Payload: &proto.Message_PreprepareData{
								PreprepareData: &proto.PrePrepareMessage{
									Proposal:     proposal,
									ProposalHash: proposalHash,
									Certificate:  certificate,
								},
							},
						}
					},
				}
				messages = mockMessages{
					subscribeFn: func(_ messages.SubscriptionDetails) *messages.Subscription {
						return &messages.Subscription{
							ID:    messages.SubscriptionID(1),
							SubCh: notifyCh,
						}
					},
					unsubscribeFn: func(_ messages.SubscriptionID) {
						cancelFn()
					},
					getValidMessagesFn: func(
						view *proto.View,
						messageType proto.MessageType,
						isValid func(message *proto.Message) bool,
					) []*proto.Message {
						return filterMessages(
							roundChangeMessages,
							isValid,
						)
					},
				}
			)

			i := NewIBFT(log, backend, transport)
			i.messages = messages
			i.state.setView(&proto.View{
				Height: 0,
				Round:  1,
			})

			notifyCh <- 1

			i.wg.Add(1)
			i.startRound(ctx)

			i.wg.Wait()

			// Make sure the node changed the state to prepare
			assert.Equal(t, prepare, i.state.name)

			// Make sure the multicasted proposal is the accepted proposal
			assert.Equal(t, multicastedPreprepare, i.state.proposalMessage)

			// Make sure the correct proposal was multicasted
			assert.True(t, proposalMatches(lastPreparedProposedBlock, multicastedPreprepare))

			// Make sure the prepare message was not multicasted
			assert.Nil(t, multicastedPrepare)
		},
	)
}

// TestRunNewRound_Validator_Zero validates the behavior
// of a non-proposer when receiving the proposal for round 0
func TestRunNewRound_Validator_Zero(t *testing.T) {
	t.Parallel()

	ctx, cancelFn := context.WithCancel(context.Background())

	var (
		proposal                          = []byte("new block")
		proposalHash                      = []byte("proposal hash")
		proposer                          = []byte("proposer")
		multicastedPrepare *proto.Message = nil
		notifyCh                          = make(chan uint64, 1)

		log       = mockLogger{}
		transport = mockTransport{
			func(message *proto.Message) {
				if message != nil && message.Type == proto.MessageType_PREPARE {
					multicastedPrepare = message
				}
			},
		}
		backend = mockBackend{
			idFn: func() []byte {
				return []byte("non proposer")
			},
			quorumFn: func(_ uint64) uint64 {
				return 1
			},
			buildPrepareMessageFn: func(proposal []byte, view *proto.View) *proto.Message {
				return &proto.Message{
					View: view,
					Type: proto.MessageType_PREPARE,
					Payload: &proto.Message_PrepareData{
						PrepareData: &proto.PrepareMessage{
							ProposalHash: proposalHash,
						},
					},
				}
			},
			isProposerFn: func(from []byte, _, _ uint64) bool {
				return bytes.Equal(from, proposer)
			},
			isValidBlockFn: func(_ []byte) bool {
				return true
			},
		}
		messages = mockMessages{
			subscribeFn: func(_ messages.SubscriptionDetails) *messages.Subscription {
				return &messages.Subscription{
					ID:    messages.SubscriptionID(1),
					SubCh: notifyCh,
				}
			},
			unsubscribeFn: func(_ messages.SubscriptionID) {
				cancelFn()
			},
			getValidMessagesFn: func(
				view *proto.View,
				_ proto.MessageType,
				isValid func(message *proto.Message) bool,
			) []*proto.Message {
				return filterMessages(
					[]*proto.Message{
						{
							View: view,
							From: proposer,
							Type: proto.MessageType_PREPREPARE,
							Payload: &proto.Message_PreprepareData{
								PreprepareData: &proto.PrePrepareMessage{
									Proposal: proposal,
								},
							},
						},
					},
					isValid,
				)
			},
		}
	)

	i := NewIBFT(log, backend, transport)
	i.messages = messages

	// Make sure the notification is sent out
	notifyCh <- 0

	i.wg.Add(1)
	i.startRound(ctx)

	i.wg.Wait()

	// Make sure the node moves to prepare state
	assert.Equal(t, prepare, i.state.name)

	// Make sure the accepted proposal is the one that was sent out
	assert.Equal(t, proposal, i.state.getProposal())

	// Make sure the correct proposal hash was multicasted
	assert.True(t, prepareHashMatches(proposalHash, multicastedPrepare))
}

// TestRunNewRound_Validator_NonZero validates the behavior
// of a non-proposer when receiving proposals for rounds > 0
func TestRunNewRound_Validator_NonZero(t *testing.T) {
	t.Parallel()

	quorum := uint64(4)
	proposalHash := []byte("proposal hash")
	proposal := []byte("new block")
	proposer := []byte("proposer")

	generateProposalWithNoPrevious := func() *proto.Message {
		roundChangeMessages := generateMessagesWithUniqueSender(quorum, proto.MessageType_ROUND_CHANGE)
		setRoundForMessages(roundChangeMessages, 1)

		return &proto.Message{
			View: &proto.View{
				Height: 0,
				Round:  1,
			},
			From: proposer,
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{
					Proposal:     proposal,
					ProposalHash: proposalHash,
					Certificate: &proto.RoundChangeCertificate{
						RoundChangeMessages: roundChangeMessages,
					},
				},
			},
		}
	}

	generateProposalWithPrevious := func() *proto.Message {
		return &proto.Message{
			View: &proto.View{
				Height: 0,
				Round:  1,
			},
			From: proposer,
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{
					Proposal:     proposal,
					ProposalHash: proposalHash,
					Certificate: &proto.RoundChangeCertificate{
						RoundChangeMessages: generateFilledRCMessages(quorum, proposal, proposalHash),
					},
				},
			},
		}
	}

	testTable := []struct {
		name            string
		proposalMessage *proto.Message
	}{
		{
			"validator receives valid block proposal (round > 0, new block)",
			generateProposalWithNoPrevious(),
		},
		{
			"validator receives valid block proposal (round > 0, old block)",
			generateProposalWithPrevious(),
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancelFn := context.WithCancel(context.Background())
			defer cancelFn()

			var (
				multicastedPrepare *proto.Message = nil
				notifyCh                          = make(chan uint64, 1)

				log       = mockLogger{}
				transport = mockTransport{
					func(message *proto.Message) {
						if message != nil && message.Type == proto.MessageType_PREPARE {
							multicastedPrepare = message
						}
					},
				}
				backend = mockBackend{
					idFn: func() []byte {
						return []byte("non proposer")
					},
					quorumFn: func(_ uint64) uint64 {
						return 1
					},
					buildPrepareMessageFn: func(proposal []byte, view *proto.View) *proto.Message {
						return &proto.Message{
							View: view,
							Type: proto.MessageType_PREPARE,
							Payload: &proto.Message_PrepareData{
								PrepareData: &proto.PrepareMessage{
									ProposalHash: proposalHash,
								},
							},
						}
					},
					isProposerFn: func(from []byte, _, _ uint64) bool {
						return bytes.Equal(from, proposer)
					},
					isValidBlockFn: func(_ []byte) bool {
						return true
					},
				}
				messages = mockMessages{
					subscribeFn: func(_ messages.SubscriptionDetails) *messages.Subscription {
						return &messages.Subscription{
							ID:    messages.SubscriptionID(1),
							SubCh: notifyCh,
						}
					},
					unsubscribeFn: func(_ messages.SubscriptionID) {
						cancelFn()
					},
					getValidMessagesFn: func(
						view *proto.View,
						_ proto.MessageType,
						isValid func(message *proto.Message) bool,
					) []*proto.Message {
						return filterMessages(
							[]*proto.Message{
								testCase.proposalMessage,
							},
							isValid,
						)
					},
				}
			)

			i := NewIBFT(log, backend, transport)
			i.messages = messages
			i.state.setView(&proto.View{
				Height: 0,
				Round:  1,
			})

			// Make sure the notification is sent out
			notifyCh <- 1

			i.wg.Add(1)
			i.startRound(ctx)

			i.wg.Wait()

			// Make sure the node moves to prepare state
			assert.Equal(t, prepare, i.state.name)

			// Make sure the accepted proposal is the one that was sent out
			assert.Equal(t, proposal, i.state.getProposal())

			// Make sure the correct proposal hash was multicasted
			assert.True(t, prepareHashMatches(proposalHash, multicastedPrepare))
		})
	}
}

// TestRunPrepare checks that the node behaves correctly
// in prepare state
func TestRunPrepare(t *testing.T) {
	t.Parallel()

	t.Run(
		"validator receives quorum of PREPARE messages",
		func(t *testing.T) {
			t.Parallel()

			ctx, cancelFn := context.WithCancel(context.Background())

			var (
				proposal                         = []byte("block proposal")
				proposalHash                     = []byte("proposal hash")
				multicastedCommit *proto.Message = nil
				notifyCh                         = make(chan uint64, 1)

				log       = mockLogger{}
				transport = mockTransport{func(message *proto.Message) {
					if message != nil && message.Type == proto.MessageType_COMMIT {
						multicastedCommit = message
					}
				}}
				backend = mockBackend{
					buildCommitMessageFn: func(_ []byte, view *proto.View) *proto.Message {
						return &proto.Message{
							View: view,
							Type: proto.MessageType_COMMIT,
							Payload: &proto.Message_CommitData{
								CommitData: &proto.CommitMessage{
									ProposalHash: proposalHash,
								},
							},
						}
					},
					quorumFn: func(_ uint64) uint64 {
						return 1
					},
					isValidProposalHashFn: func(_ []byte, hash []byte) bool {
						return bytes.Equal(proposalHash, hash)
					},
				}
				messages = mockMessages{
					subscribeFn: func(_ messages.SubscriptionDetails) *messages.Subscription {
						return &messages.Subscription{
							ID:    messages.SubscriptionID(1),
							SubCh: notifyCh,
						}
					},
					unsubscribeFn: func(_ messages.SubscriptionID) {
						cancelFn()
					},
					getValidMessagesFn: func(
						view *proto.View,
						_ proto.MessageType,
						isValid func(message *proto.Message) bool,
					) []*proto.Message {
						return filterMessages(
							[]*proto.Message{
								{
									View: view,
									Type: proto.MessageType_PREPARE,
									Payload: &proto.Message_PrepareData{
										PrepareData: &proto.PrepareMessage{
											ProposalHash: proposalHash,
										},
									},
								},
							},
							isValid,
						)
					},
				}
			)

			i := NewIBFT(log, backend, transport)
			i.state.name = prepare
			i.state.roundStarted = true
			i.state.proposalMessage = &proto.Message{
				Payload: &proto.Message_PreprepareData{
					PreprepareData: &proto.PrePrepareMessage{
						Proposal:     proposal,
						ProposalHash: proposalHash,
					},
				},
			}
			i.messages = &messages

			// Make sure the notification is present
			notifyCh <- 0

			i.wg.Add(1)
			i.startRound(ctx)

			i.wg.Wait()

			// Make sure the node moves to the commit state
			assert.Equal(t, commit, i.state.name)

			// Make sure the proposal didn't change
			assert.Equal(t, proposal, i.state.getProposal())

			// Make sure the proper proposal hash was multicasted
			assert.True(t, commitHashMatches(proposalHash, multicastedCommit))
		},
	)
}

// TestRunCommit makes sure the node
// behaves correctly in the commit state
func TestRunCommit(t *testing.T) {
	t.Parallel()

	t.Run(
		"validator received quorum of valid commit messages",
		func(t *testing.T) {
			t.Parallel()

			var (
				proposal                                         = []byte("block proposal")
				proposalHash                                     = []byte("proposal hash")
				signer                                           = []byte("signer")
				insertedProposal       []byte                    = nil
				insertedCommittedSeals []*messages.CommittedSeal = nil
				committedSeals                                   = []*messages.CommittedSeal{
					{
						Signer:    signer,
						Signature: generateSeals(1)[0],
					},
				}
				doneReceived = false
				notifyCh     = make(chan uint64, 1)

				log       = mockLogger{}
				transport = mockTransport{}
				backend   = mockBackend{
					insertBlockFn: func(proposal []byte, committedSeals []*messages.CommittedSeal) {
						insertedProposal = proposal
						insertedCommittedSeals = committedSeals
					},
					quorumFn: func(_ uint64) uint64 {
						return 1
					},
					isValidProposalHashFn: func(_ []byte, hash []byte) bool {
						return bytes.Equal(proposalHash, hash)
					},
				}
				messages = mockMessages{
					subscribeFn: func(_ messages.SubscriptionDetails) *messages.Subscription {
						return &messages.Subscription{
							ID:    messages.SubscriptionID(1),
							SubCh: notifyCh,
						}
					},
					getValidMessagesFn: func(
						view *proto.View,
						_ proto.MessageType,
						isValid func(message *proto.Message) bool,
					) []*proto.Message {
						return filterMessages(
							[]*proto.Message{
								{
									View: view,
									Type: proto.MessageType_COMMIT,
									Payload: &proto.Message_CommitData{
										CommitData: &proto.CommitMessage{
											ProposalHash:  proposalHash,
											CommittedSeal: committedSeals[0].Signature,
										},
									},
									From: signer,
								},
							},
							isValid,
						)
					},
				}
			)

			i := NewIBFT(log, backend, transport)
			i.messages = messages
			i.state.proposalMessage = &proto.Message{
				Payload: &proto.Message_PreprepareData{
					PreprepareData: &proto.PrePrepareMessage{
						Proposal:     proposal,
						ProposalHash: proposalHash,
					},
				},
			}
			i.state.roundStarted = true
			i.state.name = commit

			ctx, cancelFn := context.WithCancel(context.Background())

			go func(i *IBFT) {
				defer func() {
					cancelFn()
				}()

				select {
				case <-i.roundDone:
					doneReceived = true
				case <-time.After(5 * time.Second):
					return
				}
			}(i)

			// Make sure the notification is ready
			notifyCh <- 0

			i.wg.Add(1)
			i.startRound(ctx)

			i.wg.Wait()

			// Make sure the node changed the state to fin
			assert.Equal(t, fin, i.state.name)

			// Make sure the inserted proposal was the one present
			assert.Equal(t, insertedProposal, proposal)

			// Make sure the inserted committed seals were correct
			assert.Equal(t, insertedCommittedSeals, committedSeals)

			// Make sure the proper done channel was notified
			assert.True(t, doneReceived)
		},
	)
}

// TestIBFT_IsAcceptableMessage makes sure invalid messages
// are properly handled
func TestIBFT_IsAcceptableMessage(t *testing.T) {
	t.Parallel()

	baseView := &proto.View{
		Height: 0,
		Round:  0,
	}

	testTable := []struct {
		name          string
		view          *proto.View
		currentView   *proto.View
		invalidSender bool
		acceptable    bool
	}{
		{
			"invalid sender",
			nil,
			baseView,
			true,
			false,
		},
		{
			"malformed message",
			nil,
			baseView,
			false,
			false,
		},
		{
			"higher height number",
			&proto.View{
				Height: baseView.Height + 100,
				Round:  baseView.Round,
			},
			baseView,
			false,
			true,
		},
		{
			"higher round number",
			&proto.View{
				Height: baseView.Height,
				Round:  baseView.Round + 1,
			},
			baseView,
			false,
			true,
		},
		{
			"lower height number",
			baseView,
			&proto.View{
				Height: baseView.Height + 1,
				Round:  baseView.Round,
			},
			false,
			false,
		},
	}

	for _, testCase := range testTable {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var (
				log       = mockLogger{}
				transport = mockTransport{}
				backend   = mockBackend{
					isValidSenderFn: func(message *proto.Message) bool {
						return !testCase.invalidSender
					},
				}
			)
			i := NewIBFT(log, backend, transport)
			i.state.view = testCase.currentView

			message := &proto.Message{
				View: testCase.view,
			}

			assert.Equal(t, testCase.acceptable, i.isAcceptableMessage(message))
		})
	}
}

// TestIBFT_StartRoundTimer makes sure that the
// round timer behaves correctly
func TestIBFT_StartRoundTimer(t *testing.T) {
	t.Parallel()

	t.Run("round timer exits due to a quit signal", func(t *testing.T) {
		t.Parallel()

		var (
			wg sync.WaitGroup

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{}
		)

		i := NewIBFT(log, backend, transport)

		ctx, cancelFn := context.WithCancel(context.Background())

		wg.Add(1)
		i.wg.Add(1)
		go func() {
			i.startRoundTimer(ctx, 0)

			wg.Done()
		}()

		cancelFn()

		wg.Wait()
	})

	t.Run("round timer expires", func(t *testing.T) {
		t.Parallel()

		var (
			wg      sync.WaitGroup
			expired = false

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{}
		)

		i := NewIBFT(log, backend, transport)
		i.baseRoundTimeout = 0 * time.Second

		ctx, cancelFn := context.WithCancel(context.Background())

		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()

				cancelFn()
			}()

			select {
			case <-i.roundExpired:
				expired = true
			case <-time.After(5 * time.Second):
			}
		}()

		i.wg.Add(1)
		i.startRoundTimer(ctx, 0)

		wg.Wait()

		// Make sure the round timer expired properly
		assert.True(t, expired)
	})
}

// TestIBFT_MoveToNewRound makes sure the state is modified
// correctly during round moves
func TestIBFT_MoveToNewRound(t *testing.T) {
	t.Parallel()

	t.Run("move to new round", func(t *testing.T) {
		t.Parallel()

		var (
			expectedNewRound uint64 = 1

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{}
		)

		i := NewIBFT(log, backend, transport)

		i.moveToNewRound(expectedNewRound)

		// Make sure the view has changed
		assert.Equal(t, expectedNewRound, i.state.getRound())

		// Make sure the proposal is not present
		assert.Nil(t, i.state.getProposal())

		// Make sure the state is correct
		assert.Equal(t, newRound, i.state.name)
	})
}

// TestIBFT_FutureProposal checks the
// behavior when new proposal messages appear
func TestIBFT_FutureProposal(t *testing.T) {
	t.Parallel()

	nodeID := []byte("node ID")
	proposer := []byte("proposer")
	proposal := []byte("proposal")
	proposalHash := []byte("proposal hash")
	quorum := uint64(4)

	generateEmptyRCMessages := func(count uint64) []*proto.Message {
		// Generate random RC messages
		roundChangeMessages := generateMessages(count, proto.MessageType_ROUND_CHANGE)

		// Fill up their certificates
		for _, message := range roundChangeMessages {
			message.Payload = &proto.Message_RoundChangeData{
				RoundChangeData: &proto.RoundChangeMessage{
					LastPreparedProposedBlock: nil,
					LatestPreparedCertificate: nil,
				},
			}
		}

		return roundChangeMessages
	}

	testTable := []struct {
		name                string
		proposalView        *proto.View
		roundChangeMessages []*proto.Message
		notifyRound         uint64
	}{
		{
			"valid future proposal with new block",
			&proto.View{
				Height: 0,
				Round:  1,
			},
			generateEmptyRCMessages(quorum),
			1,
		},
		{
			"valid future proposal with old block",
			&proto.View{
				Height: 0,
				Round:  2,
			},
			generateFilledRCMessages(quorum, proposal, proposalHash),
			2,
		},
	}

	for _, testCase := range testTable {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancelFn := context.WithCancel(context.Background())

			validProposal := &proto.Message{
				View: testCase.proposalView,
				From: proposer,
				Type: proto.MessageType_PREPREPARE,
				Payload: &proto.Message_PreprepareData{
					PreprepareData: &proto.PrePrepareMessage{
						Proposal:     proposal,
						ProposalHash: proposalHash,
						Certificate: &proto.RoundChangeCertificate{
							RoundChangeMessages: testCase.roundChangeMessages,
						},
					},
				},
			}

			var (
				wg                    sync.WaitGroup
				receivedProposalEvent *newProposalEvent = nil
				notifyCh                                = make(chan uint64, 1)

				log     = mockLogger{}
				backend = mockBackend{
					isProposerFn: func(id []byte, _ uint64, _ uint64) bool {
						return !bytes.Equal(id, nodeID)
					},
					idFn: func() []byte {
						return nodeID
					},
					isValidProposalHashFn: func(p []byte, hash []byte) bool {
						return bytes.Equal(hash, proposalHash) && bytes.Equal(p, proposal)
					},
					quorumFn: func(_ uint64) uint64 {
						return quorum
					},
				}
				transport = mockTransport{}
				mMessages = mockMessages{
					subscribeFn: func(_ messages.SubscriptionDetails) *messages.Subscription {
						return &messages.Subscription{
							ID:    messages.SubscriptionID(1),
							SubCh: notifyCh,
						}
					},
					getValidMessagesFn: func(
						view *proto.View,
						_ proto.MessageType,
						isValid func(message *proto.Message) bool,
					) []*proto.Message {
						return filterMessages(
							[]*proto.Message{
								validProposal,
							},
							isValid,
						)
					},
				}
			)

			i := NewIBFT(log, backend, transport)
			i.messages = mMessages

			wg.Add(1)
			go func() {
				defer func() {
					cancelFn()

					wg.Done()
				}()

				select {
				case <-time.After(5 * time.Second):
				case event := <-i.newProposal:
					receivedProposalEvent = &event
				}
			}()

			notifyCh <- testCase.notifyRound

			i.wg.Add(1)
			i.watchForFutureProposal(ctx)

			wg.Wait()

			// Make sure the received proposal is the one that was expected
			if receivedProposalEvent == nil {
				t.Fatalf("no proposal event received")
			}

			assert.Equal(t, testCase.notifyRound, receivedProposalEvent.round)
			assert.Equal(t, proposal, messages.ExtractProposal(receivedProposalEvent.proposalMessage))
		})
	}
}

// TestIBFT_ValidPC validates that prepared certificates
// are verified correctly
func TestIBFT_ValidPC(t *testing.T) {
	t.Parallel()

	t.Run("no certificate", func(t *testing.T) {
		t.Parallel()

		var (
			certificate *proto.PreparedCertificate = nil

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{}
		)

		i := NewIBFT(log, backend, transport)

		assert.True(t, i.validPC(certificate, 0, 0))
	})

	t.Run("proposal and prepare messages mismatch", func(t *testing.T) {
		t.Parallel()

		var (
			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{}
		)

		i := NewIBFT(log, backend, transport)

		certificate := &proto.PreparedCertificate{
			ProposalMessage: nil,
			PrepareMessages: make([]*proto.Message, 0),
		}

		assert.False(t, i.validPC(certificate, 0, 0))

		certificate = &proto.PreparedCertificate{
			ProposalMessage: &proto.Message{},
			PrepareMessages: nil,
		}

		assert.False(t, i.validPC(certificate, 0, 0))
	})

	t.Run("no Quorum PP + P messages", func(t *testing.T) {
		t.Parallel()

		var (
			quorum = uint64(4)

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		certificate := &proto.PreparedCertificate{
			ProposalMessage: &proto.Message{},
			PrepareMessages: generateMessages(quorum-2, proto.MessageType_PREPARE),
		}

		assert.False(t, i.validPC(certificate, 0, 0))
	})

	t.Run("invalid proposal message type", func(t *testing.T) {
		t.Parallel()

		var (
			quorum = uint64(4)

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		certificate := &proto.PreparedCertificate{
			ProposalMessage: &proto.Message{
				Type: proto.MessageType_PREPARE,
			},
			PrepareMessages: generateMessages(quorum-1, proto.MessageType_PREPARE),
		}

		assert.False(t, i.validPC(certificate, 0, 0))
	})

	t.Run("invalid prepare message type", func(t *testing.T) {
		t.Parallel()

		var (
			quorum = uint64(4)

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		certificate := &proto.PreparedCertificate{
			ProposalMessage: &proto.Message{
				Type: proto.MessageType_PREPREPARE,
			},
			PrepareMessages: generateMessages(quorum-1, proto.MessageType_PREPARE),
		}

		// Make sure one of the messages has an invalid type
		certificate.PrepareMessages[0].Type = proto.MessageType_ROUND_CHANGE

		assert.False(t, i.validPC(certificate, 0, 0))
	})

	t.Run("non unique senders", func(t *testing.T) {
		t.Parallel()

		var (
			quorum = uint64(4)
			sender = []byte("node x")

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		certificate := &proto.PreparedCertificate{
			ProposalMessage: &proto.Message{
				Type: proto.MessageType_PREPREPARE,
				From: sender,
			},
			PrepareMessages: generateMessagesWithSender(quorum-1, proto.MessageType_PREPARE, sender),
		}

		assert.False(t, i.validPC(certificate, 0, 0))
	})

	t.Run("differing proposal hashes", func(t *testing.T) {
		t.Parallel()

		var (
			quorum = uint64(4)
			sender = []byte("unique node")

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		proposal := generateMessagesWithSender(1, proto.MessageType_PREPREPARE, sender)[0]

		certificate := &proto.PreparedCertificate{
			ProposalMessage: proposal,
			PrepareMessages: generateMessagesWithUniqueSender(quorum-1, proto.MessageType_PREPARE),
		}

		// Make sure the proposal has a different hash than the prepare messages
		appendProposalHash([]*proto.Message{certificate.ProposalMessage}, []byte("proposal hash 1"))
		appendProposalHash(certificate.PrepareMessages, []byte("proposal hash 2"))

		assert.False(t, i.validPC(certificate, 0, 0))
	})

	t.Run("rounds not lower than rLimit", func(t *testing.T) {
		t.Parallel()

		var (
			quorum       = uint64(4)
			rLimit       = uint64(1)
			sender       = []byte("unique node")
			proposalHash = []byte("proposal hash")

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		proposal := generateMessagesWithSender(1, proto.MessageType_PREPREPARE, sender)[0]

		certificate := &proto.PreparedCertificate{
			ProposalMessage: proposal,
			PrepareMessages: generateMessagesWithUniqueSender(quorum-1, proto.MessageType_PREPARE),
		}

		// Make sure they all have the same proposal hash
		allMessages := append([]*proto.Message{certificate.ProposalMessage}, certificate.PrepareMessages...)
		appendProposalHash(
			allMessages,
			proposalHash,
		)

		setRoundForMessages(allMessages, rLimit+1)

		assert.False(t, i.validPC(certificate, rLimit, 0))
	})

	t.Run("heights are not the same", func(t *testing.T) {
		t.Parallel()

		var (
			quorum       = uint64(4)
			rLimit       = uint64(1)
			sender       = []byte("unique node")
			proposalHash = []byte("proposal hash")

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
				isProposerFn: func(proposer []byte, _ uint64, _ uint64) bool {
					return !bytes.Equal(proposer, sender)
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		proposal := generateMessagesWithSender(1, proto.MessageType_PREPREPARE, sender)[0]

		// Make sure the height is invalid for the proposal
		proposal.View.Height = 10

		certificate := &proto.PreparedCertificate{
			ProposalMessage: proposal,
			PrepareMessages: generateMessagesWithUniqueSender(quorum-1, proto.MessageType_PREPARE),
		}

		// Make sure they all have the same proposal hash
		allMessages := append([]*proto.Message{certificate.ProposalMessage}, certificate.PrepareMessages...)
		appendProposalHash(
			allMessages,
			proposalHash,
		)

		setRoundForMessages(allMessages, rLimit-1)

		assert.False(t, i.validPC(certificate, rLimit, 0))
	})

	t.Run("proposal not from proposer", func(t *testing.T) {
		t.Parallel()

		var (
			quorum       = uint64(4)
			rLimit       = uint64(1)
			sender       = []byte("unique node")
			proposalHash = []byte("proposal hash")

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
				isProposerFn: func(proposer []byte, _ uint64, _ uint64) bool {
					return !bytes.Equal(proposer, sender)
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		proposal := generateMessagesWithSender(1, proto.MessageType_PREPREPARE, sender)[0]

		certificate := &proto.PreparedCertificate{
			ProposalMessage: proposal,
			PrepareMessages: generateMessagesWithUniqueSender(quorum-1, proto.MessageType_PREPARE),
		}

		// Make sure they all have the same proposal hash
		allMessages := append([]*proto.Message{certificate.ProposalMessage}, certificate.PrepareMessages...)
		appendProposalHash(
			allMessages,
			proposalHash,
		)

		setRoundForMessages(allMessages, rLimit-1)

		assert.False(t, i.validPC(certificate, rLimit, 0))
	})

	t.Run("prepare is from an invalid sender", func(t *testing.T) {
		t.Parallel()

		var (
			quorum       = uint64(4)
			rLimit       = uint64(1)
			sender       = []byte("unique node")
			proposalHash = []byte("proposal hash")

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
				isProposerFn: func(proposer []byte, _ uint64, _ uint64) bool {
					return bytes.Equal(proposer, sender)
				},
				isValidSenderFn: func(message *proto.Message) bool {
					// One of the messages will be invalid
					return !bytes.Equal(message.From, []byte("node 1"))
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		proposal := generateMessagesWithSender(1, proto.MessageType_PREPREPARE, sender)[0]

		certificate := &proto.PreparedCertificate{
			ProposalMessage: proposal,
			PrepareMessages: generateMessagesWithUniqueSender(quorum-1, proto.MessageType_PREPARE),
		}

		// Make sure they all have the same proposal hash
		allMessages := append([]*proto.Message{certificate.ProposalMessage}, certificate.PrepareMessages...)
		appendProposalHash(
			allMessages,
			proposalHash,
		)

		setRoundForMessages(allMessages, rLimit-1)

		assert.False(t, i.validPC(certificate, rLimit, 0))
	})

	t.Run("completely valid PC", func(t *testing.T) {
		t.Parallel()

		var (
			quorum       = uint64(4)
			rLimit       = uint64(1)
			sender       = []byte("unique node")
			proposalHash = []byte("proposal hash")

			log       = mockLogger{}
			transport = mockTransport{}
			backend   = mockBackend{
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
				isProposerFn: func(proposer []byte, _ uint64, _ uint64) bool {
					return bytes.Equal(proposer, sender)
				},
				isValidSenderFn: func(message *proto.Message) bool {
					return true
				},
			}
		)

		i := NewIBFT(log, backend, transport)

		proposal := generateMessagesWithSender(1, proto.MessageType_PREPREPARE, sender)[0]

		certificate := &proto.PreparedCertificate{
			ProposalMessage: proposal,
			PrepareMessages: generateMessagesWithUniqueSender(quorum-1, proto.MessageType_PREPARE),
		}

		// Make sure they all have the same proposal hash
		allMessages := append([]*proto.Message{certificate.ProposalMessage}, certificate.PrepareMessages...)
		appendProposalHash(
			allMessages,
			proposalHash,
		)

		setRoundForMessages(allMessages, rLimit-1)

		assert.True(t, i.validPC(certificate, rLimit, 0))
	})
}

func TestIBFT_ValidateProposal(t *testing.T) {
	t.Parallel()

	t.Run("proposer is not valid", func(t *testing.T) {
		t.Parallel()

		var (
			log     = mockLogger{}
			backend = mockBackend{
				isProposerFn: func(_ []byte, _ uint64, _ uint64) bool {
					return false
				},
			}
			transport = mockTransport{}
		)

		i := NewIBFT(log, backend, transport)

		baseView := &proto.View{
			Height: 0,
			Round:  0,
		}
		proposal := &proto.Message{
			View: baseView,
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{},
			},
		}

		assert.False(t, i.validateProposal(proposal, baseView))
	})

	t.Run("block is not valid", func(t *testing.T) {
		t.Parallel()

		var (
			log     = mockLogger{}
			backend = mockBackend{
				isProposerFn: func(_ []byte, _ uint64, _ uint64) bool {
					return true
				},
				isValidBlockFn: func(_ []byte) bool {
					return false
				},
			}
			transport = mockTransport{}
		)

		i := NewIBFT(log, backend, transport)

		baseView := &proto.View{
			Height: 0,
			Round:  0,
		}
		proposal := &proto.Message{
			View: baseView,
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{},
			},
		}

		assert.False(t, i.validateProposal(proposal, baseView))
	})

	t.Run("proposal hash is not valid", func(t *testing.T) {
		t.Parallel()

		var (
			log     = mockLogger{}
			backend = mockBackend{
				isValidProposalHashFn: func(_ []byte, _ []byte) bool {
					return false
				},
				isProposerFn: func(_ []byte, _ uint64, _ uint64) bool {
					return true
				},
			}
			transport = mockTransport{}
		)

		i := NewIBFT(log, backend, transport)

		baseView := &proto.View{
			Height: 0,
			Round:  0,
		}
		proposal := &proto.Message{
			View: baseView,
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{},
			},
		}

		assert.False(t, i.validateProposal(proposal, baseView))
	})

	t.Run("certificate is not present", func(t *testing.T) {
		t.Parallel()

		var (
			log     = mockLogger{}
			backend = mockBackend{
				isProposerFn: func(_ []byte, _ uint64, _ uint64) bool {
					return true
				},
			}
			transport = mockTransport{}
		)

		i := NewIBFT(log, backend, transport)

		baseView := &proto.View{
			Height: 0,
			Round:  0,
		}
		proposal := &proto.Message{
			View: baseView,
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{
					Certificate: nil,
				},
			},
		}

		assert.False(t, i.validateProposal(proposal, baseView))
	})

	t.Run("there are < quorum RC messages in the certificate", func(t *testing.T) {
		t.Parallel()

		var (
			quorum = uint64(4)

			log     = mockLogger{}
			backend = mockBackend{
				isProposerFn: func(_ []byte, _ uint64, _ uint64) bool {
					return true
				},
				quorumFn: func(_ uint64) uint64 {
					return quorum
				},
			}
			transport = mockTransport{}
		)

		i := NewIBFT(log, backend, transport)

		baseView := &proto.View{
			Height: 0,
			Round:  0,
		}
		proposal := &proto.Message{
			View: baseView,
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{
					Certificate: &proto.RoundChangeCertificate{
						RoundChangeMessages: generateMessages(quorum-1, proto.MessageType_ROUND_CHANGE),
					},
				},
			},
		}

		assert.False(t, i.validateProposal(proposal, baseView))
	})

	t.Run("current node should not be the proposer", func(t *testing.T) {
		t.Parallel()

		var (
			quorum     = uint64(4)
			id         = []byte("node id")
			uniqueNode = []byte("unique node")

			log     = mockLogger{}
			backend = mockBackend{
				idFn: func() []byte {
					return id
				},
				isProposerFn: func(proposer []byte, _ uint64, _ uint64) bool {
					if bytes.Equal(proposer, uniqueNode) {
						return true
					}

					return bytes.Equal(proposer, id)
				},
			}
			transport = mockTransport{}
		)

		i := NewIBFT(log, backend, transport)

		baseView := &proto.View{
			Height: 0,
			Round:  0,
		}
		proposal := &proto.Message{
			View: baseView,
			From: uniqueNode,
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{
					Certificate: &proto.RoundChangeCertificate{
						RoundChangeMessages: generateMessages(quorum, proto.MessageType_ROUND_CHANGE),
					},
				},
			},
		}

		assert.False(t, i.validateProposal(proposal, baseView))
	})

	t.Run("current node should not be the proposer", func(t *testing.T) {
		t.Parallel()

		var (
			quorum = uint64(4)
			id     = []byte("node id")

			log     = mockLogger{}
			backend = mockBackend{
				idFn: func() []byte {
					return id
				},
				isProposerFn: func(proposer []byte, _ uint64, _ uint64) bool {
					return bytes.Equal(proposer, id)
				},
			}
			transport = mockTransport{}
		)

		i := NewIBFT(log, backend, transport)

		baseView := &proto.View{
			Height: 0,
			Round:  0,
		}
		proposal := &proto.Message{
			View: baseView,
			Type: proto.MessageType_PREPREPARE,
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{
					Certificate: &proto.RoundChangeCertificate{
						RoundChangeMessages: generateMessages(quorum, proto.MessageType_ROUND_CHANGE),
					},
				},
			},
		}

		assert.False(t, i.validateProposal(proposal, baseView))
	})
}

// TestIBFT_WatchForFutureRCC verifies that future RCC
// are handled properly
func TestIBFT_WatchForFutureRCC(t *testing.T) {
	t.Parallel()

	quorum := uint64(4)
	proposal := []byte("proposal")
	rccRound := uint64(10)
	proposalHash := []byte("proposal hash")

	roundChangeMessages := generateFilledRCMessages(quorum, proposal, proposalHash)
	setRoundForMessages(roundChangeMessages, rccRound)

	var (
		receivedRound = uint64(0)
		notifyCh      = make(chan uint64, 1)

		log       = mockLogger{}
		transport = mockTransport{}
		backend   = mockBackend{
			quorumFn: func(_ uint64) uint64 {
				return quorum
			},
			isProposerFn: func(_ []byte, _ uint64, _ uint64) bool {
				return true
			},
		}
		messages = mockMessages{
			subscribeFn: func(_ messages.SubscriptionDetails) *messages.Subscription {
				return &messages.Subscription{
					ID:    messages.SubscriptionID(1),
					SubCh: notifyCh,
				}
			},
			getValidMessagesFn: func(
				view *proto.View,
				messageType proto.MessageType,
				isValid func(message *proto.Message) bool,
			) []*proto.Message {
				return filterMessages(
					roundChangeMessages,
					isValid,
				)
			},
		}
	)

	i := NewIBFT(log, backend, transport)
	i.messages = messages

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	go func() {
		defer cancelFn()

		select {
		case r := <-i.roundCertificate:
			receivedRound = r
		case <-time.After(5 * time.Second):
		}
	}()

	// Have the notification waiting
	notifyCh <- rccRound

	i.wg.Add(1)
	i.watchForRoundChangeCertificates(ctx)
	i.wg.Wait()

	// Make sure the notification round was correct
	assert.Equal(t, rccRound, receivedRound)
}

// TestState_String makes sure the string representation
// of states is correct
func TestState_String(t *testing.T) {
	stringMap := map[stateType]string{
		newRound: "new round",
		prepare:  "prepare",
		commit:   "commit",
		fin:      "fin",
	}

	stateTypes := []stateType{
		newRound,
		prepare,
		commit,
		fin,
	}

	for _, stateT := range stateTypes {
		assert.Equal(t, stringMap[stateT], stateT.String())
	}
}

// TestIBFT_RunSequence_NewProposal verifies that the
// state changes correctly when receiving a higher proposal event
func TestIBFT_RunSequence_NewProposal(t *testing.T) {
	t.Parallel()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	var (
		proposal = []byte("proposal")
		round    = uint64(10)
		height   = uint64(1)
		quorum   = uint64(4)

		log     = mockLogger{}
		backend = mockBackend{
			quorumFn: func(_ uint64) uint64 {
				return quorum
			},
		}
		transport = mockTransport{}
	)

	i := NewIBFT(log, backend, transport)
	i.newProposal = make(chan newProposalEvent, 1)

	ev := newProposalEvent{
		proposalMessage: &proto.Message{
			Payload: &proto.Message_PreprepareData{
				PreprepareData: &proto.PrePrepareMessage{
					Proposal: proposal,
				},
			},
		},
		round: round,
	}

	// Make sure the event is waiting
	i.newProposal <- ev

	// Spawn a go-routine that's going to turn off the sequence after 1s
	go func() {
		defer cancelFn()

		<-time.After(1 * time.Second)
	}()

	i.RunSequence(ctx, height)

	// Make sure the correct proposal message was accepted
	assert.Equal(t, ev.proposalMessage, i.state.proposalMessage)

	// Make sure the correct round was moved to
	assert.Equal(t, ev.round, i.state.view.Round)
	assert.Equal(t, height, i.state.view.Height)

	// Make sure the round has been started
	assert.True(t, i.state.roundStarted)

	// Make sure the state is the prepare state
	assert.Equal(t, prepare, i.state.name)
}

// TestIBFT_RunSequence_FutureRCC verifies that the
// state changes correctly when receiving a higher RCC event
func TestIBFT_RunSequence_FutureRCC(t *testing.T) {
	t.Parallel()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	var (
		round  = uint64(10)
		height = uint64(1)
		quorum = uint64(4)

		log     = mockLogger{}
		backend = mockBackend{
			quorumFn: func(_ uint64) uint64 {
				return quorum
			},
		}
		transport = mockTransport{}
	)

	i := NewIBFT(log, backend, transport)
	i.roundCertificate = make(chan uint64, 1)

	// Make sure the round event is waiting
	i.roundCertificate <- round

	// Spawn a go-routine that's going to turn off the sequence after 1s
	go func() {
		defer cancelFn()

		<-time.After(1 * time.Second)
	}()

	i.RunSequence(ctx, height)

	// Make sure the proposal message is not set
	assert.Nil(t, i.state.proposalMessage)

	// Make sure the correct round was moved to
	assert.Equal(t, round, i.state.view.Round)
	assert.Equal(t, height, i.state.view.Height)

	// Make sure the new round has been started
	assert.True(t, i.state.roundStarted)

	// Make sure the state is the new round state
	assert.Equal(t, newRound, i.state.name)
}

// TestIBFT_ExtendRoundTimer makes sure the round timeout
// is extended correctly
func TestIBFT_ExtendRoundTimer(t *testing.T) {
	t.Parallel()

	var (
		additionalTimeout = 10 * time.Second

		log       = mockLogger{}
		backend   = mockBackend{}
		transport = mockTransport{}
	)

	i := NewIBFT(log, backend, transport)

	i.ExtendRoundTimeout(additionalTimeout)

	// Make sure the round timeout was extended
	assert.Equal(t, additionalTimeout, i.additionalTimeout)
}
