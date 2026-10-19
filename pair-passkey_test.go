// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waCompanionReg"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestPairPasskey_PasskeyHandoffKeyValid(t *testing.T) {
	var nilKey *passkeyHandoffKey
	assert.False(t, nilKey.Valid())

	validKey := &passkeyHandoffKey{
		hmac: []byte("hmac"),
		ts:   time.Now().Add(-1 * time.Minute),
	}
	assert.True(t, validKey.Valid())

	expiredKey := &passkeyHandoffKey{
		hmac: []byte("hmac"),
		ts:   time.Now().Add(-6 * time.Minute),
	}
	assert.False(t, expiredKey.Valid())
}

func TestPairPasskey_HandlePasskeyNotification(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{AdvSecretKey: make([]byte, 32)}, waLog.Noop)

	t.Run("Non-server JID ignored", func(t *testing.T) {
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.NewJID("123", types.DefaultUserServer)},
		}
		cli.handlePasskeyNotification(ctx, node)
		assert.Nil(t, cli.passkeyHandoffKey.Load())
	})

	t.Run("Valid notification parsed directly", func(t *testing.T) {
		pkJSON, err := json.Marshal(&types.WebAuthnPublicKey{
			Challenge:     []byte("challenge-123"),
			RelyingPartID: "whatsapp.com",
		})
		require.NoError(t, err)

		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.ServerJID},
			Content: []waBinary.Node{{
				Tag:     "passkey_request_options",
				Content: pkJSON,
			}},
		}

		var dispatched *events.PairPasskeyRequest
		cli.AddEventHandler(func(evt any) {
			if req, ok := evt.(*events.PairPasskeyRequest); ok {
				dispatched = req
			}
		})

		cli.handlePasskeyNotification(ctx, node)
		require.NotNil(t, dispatched)
		assert.Equal(t, []byte("challenge-123"), []byte(dispatched.PublicKey.Challenge))
		handoff := cli.passkeyHandoffKey.Load()
		require.NotNil(t, handoff)
		assert.True(t, handoff.Valid())
	})

	t.Run("Notification fallback to getPasskeyRequestOptions on parse error", func(t *testing.T) {
		badNode := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.ServerJID},
			Content: []waBinary.Node{{
				Tag:     "passkey_request_options",
				Content: []byte("invalid-json"),
			}},
		}

		pkJSON, _ := json.Marshal(&types.WebAuthnPublicKey{
			Challenge: []byte("fallback-challenge"),
		})
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "md", query.Namespace)
			assert.Equal(t, iqGet, query.Type)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:     "passkey_request_options",
					Content: pkJSON,
				}},
			}, nil
		}

		var dispatched *events.PairPasskeyRequest
		cli.AddEventHandler(func(evt any) {
			if req, ok := evt.(*events.PairPasskeyRequest); ok {
				dispatched = req
			}
		})

		cli.handlePasskeyNotification(ctx, badNode)
		require.NotNil(t, dispatched)
		assert.Equal(t, []byte("fallback-challenge"), []byte(dispatched.PublicKey.Challenge))
	})

	t.Run("Notification fallback failure dispatches PairPasskeyError", func(t *testing.T) {
		badNode := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.ServerJID},
			Content: []waBinary.Node{{
				Tag: "invalid_missing_options",
			}},
		}

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("fallback network fail")
		}

		var errEvt *events.PairPasskeyError
		cli.AddEventHandler(func(evt any) {
			if pErr, ok := evt.(*events.PairPasskeyError); ok {
				errEvt = pErr
			}
		})

		cli.handlePasskeyNotification(ctx, badNode)
		require.NotNil(t, errEvt)
		assert.False(t, errEvt.Continuation)
		assert.Contains(t, errEvt.Error.Error(), "fetching key also failed")
	})
}

