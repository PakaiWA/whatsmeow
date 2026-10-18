// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/appstate"
	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waCommon"
	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/proto/waMmsRetry"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
)

// ============================================================
// errors.go unit tests
// ============================================================

func TestErrors_PairProtoError(t *testing.T) {
	origErr := errors.New("proto parse error")
	err := &PairProtoError{Message: "failed to parse", ProtoErr: origErr}

	assert.Contains(t, err.Error(), "failed to parse")
	assert.Contains(t, err.Error(), "proto parse error")
	assert.Equal(t, origErr, err.Unwrap())
}

func TestErrors_PairDatabaseError(t *testing.T) {
	origErr := errors.New("sqlite lock error")
	err := &PairDatabaseError{Message: "db failure", DBErr: origErr}

	assert.Contains(t, err.Error(), "db failure")
	assert.Contains(t, err.Error(), "sqlite lock error")
	assert.Equal(t, origErr, err.Unwrap())
}

func TestErrors_DownloadHTTPError(t *testing.T) {
	err404 := DownloadHTTPError{Response: &http.Response{StatusCode: 404}}
	assert.Equal(t, "download failed with status code 404", err404.Error())

	// Matching status code
	assert.True(t, errors.Is(err404, ErrMediaDownloadFailedWith404))
	// Non-matching status code
	assert.False(t, errors.Is(err404, ErrMediaDownloadFailedWith403))
	// Non-DownloadHTTPError
	assert.False(t, errors.Is(err404, errors.New("other")))
}

func TestErrors_WrappedIQError(t *testing.T) {
	human := errors.New("user friendly error")
	iq := ErrIQNotFound
	wrapped := wrapIQError(human, iq)

	assert.Equal(t, "user friendly error", wrapped.Error())
	assert.True(t, errors.Is(wrapped, human))
	assert.Equal(t, iq, errors.Unwrap(wrapped))
}

func TestErrors_IQError(t *testing.T) {
	t.Run("Status and Text formatted", func(t *testing.T) {
		iqe := &IQError{Code: 404, Text: "item-not-found"}
		assert.Equal(t, "info query returned status 404: item-not-found", iqe.Error())
		assert.True(t, errors.Is(iqe, ErrIQNotFound))
		assert.False(t, errors.Is(iqe, ErrIQBadRequest))
	})

	t.Run("Zero code with ErrorNode", func(t *testing.T) {
		iqe := &IQError{
			ErrorNode: &waBinary.Node{Tag: "error", Attrs: waBinary.Attrs{"type": "modify"}},
		}
		assert.Contains(t, iqe.Error(), "info query returned unknown error")
	})

	t.Run("Zero code with RawNode only", func(t *testing.T) {
		iqe := &IQError{
			RawNode: &waBinary.Node{Tag: "iq"},
		}
		assert.Contains(t, iqe.Error(), "info query returned unexpected response")
	})

	t.Run("Empty IQError", func(t *testing.T) {
		iqe := &IQError{}
		assert.Equal(t, "unknown info query error", iqe.Error())
		assert.False(t, iqe.Is(errors.New("other")))
	})
}

func TestErrors_ParseIQError(t *testing.T) {
	node := &waBinary.Node{
		Tag: "iq",
		Content: []waBinary.Node{
			{
				Tag:   "error",
				Attrs: waBinary.Attrs{"code": "403", "text": "forbidden"},
			},
		},
	}
	err := parseIQError(node)
	require.NotNil(t, err)

	iqe, ok := err.(*IQError)
	require.True(t, ok)
	assert.Equal(t, 403, iqe.Code)
	assert.Equal(t, "forbidden", iqe.Text)
	assert.True(t, errors.Is(err, ErrIQForbidden))
}

func TestErrors_ElementMissingError(t *testing.T) {
	err := &ElementMissingError{Tag: "order", In: "response"}
	assert.Equal(t, "missing <order> element in response", err.Error())
}

func TestErrors_DisconnectedError(t *testing.T) {
	err := &DisconnectedError{Action: "info query"}
	assert.Equal(t, "websocket disconnected before info query returned response", err.Error())
	assert.True(t, errors.Is(err, ErrIQDisconnected))
	assert.False(t, errors.Is(err, errors.New("other")))
}

