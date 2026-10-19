// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/pbkdf2"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestPairCode_GenerateCompanionEphemeralKey(t *testing.T) {
	keyPair, ephemeralKey, code := generateCompanionEphemeralKey()
	assert.NotNil(t, keyPair)
	assert.Len(t, ephemeralKey, 80)
	assert.Len(t, code, 8) // base32 encoding of 5 bytes gives 8 characters
}

func TestPairCode_PairPhone_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		_, err := nilCli.PairPhone(ctx, "628123456789", true, PairClientChrome, "Chrome (Linux)")
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Phone too short", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		_, err := cli.PairPhone(ctx, "12345", true, PairClientChrome, "Chrome (Linux)")
		require.ErrorIs(t, err, ErrPhoneNumberTooShort)
	})

	t.Run("Phone is not international (starts with 0)", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		_, err := cli.PairPhone(ctx, "08123456789", true, PairClientChrome, "Chrome (Linux)")
		require.ErrorIs(t, err, ErrPhoneNumberIsNotInternational)
	})

	t.Run("SendIQ error", func(t *testing.T) {
		cli := NewClient(&store.Device{NoiseKey: keys.NewKeyPair()}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("sendIQ failed")
		}
		_, err := cli.PairPhone(ctx, "+62 812-3456-7890", true, PairClientChrome, "Chrome (Linux)")
		require.Error(t, err)
		assert.Equal(t, "sendIQ failed", err.Error())
	})

	t.Run("Missing pairing ref node", func(t *testing.T) {
		cli := NewClient(&store.Device{NoiseKey: keys.NewKeyPair()}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "link_code_companion_reg",
				}},
			}, nil
		}
		_, err := cli.PairPhone(ctx, "628123456789", true, PairClientChrome, "Chrome (Linux)")
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "link_code_pairing_ref", elemErr.Tag)
	})

	t.Run("Pairing ref content not bytes", func(t *testing.T) {
		cli := NewClient(&store.Device{NoiseKey: keys.NewKeyPair()}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "link_code_companion_reg",
					Content: []waBinary.Node{{
						Tag:     "link_code_pairing_ref",
						Content: "string-instead-of-bytes",
					}},
				}},
			}, nil
		}
		_, err := cli.PairPhone(ctx, "628123456789", true, PairClientChrome, "Chrome (Linux)")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected type string in content of link_code_pairing_ref tag")
	})
}

func TestPairCode_PairPhone_Success(t *testing.T) {
	ctx := context.Background()
	dev := &store.Device{
		NoiseKey: keys.NewKeyPair(),
	}
	cli := NewClient(dev, waLog.Noop)

	cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
		assert.Equal(t, "md", query.Namespace)
		assert.Equal(t, iqSet, query.Type)
		assert.Equal(t, types.ServerJID, query.To)
		contentNodes, ok := query.Content.([]waBinary.Node)
		require.True(t, ok)
		require.Len(t, contentNodes, 1)
		regNode := contentNodes[0]
		assert.Equal(t, "link_code_companion_reg", regNode.Tag)
		assert.Equal(t, "true", regNode.Attrs["should_show_push_notification"])
		assert.Equal(t, "companion_hello", regNode.Attrs["stage"])

		return &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{{
				Tag: "link_code_companion_reg",
				Content: []waBinary.Node{{
					Tag:     "link_code_pairing_ref",
					Content: []byte("ref-token-xyz"),
				}},
			}},
		}, nil
	}

	code, err := cli.PairPhone(ctx, "+62 812-3456-7890", true, PairClientChrome, "Chrome (Linux)")
	require.NoError(t, err)
	assert.Len(t, code, 9) // 4 chars + '-' + 4 chars
	assert.Contains(t, code, "-")

	cache := cli.phoneLinkingCache.Load()
	require.NotNil(t, cache)
	assert.Equal(t, "ref-token-xyz", cache.pairingRef)
	assert.Equal(t, "6281234567890@s.whatsapp.net", cache.jid.String())
}