func TestPairPasskey_SendPasskeyResponse(t *testing.T) {
	ctx := context.Background()
	dev := &store.Device{
		NoiseKey: keys.NewKeyPair(),
	}
	cli := NewClient(dev, waLog.Noop)

	passkeyResp := &types.WebAuthnResponse{
		RawID: []byte("credential-id-123"),
		Response: types.WebAuthnResponseData{
			ClientDataJSON: []byte(`{"type":"webauthn.get"}`),
		},
	}

	t.Run("Get companion ref failure", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("ref fail")
		}
		err := cli.SendPasskeyResponse(ctx, passkeyResp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get companion ref")
	})

	t.Run("Send prologue success with handoff key", func(t *testing.T) {
		cli.passkeyHandoffKey.Store(&passkeyHandoffKey{
			hmac: []byte("01234567890123456789012345678901"),
			ts:   time.Now(),
		})

		var capturedQueries []infoQuery
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			capturedQueries = append(capturedQueries, query)
			if query.Type == iqGet {
				return &waBinary.Node{
					Tag: "iq",
					Content: []waBinary.Node{{
						Tag:     "ref",
						Content: []byte("companion-ref-abc"),
					}},
				}, nil
			}
			return &waBinary.Node{Tag: "iq"}, nil
		}

		err := cli.SendPasskeyResponse(ctx, passkeyResp)
		require.NoError(t, err)
		require.Len(t, capturedQueries, 2)

		// First was get ref, second was set passkey_prologue
		prologueQuery := capturedQueries[1]
		prologueNodes, ok := prologueQuery.Content.([]waBinary.Node)
		require.True(t, ok)
		require.Len(t, prologueNodes, 1)
		prologueNode := prologueNodes[0]
		assert.Equal(t, "passkey_prologue", prologueNode.Tag)
		assert.NotNil(t, prologueNode.GetChildByTag("pairing_handoff_proof"))
		assert.True(t, cli.passkeySkipHandoffUX.Load())

		// Cache should be set
		cache := cli.passkeyLinkingCache.Load()
		require.NotNil(t, cache)
		assert.Equal(t, "companion-ref-abc", cache.pairingRef)
	})

	t.Run("Send prologue success without valid handoff key", func(t *testing.T) {
		cli.passkeyHandoffKey.Store(nil)

		var capturedQueries []infoQuery
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			capturedQueries = append(capturedQueries, query)
			if query.Type == iqGet {
				return &waBinary.Node{
					Tag: "iq",
					Content: []waBinary.Node{{
						Tag:     "ref",
						Content: []byte("companion-ref-xyz"),
					}},
				}, nil
			}
			return &waBinary.Node{Tag: "iq"}, nil
		}

		err := cli.SendPasskeyResponse(ctx, passkeyResp)
		require.NoError(t, err)
		assert.False(t, cli.passkeySkipHandoffUX.Load())
	})

	t.Run("Send prologue sendIQ error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			if query.Type == iqGet {
				return &waBinary.Node{
					Tag: "iq",
					Content: []waBinary.Node{{
						Tag:     "ref",
						Content: []byte("companion-ref-xyz"),
					}},
				}, nil
			}
			return nil, errors.New("prologue iq rejected")
		}

		err := cli.SendPasskeyResponse(ctx, passkeyResp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send passkey response")
	})
}