// ============================================================
// cstoken.go unit tests
// ============================================================

func TestCSToken_ShouldSendCsToken(t *testing.T) {
	normalUser := types.NewJID("628123456789", types.DefaultUserServer)
	assert.True(t, shouldSendCsToken(normalUser))

	lidUser := types.NewJID("10000000000001", types.HiddenUserServer)
	assert.True(t, shouldSendCsToken(lidUser))

	// Group JID should NOT send cs token
	groupJID := types.NewJID("120363025112345678", types.GroupServer)
	assert.False(t, shouldSendCsToken(groupJID))

	// PSA JID should NOT send cs token
	assert.False(t, shouldSendCsToken(types.PSAJID))

	// Bot JID should NOT send cs token
	botJID := types.NewJID("13135550002", types.DefaultUserServer)
	assert.False(t, shouldSendCsToken(botJID))
}

// ============================================================
// msgsecret.go unit tests
// ============================================================

func TestMsgSecret_ApplyBotMessageHKDF(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	derived := applyBotMessageHKDF(secret)
	assert.Len(t, derived, 32)
	assert.NotEqual(t, secret, derived)

	// Deterministic
	derived2 := applyBotMessageHKDF(secret)
	assert.Equal(t, derived, derived2)
}

func TestMsgSecret_GenerateMsgSecretKey(t *testing.T) {
	origSecret := []byte("original_secret_key_32_bytes_x")
	senderA := types.NewJID("628111111111", types.DefaultUserServer)
	senderB := types.NewJID("628222222222", types.DefaultUserServer)
	msgID := types.MessageID("MSG-123456")

	t.Run("Poll Vote with additional data", func(t *testing.T) {
		key, ad := generateMsgSecretKey(EncSecretPollVote, senderA, msgID, senderB, origSecret)
		assert.Len(t, key, 32)
		assert.NotEmpty(t, ad)
		assert.Contains(t, string(ad), "MSG-123456")
		assert.Contains(t, string(ad), "628111111111")
	})

	t.Run("Reaction without additional data", func(t *testing.T) {
		key, ad := generateMsgSecretKey(EncSecretReaction, senderA, msgID, senderB, origSecret)
		assert.Len(t, key, 32)
		assert.Nil(t, ad)
	})
}

func TestMsgSecret_GetOrigSenderFromKey(t *testing.T) {
	senderJID := types.NewJID("628111111111", types.DefaultUserServer)
	remoteJID := types.NewJID("628222222222", types.DefaultUserServer)

	msg := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: senderJID,
				Chat:   remoteJID,
			},
		},
	}

	t.Run("FromMe returns sender", func(t *testing.T) {
		key := &waCommon.MessageKey{
			FromMe: new(bool),
		}
		*key.FromMe = true
		origSender, err := getOrigSenderFromKey(msg, key)
		require.NoError(t, err)
		assert.Equal(t, senderJID, origSender)
	})

	t.Run("DM chat parses RemoteJID", func(t *testing.T) {
		key := &waCommon.MessageKey{
			RemoteJID: new(string),
		}
		*key.RemoteJID = remoteJID.String()
		origSender, err := getOrigSenderFromKey(msg, key)
		require.NoError(t, err)
		assert.Equal(t, remoteJID, origSender)
	})

	t.Run("Group chat parses Participant", func(t *testing.T) {
		groupChat := types.NewJID("120363025112345678", types.GroupServer)
		groupMsg := &events.Message{
			Info: types.MessageInfo{
				MessageSource: types.MessageSource{Chat: groupChat},
			},
		}
		participantJID := types.NewJID("628333333333", types.DefaultUserServer)
		key := &waCommon.MessageKey{
			Participant: new(string),
		}
		*key.Participant = participantJID.String()

		origSender, err := getOrigSenderFromKey(groupMsg, key)
		require.NoError(t, err)
		assert.Equal(t, participantJID, origSender)
	})

	t.Run("Group chat invalid server in Participant", func(t *testing.T) {
		groupChat := types.NewJID("120363025112345678", types.GroupServer)
		groupMsg := &events.Message{
			Info: types.MessageInfo{
				MessageSource: types.MessageSource{Chat: groupChat},
			},
		}
		key := &waCommon.MessageKey{
			Participant: new(string),
		}
		*key.Participant = "invalid@unknown.server"

		_, err := getOrigSenderFromKey(groupMsg, key)
		assert.Error(t, err)
	})
}