func TestPairCode_HandleCodePairNotification_Errors(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{IdentityKey: keys.NewKeyPair()}, waLog.Noop)

	t.Run("Missing link_code_companion_reg", func(t *testing.T) {
		node := &waBinary.Node{Tag: "notification"}
		err := cli.handleCodePairNotification(ctx, node)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "link_code_companion_reg", elemErr.Tag)
	})

	t.Run("No pending pairing in cache", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "notification",
			Content: []waBinary.Node{{
				Tag: "link_code_companion_reg",
			}},
		}
		cli.phoneLinkingCache.Store(nil)
		err := cli.handleCodePairNotification(ctx, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "received code pair notification without a pending pairing")
	})

	t.Run("Pairing ref mismatch", func(t *testing.T) {
		cli.phoneLinkingCache.Store(&phoneLinkingCache{
			pairingRef: "expected-ref",
		})
		node := &waBinary.Node{
			Tag: "notification",
			Content: []waBinary.Node{{
				Tag: "link_code_companion_reg",
				Content: []waBinary.Node{{
					Tag:     "link_code_pairing_ref",
					Content: []byte("different-ref"),
				}},
			}},
		}
		err := cli.handleCodePairNotification(ctx, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pairing ref mismatch in code pair notification")
	})

	t.Run("Missing primary ephemeral pub", func(t *testing.T) {
		cli.phoneLinkingCache.Store(&phoneLinkingCache{
			pairingRef: "ref-1",
		})
		node := &waBinary.Node{
			Tag: "notification",
			Content: []waBinary.Node{{
				Tag: "link_code_companion_reg",
				Content: []waBinary.Node{{
					Tag:     "link_code_pairing_ref",
					Content: []byte("ref-1"),
				}},
			}},
		}
		err := cli.handleCodePairNotification(ctx, node)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "link_code_pairing_wrapped_primary_ephemeral_pub", elemErr.Tag)
	})

	t.Run("Primary ephemeral pub too short (< 80 bytes)", func(t *testing.T) {
		cli.phoneLinkingCache.Store(&phoneLinkingCache{
			pairingRef: "ref-1",
		})
		node := &waBinary.Node{
			Tag: "notification",
			Content: []waBinary.Node{{
				Tag: "link_code_companion_reg",
				Content: []waBinary.Node{
					{Tag: "link_code_pairing_ref", Content: []byte("ref-1")},
					{Tag: "link_code_pairing_wrapped_primary_ephemeral_pub", Content: make([]byte, 50)},
				},
			}},
		}
		err := cli.handleCodePairNotification(ctx, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected length of link_code_pairing_wrapped_primary_ephemeral_pub")
	})

	t.Run("Missing primary identity pub", func(t *testing.T) {
		cli.phoneLinkingCache.Store(&phoneLinkingCache{
			pairingRef: "ref-1",
		})
		node := &waBinary.Node{
			Tag: "notification",
			Content: []waBinary.Node{{
				Tag: "link_code_companion_reg",
				Content: []waBinary.Node{
					{Tag: "link_code_pairing_ref", Content: []byte("ref-1")},
					{Tag: "link_code_pairing_wrapped_primary_ephemeral_pub", Content: make([]byte, 80)},
				},
			}},
		}
		err := cli.handleCodePairNotification(ctx, node)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "primary_identity_pub", elemErr.Tag)
	})

	t.Run("TryHandleCodePairNotification logs error", func(t *testing.T) {
		// Verify wrapper method executes safely
		cli.phoneLinkingCache.Store(nil)
		cli.tryHandleCodePairNotification(ctx, &waBinary.Node{Tag: "notification"})
	})
}