func TestPairPasskey_HandleContinuation(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("Non-server JID ignored in tryHandlePasskeyContinuationNotification", func(t *testing.T) {
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.NewJID("123", types.DefaultUserServer)},
		}
		cli.tryHandlePasskeyContinuationNotification(ctx, node)
	})

	t.Run("Without linking cache returns error", func(t *testing.T) {
		cli.passkeyLinkingCache.Store(nil)
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.ServerJID},
		}

		var errEvt *events.PairPasskeyError
		cli.AddEventHandler(func(evt any) {
			if pe, ok := evt.(*events.PairPasskeyError); ok {
				errEvt = pe
			}
		})

		cli.tryHandlePasskeyContinuationNotification(ctx, node)
		require.NotNil(t, errEvt)
		assert.True(t, errEvt.Continuation)
		assert.Contains(t, errEvt.Error.Error(), "without a linking cache")
	})

	t.Run("Continuation success flow", func(t *testing.T) {
		companionEphemeralKeyPair := keys.NewKeyPair()
		primaryEphemeralKeyPair := keys.NewKeyPair()

		companionNonce := make([]byte, 32)
		primaryNonce := make([]byte, 32)
		for i := range companionNonce {
			companionNonce[i] = byte(i)
			primaryNonce[i] = byte(i * 2)
		}

		cli.passkeyLinkingCache.Store(&passkeyLinkingCache{
			keyPair:        companionEphemeralKeyPair,
			companionNonce: companionNonce,
			pairingRef:     "ref-123",
			deviceType:     waCompanionReg.DeviceProps_CHROME,
		})
		cli.passkeySkipHandoffUX.Store(true)

		primaryIdentBytes, err := proto.Marshal(&waCompanionReg.PrimaryEphemeralIdentity{
			PublicKey: primaryEphemeralKeyPair.Pub[:],
			Nonce:     primaryNonce,
		})
		require.NoError(t, err)

		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.ServerJID},
			Content: []waBinary.Node{{
				Tag:     "primary_ephemeral_identity",
				Content: primaryIdentBytes,
			}},
		}

		var sentIQ infoQuery
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			sentIQ = query
			return &waBinary.Node{Tag: "iq"}, nil
		}

		var confirmationEvt *events.PairPasskeyConfirmation
		cli.AddEventHandler(func(evt any) {
			if ce, ok := evt.(*events.PairPasskeyConfirmation); ok {
				confirmationEvt = ce
			}
		})

		err = cli.handlePasskeyContinuationNotification(ctx, node)
		require.NoError(t, err)

		// Check companion nonce IQ sent
		assert.Equal(t, "md", sentIQ.Namespace)
		assert.Equal(t, iqSet, sentIQ.Type)
		sentNodes, ok := sentIQ.Content.([]waBinary.Node)
		require.True(t, ok)
		require.Len(t, sentNodes, 1)
		assert.Equal(t, companionNonce, sentNodes[0].Content)

		// Check confirmation event dispatched
		require.NotNil(t, confirmationEvt)
		assert.NotEmpty(t, confirmationEvt.Code)
		assert.Contains(t, confirmationEvt.Code, "-")
		assert.True(t, confirmationEvt.SkipHandoffUX)

		// Verify encryptionKey generated in cache
		cache := cli.passkeyLinkingCache.Load()
		require.NotNil(t, cache)
		assert.Len(t, cache.encryptionKey, 32)
	})

	t.Run("Continuation send companion nonce failure", func(t *testing.T) {
		companionEphemeralKeyPair := keys.NewKeyPair()
		primaryEphemeralKeyPair := keys.NewKeyPair()
		cli.passkeyLinkingCache.Store(&passkeyLinkingCache{
			keyPair:        companionEphemeralKeyPair,
			companionNonce: make([]byte, 32),
			pairingRef:     "ref-123",
		})

		primaryIdentBytes, _ := proto.Marshal(&waCompanionReg.PrimaryEphemeralIdentity{
			PublicKey: primaryEphemeralKeyPair.Pub[:],
			Nonce:     make([]byte, 32),
		})
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.ServerJID},
			Content: []waBinary.Node{{
				Tag:     "primary_ephemeral_identity",
				Content: primaryIdentBytes,
			}},
		}

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("companion nonce fail")
		}

		err := cli.handlePasskeyContinuationNotification(ctx, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send companion nonce")
	})
}