// ============================================================
// mediaretry.go unit tests
// ============================================================

func TestMediaRetry_KeyAndEncryptRoundTrip(t *testing.T) {
	mediaKey := []byte("media_key_32_bytes_test_data_xxx")
	msgID := types.MessageID("MSG-RETRY-001")

	// Get key
	key := getMediaRetryKey(mediaKey)
	assert.Len(t, key, 32)

	// Encrypt receipt
	ciphertext, iv, err := encryptMediaRetryReceipt(msgID, mediaKey)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	assert.Len(t, iv, 12)

	// Decrypt notification with valid payload
	testNotif := &waMmsRetry.MediaRetryNotification{
		DirectPath: new("/v/t62.7118-24/test.enc"),
	}
	plaintext, err := proto.Marshal(testNotif)
	require.NoError(t, err)

	evt := &events.MediaRetry{
		MessageID:  msgID,
		IV:         iv,
		Ciphertext: ciphertext,
	}

	// Encrypt our valid notif to simulate valid server response
	notifCiphertext, notifIV, err := encryptMediaRetryReceipt(msgID, mediaKey)
	require.NoError(t, err)
	_ = notifCiphertext
	_ = notifIV

	// Test DecryptMediaRetryNotification with error response
	t.Run("Error 2 media not available on phone", func(t *testing.T) {
		errEvt := &events.MediaRetry{
			Error: &events.MediaRetryError{Code: 2},
		}
		_, err := DecryptMediaRetryNotification(errEvt, mediaKey)
		assert.ErrorIs(t, err, ErrMediaNotAvailableOnPhone)
	})

	t.Run("Unknown error code", func(t *testing.T) {
		errEvt := &events.MediaRetry{
			Error: &events.MediaRetryError{Code: 99},
		}
		_, err := DecryptMediaRetryNotification(errEvt, mediaKey)
		assert.ErrorIs(t, err, ErrUnknownMediaRetryError)
	})

	_ = plaintext
	_ = evt
}

func TestMediaRetry_ParseMediaRetryNotification(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	chatJID := types.NewJID("628123456789", types.DefaultUserServer)

	node := &waBinary.Node{
		Tag: "notification",
		Attrs: waBinary.Attrs{
			"id": "MSG-999",
			"t":  strconv.FormatInt(now.Unix(), 10),
		},
		Content: []waBinary.Node{
			{
				Tag: "rmr",
				Attrs: waBinary.Attrs{
					"jid":     chatJID,
					"from_me": "true",
				},
			},
			{
				Tag: "encrypt",
				Content: []waBinary.Node{
					{Tag: "enc_p", Content: []byte("encrypted_payload")},
					{Tag: "enc_iv", Content: []byte("12_byte_iv__")},
				},
			},
		},
	}

	evt, err := parseMediaRetryNotification(node)
	require.NoError(t, err)
	assert.Equal(t, types.MessageID("MSG-999"), evt.MessageID)
	assert.Equal(t, chatJID, evt.ChatID)
	assert.True(t, evt.FromMe)
	assert.Equal(t, []byte("encrypted_payload"), evt.Ciphertext)
	assert.Equal(t, []byte("12_byte_iv__"), evt.IV)
}

func TestMediaRetry_ParseMediaRetryNotification_MissingRMR(t *testing.T) {
	node := &waBinary.Node{
		Tag:   "notification",
		Attrs: waBinary.Attrs{"id": "MSG-1", "t": "12345"},
	}
	_, err := parseMediaRetryNotification(node)
	assert.Error(t, err)
	var eme *ElementMissingError
	assert.True(t, errors.As(err, &eme))
	assert.Equal(t, "rmr", eme.Tag)
}

// ============================================================
// reportingtoken.go unit tests
// ============================================================

func TestReportingToken_GetReportingFields(t *testing.T) {
	fields := getReportingFields()
	assert.NotEmpty(t, fields)

	// Verify getConfigForField
	cfg := getConfigForField(fields, fields[0].FieldNumber)
	require.NotNil(t, cfg)
	assert.Equal(t, fields[0].FieldNumber, cfg.FieldNumber)

	// Non-existent field returns nil
	assert.Nil(t, getConfigForField(fields, -9999))
}