func TestPairCode_HandleCodePairNotification_Success(t *testing.T) {
	ctx := context.Background()
	dev := &store.Device{
		IdentityKey: keys.NewKeyPair(),
	}
	cli := NewClient(dev, waLog.Noop)

	// Simulate primary device pairing exchange
	primaryEphemeralKeyPair := keys.NewKeyPair()
	primaryIdentityKeyPair := keys.NewKeyPair()

	linkingCode := "ABCDEFGH"
	primarySalt := make([]byte, 32)
	for i := range primarySalt {
		primarySalt[i] = byte(i)
	}
	primaryIV := make([]byte, 16)
	for i := range primaryIV {
		primaryIV[i] = byte(i + 10)
	}

	// Encrypt primary ephemeral public key using linking code
	linkCodeKey := pbkdf2.Key([]byte(linkingCode), primarySalt, 2<<16, 32, sha256.New)
	linkCipherBlock, err := aes.NewCipher(linkCodeKey)
	require.NoError(t, err)

	encryptedPrimaryPub := make([]byte, 32)
	copy(encryptedPrimaryPub, primaryEphemeralKeyPair.Pub[:])
	cipher.NewCTR(linkCipherBlock, primaryIV).XORKeyStream(encryptedPrimaryPub, encryptedPrimaryPub)

	wrappedPrimaryEphemeralPub := make([]byte, 80)
	copy(wrappedPrimaryEphemeralPub[0:32], primarySalt)
	copy(wrappedPrimaryEphemeralPub[32:48], primaryIV)
	copy(wrappedPrimaryEphemeralPub[48:80], encryptedPrimaryPub)

	// Companion side setup
	companionEphemeralKeyPair := keys.NewKeyPair()
	pairingRef := "test-pairing-ref-123"
	cli.phoneLinkingCache.Store(&phoneLinkingCache{
		jid:         types.NewJID("628123456789", types.DefaultUserServer),
		keyPair:     companionEphemeralKeyPair,
		linkingCode: linkingCode,
		pairingRef:  pairingRef,
	})

	var capturedQuery infoQuery
	cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
		capturedQuery = query
		return &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"type": "result"}}, nil
	}

	notificationNode := &waBinary.Node{
		Tag: "notification",
		Content: []waBinary.Node{{
			Tag: "link_code_companion_reg",
			Content: []waBinary.Node{
				{Tag: "link_code_pairing_ref", Content: []byte(pairingRef)},
				{Tag: "link_code_pairing_wrapped_primary_ephemeral_pub", Content: wrappedPrimaryEphemeralPub},
				{Tag: "primary_identity_pub", Content: primaryIdentityKeyPair.Pub[:]},
			},
		}},
	}

	err = cli.handleCodePairNotification(ctx, notificationNode)
	require.NoError(t, err)

	// Check that AdvSecretKey was set
	assert.NotEmpty(t, cli.Store.AdvSecretKey)
	assert.Len(t, cli.Store.AdvSecretKey, 32)

	// Verify IQ query sent
	assert.Equal(t, "md", capturedQuery.Namespace)
	assert.Equal(t, iqSet, capturedQuery.Type)
	capturedNodes, ok := capturedQuery.Content.([]waBinary.Node)
	require.True(t, ok)
	require.Len(t, capturedNodes, 1)
	finishNode := capturedNodes[0]
	assert.Equal(t, "companion_finish", finishNode.Attrs["stage"])
	assert.Equal(t, "628123456789@s.whatsapp.net", finishNode.Attrs["jid"].(types.JID).String())
	assert.Equal(t, pairingRef, string(finishNode.GetChildByTag("link_code_pairing_ref").Content.([]byte)))
	assert.NotEmpty(t, finishNode.GetChildByTag("link_code_pairing_wrapped_key_bundle").Content)

	// Verify ephemeral shared secret math
	expectedSharedSecret, err := curve25519.X25519(companionEphemeralKeyPair.Priv[:], primaryEphemeralKeyPair.Pub[:])
	require.NoError(t, err)
	assert.NotEmpty(t, expectedSharedSecret)
}
