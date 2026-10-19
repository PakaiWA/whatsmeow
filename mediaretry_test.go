// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waMmsRetry"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	"github.com/PakaiWA/whatsmeow/util/gcmutil"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestMediaRetry_GetMediaRetryKey(t *testing.T) {
	mediaKey := []byte("01234567890123456789012345678901")
	key1 := getMediaRetryKey(mediaKey)
	key2 := getMediaRetryKey(mediaKey)
	assert.Len(t, key1, 32)
	assert.Equal(t, key1, key2)
}

func TestMediaRetry_EncryptAndDecryptReceipt(t *testing.T) {
	msgID := types.MessageID("3EB01234567890ABCDEF")
	mediaKey := []byte("01234567890123456789012345678901")

	ciphertext, iv, err := encryptMediaRetryReceipt(msgID, mediaKey)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	assert.Len(t, iv, 12)

	// Decrypt notification simulating successful re-upload
	retryNotif := &waMmsRetry.MediaRetryNotification{
		StanzaID:   new(string),
		DirectPath: new(string),
		Result:     waMmsRetry.MediaRetryNotification_SUCCESS.Enum(),
	}
	*retryNotif.StanzaID = string(msgID)
	*retryNotif.DirectPath = "/v/t62.7118-24/test.enc"

	plaintext, err := proto.Marshal(retryNotif)
	require.NoError(t, err)

	notifCiphertext, err := gcmutil.Encrypt(getMediaRetryKey(mediaKey), iv, plaintext, []byte(msgID))
	require.NoError(t, err)

	evt := &events.MediaRetry{
		MessageID:  msgID,
		Ciphertext: notifCiphertext,
		IV:         iv,
	}

	decrypted, err := DecryptMediaRetryNotification(evt, mediaKey)
	require.NoError(t, err)
	assert.Equal(t, "/v/t62.7118-24/test.enc", decrypted.GetDirectPath())
	assert.Equal(t, waMmsRetry.MediaRetryNotification_SUCCESS, decrypted.GetResult())
}

func TestMediaRetry_DecryptMediaRetryNotification_Errors(t *testing.T) {
	mediaKey := []byte("01234567890123456789012345678901")

	t.Run("Error Code 2 (ErrMediaNotAvailableOnPhone)", func(t *testing.T) {
		evt := &events.MediaRetry{
			Error: &events.MediaRetryError{Code: 2},
		}
		_, err := DecryptMediaRetryNotification(evt, mediaKey)
		require.ErrorIs(t, err, ErrMediaNotAvailableOnPhone)
	})

	t.Run("Other Error Code (ErrUnknownMediaRetryError)", func(t *testing.T) {
		evt := &events.MediaRetry{
			Error: &events.MediaRetryError{Code: 5},
		}
		_, err := DecryptMediaRetryNotification(evt, mediaKey)
		require.ErrorIs(t, err, ErrUnknownMediaRetryError)
	})

	t.Run("Decrypt authentication error", func(t *testing.T) {
		evt := &events.MediaRetry{
			MessageID:  "msg-1",
			Ciphertext: []byte("invalid-ciphertext"),
			IV:         make([]byte, 12),
		}
		_, err := DecryptMediaRetryNotification(evt, mediaKey)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decrypt notification")
	})

	t.Run("Unmarshal corrupted plaintext", func(t *testing.T) {
		iv := make([]byte, 12)
		corruptedPlaintext := []byte{0xff, 0xff, 0xff}
		ciphertext, err := gcmutil.Encrypt(getMediaRetryKey(mediaKey), iv, corruptedPlaintext, []byte("msg-corrupted"))
		require.NoError(t, err)

		evt := &events.MediaRetry{
			MessageID:  "msg-corrupted",
			Ciphertext: ciphertext,
			IV:         iv,
		}
		_, err = DecryptMediaRetryNotification(evt, mediaKey)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal notification")
	})
}

func TestMediaRetry_SendMediaRetryReceipt(t *testing.T) {
	ctx := context.Background()
	mediaKey := []byte("01234567890123456789012345678901")

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		err := nilCli.SendMediaRetryReceipt(ctx, &types.MessageInfo{ID: "123"}, mediaKey)
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Not logged in (empty own ID)", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		err := cli.SendMediaRetryReceipt(ctx, &types.MessageInfo{ID: "123"}, mediaKey)
		require.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("SendMediaRetryReceipt success group and 1-on-1", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		chatJID := types.NewJID("group-123", types.GroupServer)
		senderJID := types.NewJID("user-sender", types.DefaultUserServer)

		groupMsg := &types.MessageInfo{
			ID: "msg-group-1",
			MessageSource: types.MessageSource{
				Chat:     chatJID,
				Sender:   senderJID,
				IsGroup:  true,
				IsFromMe: false,
			},
		}

		err := cli.SendMediaRetryReceipt(ctx, groupMsg, mediaKey)
		require.NoError(t, err)

		directMsg := &types.MessageInfo{
			ID: "msg-direct-1",
			MessageSource: types.MessageSource{
				Chat:     senderJID,
				Sender:   senderJID,
				IsGroup:  false,
				IsFromMe: true,
			},
		}
		err = cli.SendMediaRetryReceipt(ctx, directMsg, mediaKey)
		require.NoError(t, err)
	})
}