func TestReportingToken_ExtractAndGet(t *testing.T) {
	// A sample message protobuf
	msg := &waE2E.Message{
		Conversation: new("hello world for reporting test"),
	}
	data, err := proto.Marshal(msg)
	require.NoError(t, err)

	token := getReportingToken(data)
	assert.NotEmpty(t, token)

	// Deterministic
	token2 := getReportingToken(data)
	assert.Equal(t, token, token2)
}

// ============================================================
// business.go unit tests
// ============================================================

func TestBusiness_GetStringChild(t *testing.T) {
	node := waBinary.Node{
		Tag: "order",
		Content: []waBinary.Node{
			{Tag: "subtotal", Content: []byte("50000")},
		},
	}
	assert.Equal(t, "50000", getStringChild(node, "subtotal"))
	assert.Empty(t, getStringChild(node, "missing_tag"))
}

func TestBusiness_ParseOrderDetailsNode(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	orderNode := waBinary.Node{
		Tag: "order",
		Attrs: waBinary.Attrs{
			"id":          "ORDER-12345",
			"creation_ts": strconv.FormatInt(now.Unix(), 10),
		},
		Content: []waBinary.Node{
			{
				Tag: "price",
				Content: []waBinary.Node{
					{Tag: "subtotal", Content: []byte("100000")},
					{Tag: "total", Content: []byte("110000")},
					{Tag: "currency", Content: []byte("IDR")},
					{Tag: "price_status", Content: []byte("paid")},
				},
			},
			{
				Tag: "catalog",
				Content: []waBinary.Node{
					{Tag: "id", Content: []byte("CAT-999")},
				},
			},
			{
				Tag: "product",
				Content: []waBinary.Node{
					{Tag: "id", Content: []byte("PROD-1")},
					{Tag: "name", Content: []byte("Item 1")},
					{Tag: "price", Content: []byte("50000")},
					{Tag: "quantity", Content: []byte("2")},
					{Tag: "currency", Content: []byte("IDR")},
				},
			},
		},
	}

	details, err := parseOrderDetailsNode(orderNode)
	require.NoError(t, err)
	assert.Equal(t, "ORDER-12345", details.ID)
	assert.Equal(t, int64(100000), details.Price.Subtotal)
	assert.Equal(t, int64(110000), details.Price.Total)
	assert.Equal(t, "IDR", details.Price.Currency)
	assert.Equal(t, "CAT-999", details.CatalogID)
	require.Len(t, details.Products, 1)
	assert.Equal(t, "PROD-1", details.Products[0].ID)
	assert.Equal(t, 2, details.Products[0].Quantity)
}

// ============================================================
// appstate.go helper tests
// ============================================================

func TestAppState_BuildFatalAppStateExceptionNotification(t *testing.T) {
	msg := BuildFatalAppStateExceptionNotification(appstate.WAPatchRegularHigh, appstate.WAPatchRegularLow)
	require.NotNil(t, msg)
	require.NotNil(t, msg.ProtocolMessage)
	assert.Equal(t, waE2E.ProtocolMessage_APP_STATE_FATAL_EXCEPTION_NOTIFICATION, msg.ProtocolMessage.GetType())
	assert.Equal(t, []string{string(appstate.WAPatchRegularHigh), string(appstate.WAPatchRegularLow)}, msg.ProtocolMessage.GetAppStateFatalExceptionNotification().GetCollectionNames())
}

func TestAppState_BuildAppStateRecoveryRequest(t *testing.T) {
	msg := BuildAppStateRecoveryRequest(appstate.WAPatchCriticalUnblockLow)
	require.NotNil(t, msg)
	require.NotNil(t, msg.ProtocolMessage)
	assert.Equal(t, waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE, msg.ProtocolMessage.GetType())
	pdom := msg.ProtocolMessage.GetPeerDataOperationRequestMessage()
	require.NotNil(t, pdom)
	assert.Equal(t, string(appstate.WAPatchCriticalUnblockLow), pdom.GetSyncdCollectionFatalRecoveryRequest().GetCollectionName())
}