func TestPairPasskey_SendPasskeyConfirmation(t *testing.T) {
	ctx := context.Background()
	dev := &store.Device{
		NoiseKey:     keys.NewKeyPair(),
		IdentityKey:  keys.NewKeyPair(),
		AdvSecretKey: make([]byte, 32),
	}
	cli := NewClient(dev, waLog.Noop)

	t.Run("No linking cache", func(t *testing.T) {
		cli.passkeyLinkingCache.Store(nil)
		err := cli.SendPasskeyConfirmation(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no passkey linking cache available")
	})

	t.Run("Cache missing encryption key", func(t *testing.T) {
		cli.passkeyLinkingCache.Store(&passkeyLinkingCache{
			encryptionKey: nil,
		})
		err := cli.SendPasskeyConfirmation(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not have an encryption key yet")
	})

	t.Run("SendPasskeyConfirmation success", func(t *testing.T) {
		cli.passkeyLinkingCache.Store(&passkeyLinkingCache{
			encryptionKey: make([]byte, 32),
		})

		var sentIQ infoQuery
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			sentIQ = query
			return &waBinary.Node{Tag: "iq"}, nil
		}

		err := cli.SendPasskeyConfirmation(ctx)
		require.NoError(t, err)

		assert.Equal(t, "md", sentIQ.Namespace)
		assert.Equal(t, iqSet, sentIQ.Type)
		nodes, ok := sentIQ.Content.([]waBinary.Node)
		require.True(t, ok)
		require.Len(t, nodes, 1)
		assert.Equal(t, "encrypted_pairing_request", nodes[0].Tag)
		assert.NotEmpty(t, nodes[0].Content)

		// Cache cleared on success
		assert.Nil(t, cli.passkeyLinkingCache.Load())
	})

	t.Run("SendPasskeyConfirmation IQ error", func(t *testing.T) {
		cli.passkeyLinkingCache.Store(&passkeyLinkingCache{
			encryptionKey: make([]byte, 32),
		})

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("confirmation iq fail")
		}

		err := cli.SendPasskeyConfirmation(ctx)
		require.Error(t, err)
	})
}

func TestPairPasskey_Helpers_Errors(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("getCompanionRef missing ref tag", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.getCompanionRef(ctx)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "ref", elemErr.Tag)
	})

	t.Run("getCompanionRef ref content not bytes", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:     "ref",
					Content: 12345,
				}},
			}, nil
		}
		_, err := cli.getCompanionRef(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected content type int for <ref> node")
	})

	t.Run("parsePasskeyNotification missing tag", func(t *testing.T) {
		node := &waBinary.Node{Tag: "iq"}
		_, err := parsePasskeyNotification(node)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
	})

	t.Run("parsePasskeyNotification content not bytes", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{{
				Tag:     "passkey_request_options",
				Content: 999,
			}},
		}
		_, err := parsePasskeyNotification(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected content type int")
	})

	t.Run("parsePasskeyContinuationNotification missing tag", func(t *testing.T) {
		node := &waBinary.Node{Tag: "iq"}
		_, err := parsePasskeyContinuationNotification(node)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
	})

	t.Run("parsePasskeyContinuationNotification content not bytes", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{{
				Tag:     "primary_ephemeral_identity",
				Content: "not-bytes",
			}},
		}
		_, err := parsePasskeyContinuationNotification(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected content type string")
	})

	t.Run("parsePasskeyContinuationNotification invalid proto", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{{
				Tag:     "primary_ephemeral_identity",
				Content: []byte("malformed-proto"),
			}},
		}
		_, err := parsePasskeyContinuationNotification(node)
		require.Error(t, err)
	})

	t.Run("parsePasskeyContinuationNotification invalid pubkey length", func(t *testing.T) {
		identBytes, _ := proto.Marshal(&waCompanionReg.PrimaryEphemeralIdentity{
			PublicKey: []byte{1, 2, 3},
			Nonce:     make([]byte, 32),
		})
		node := &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{{
				Tag:     "primary_ephemeral_identity",
				Content: identBytes,
			}},
		}
		_, err := parsePasskeyContinuationNotification(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected public key length")
	})

	t.Run("parsePasskeyContinuationNotification invalid nonce length", func(t *testing.T) {
		identBytes, _ := proto.Marshal(&waCompanionReg.PrimaryEphemeralIdentity{
			PublicKey: make([]byte, 32),
			Nonce:     []byte{1, 2, 3},
		})
		node := &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{{
				Tag:     "primary_ephemeral_identity",
				Content: identBytes,
			}},
		}
		_, err := parsePasskeyContinuationNotification(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected nonce length")
	})
}