func TestMediaRetry_ParseAndHandleNotification(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("Missing attributes in root node", func(t *testing.T) {
		node := &waBinary.Node{Tag: "notification"}
		_, err := parseMediaRetryNotification(node)
		require.Error(t, err)
	})

	t.Run("Missing rmr child tag", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"t":  "1700000000",
				"id": "msg-123",
			},
		}
		_, err := parseMediaRetryNotification(node)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "rmr", elemErr.Tag)
	})

	t.Run("Missing attributes in rmr tag", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"t":  "1700000000",
				"id": "msg-123",
			},
			Content: []waBinary.Node{{
				Tag: "rmr",
				// Missing jid and from_me
			}},
		}
		_, err := parseMediaRetryNotification(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing attributes in <rmr> tag")
	})

	t.Run("Valid notification with error child", func(t *testing.T) {
		chat := types.NewJID("chat-1", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"t":  "1700000000",
				"id": "msg-err-1",
			},
			Content: []waBinary.Node{
				{
					Tag: "rmr",
					Attrs: waBinary.Attrs{
						"jid":     chat,
						"from_me": "false",
					},
				},
				{
					Tag:   "error",
					Attrs: waBinary.Attrs{"code": "2"},
				},
			},
		}

		var dispatched *events.MediaRetry
		cli.AddEventHandler(func(evt any) {
			if mr, ok := evt.(*events.MediaRetry); ok {
				dispatched = mr
			}
		})

		cli.handleMediaRetryNotification(ctx, node)
		require.NotNil(t, dispatched)
		assert.Equal(t, types.MessageID("msg-err-1"), dispatched.MessageID)
		require.NotNil(t, dispatched.Error)
		assert.Equal(t, 2, dispatched.Error.Code)
		assert.Equal(t, chat, dispatched.ChatID)
	})

	t.Run("Missing enc_p in encrypt child", func(t *testing.T) {
		chat := types.NewJID("chat-1", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"t":  "1700000000",
				"id": "msg-no-enc-p",
			},
			Content: []waBinary.Node{
				{
					Tag: "rmr",
					Attrs: waBinary.Attrs{
						"jid":     chat,
						"from_me": "false",
					},
				},
				{
					Tag: "encrypt",
					Content: []waBinary.Node{
						{Tag: "enc_iv", Content: make([]byte, 12)},
					},
				},
			},
		}

		_, err := parseMediaRetryNotification(node)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "enc_p", elemErr.Tag)
	})

	t.Run("Missing enc_iv in encrypt child", func(t *testing.T) {
		chat := types.NewJID("chat-1", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"t":  "1700000000",
				"id": "msg-no-enc-iv",
			},
			Content: []waBinary.Node{
				{
					Tag: "rmr",
					Attrs: waBinary.Attrs{
						"jid":     chat,
						"from_me": "false",
					},
				},
				{
					Tag: "encrypt",
					Content: []waBinary.Node{
						{Tag: "enc_p", Content: []byte("cipher")},
					},
				},
			},
		}

		_, err := parseMediaRetryNotification(node)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "enc_iv", elemErr.Tag)
	})

	t.Run("Valid encrypted retry notification handled and dispatched", func(t *testing.T) {
		chat := types.NewJID("chat-1", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"t":  "1700000000",
				"id": "msg-ok",
			},
			Content: []waBinary.Node{
				{
					Tag: "rmr",
					Attrs: waBinary.Attrs{
						"jid":     chat,
						"from_me": "true",
					},
				},
				{
					Tag: "encrypt",
					Content: []waBinary.Node{
						{Tag: "enc_p", Content: []byte("cipher-data")},
						{Tag: "enc_iv", Content: make([]byte, 12)},
					},
				},
			},
		}

		var dispatched *events.MediaRetry
		cli.AddEventHandler(func(evt any) {
			if mr, ok := evt.(*events.MediaRetry); ok {
				dispatched = mr
			}
		})

		cli.handleMediaRetryNotification(ctx, node)
		require.NotNil(t, dispatched)
		assert.Equal(t, types.MessageID("msg-ok"), dispatched.MessageID)
		assert.Equal(t, []byte("cipher-data"), dispatched.Ciphertext)
		assert.True(t, dispatched.FromMe)
	})
}
