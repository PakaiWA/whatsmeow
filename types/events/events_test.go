// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package events

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/proto/waArmadilloApplication"
	"github.com/PakaiWA/whatsmeow/proto/waConsumerApplication"
	"github.com/PakaiWA/whatsmeow/proto/waE2E"
)

func TestPermanentDisconnectDescription(t *testing.T) {
	loggedOut := &LoggedOut{Reason: ConnectFailureLoggedOut}
	assert.Equal(t, "401: logged out from another device", loggedOut.PermanentDisconnectDescription())

	streamReplaced := &StreamReplaced{}
	assert.Equal(t, "stream replaced", streamReplaced.PermanentDisconnectDescription())

	clientOutdated := &ClientOutdated{}
	assert.Equal(t, "client outdated", clientOutdated.PermanentDisconnectDescription())

	catRefreshErr := &CATRefreshError{Error: errors.New("cat error")}
	assert.Equal(t, "CAT refresh failed", catRefreshErr.PermanentDisconnectDescription())

	tempBanNoExpire := &TemporaryBan{Code: TempBanSentToTooManyPeople, Expire: 0}
	assert.Equal(t, "temporarily banned: You've been temporarily banned: 101: you sent too many messages to people who don't have you in their address books", tempBanNoExpire.PermanentDisconnectDescription())

	tempBanWithExpire := &TemporaryBan{Code: TempBanBlockedByUsers, Expire: 5 * time.Minute}
	assert.Equal(t, "temporarily banned: You've been temporarily banned: 102: too many people blocked you. The ban expires in 5m0s", tempBanWithExpire.PermanentDisconnectDescription())

	connectFailure := &ConnectFailure{Reason: ConnectFailureBadUserAgent}
	assert.Equal(t, "connect failure: 409: client user agent was rejected", connectFailure.PermanentDisconnectDescription())
}

func TestTempBanReason_String(t *testing.T) {
	tests := []struct {
		code     TempBanReason
		expected string
	}{
		{TempBanSentToTooManyPeople, "101: you sent too many messages to people who don't have you in their address books"},
		{TempBanBlockedByUsers, "102: too many people blocked you"},
		{TempBanCreatedTooManyGroups, "103: you created too many groups with people who don't have you in their address books"},
		{TempBanSentTooManySameMessage, "104: you sent the same message to too many people"},
		{TempBanBroadcastList, "106: you sent too many messages to a broadcast list"},
		{TempBanReason(999), "999: you may have violated the terms of service (unknown error)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.String())
		})
	}
}

func TestConnectFailureReason_Methods(t *testing.T) {
	t.Run("IsLoggedOut", func(t *testing.T) {
		assert.True(t, ConnectFailureLoggedOut.IsLoggedOut())
		assert.True(t, ConnectFailureMainDeviceGone.IsLoggedOut())
		assert.True(t, ConnectFailureUnknownLogout.IsLoggedOut())

		assert.False(t, ConnectFailureGeneric.IsLoggedOut())
		assert.False(t, ConnectFailureTempBanned.IsLoggedOut())
		assert.False(t, ConnectFailureClientOutdated.IsLoggedOut())
		assert.False(t, ConnectFailureInternalServerError.IsLoggedOut())
	})

	t.Run("NumberString", func(t *testing.T) {
		assert.Equal(t, "401", ConnectFailureLoggedOut.NumberString())
		assert.Equal(t, "500", ConnectFailureInternalServerError.NumberString())
	})

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "401: logged out from another device", ConnectFailureLoggedOut.String())
		assert.Equal(t, "402: account temporarily banned", ConnectFailureTempBanned.String())
		assert.Equal(t, "403: primary device was logged out", ConnectFailureMainDeviceGone.String())
		assert.Equal(t, "406: logged out for unknown reason", ConnectFailureUnknownLogout.String())
		assert.Equal(t, "405: client is out of date", ConnectFailureClientOutdated.String())
		assert.Equal(t, "409: client user agent was rejected", ConnectFailureBadUserAgent.String())
		assert.Equal(t, "413: messenger crypto auth token has expired", ConnectFailureCATExpired.String())
		assert.Equal(t, "414: messenger crypto auth token is invalid", ConnectFailureCATInvalid.String())
		assert.Equal(t, "999: unknown error", ConnectFailureReason(999).String())
	})
}

func TestFBMessage_Getters(t *testing.T) {
	consumerApp := &waConsumerApplication.ConsumerApplication{}
	armadilloApp := &waArmadilloApplication.Armadillo{}

	t.Run("GetConsumerApplication matching and nil", func(t *testing.T) {
		fbMsg := &FBMessage{Message: consumerApp}
		assert.Same(t, consumerApp, fbMsg.GetConsumerApplication())

		fbMsgNil := &FBMessage{Message: armadilloApp}
		assert.Nil(t, fbMsgNil.GetConsumerApplication())

		fbMsgEmpty := &FBMessage{Message: nil}
		assert.Nil(t, fbMsgEmpty.GetConsumerApplication())
	})

	t.Run("GetArmadillo matching and nil", func(t *testing.T) {
		fbMsg := &FBMessage{Message: armadilloApp}
		assert.Same(t, armadilloApp, fbMsg.GetArmadillo())

		fbMsgNil := &FBMessage{Message: consumerApp}
		assert.Nil(t, fbMsgNil.GetArmadillo())

		fbMsgEmpty := &FBMessage{Message: nil}
		assert.Nil(t, fbMsgEmpty.GetArmadillo())
	})
}

func TestMessage_UnwrapRaw(t *testing.T) {
	baseTextMsg := &waE2E.Message{
		Conversation: proto.String("hello world"),
	}

	t.Run("Direct conversation message", func(t *testing.T) {
		msg := &Message{RawMessage: baseTextMsg}
		unwrapped := msg.UnwrapRaw()
		require.NotNil(t, unwrapped)
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		assert.False(t, unwrapped.IsEphemeral)
		assert.False(t, unwrapped.IsViewOnce)
		assert.False(t, unwrapped.IsEdit)
	})

	t.Run("DeviceSentMessage", func(t *testing.T) {
		raw := &waE2E.Message{
			DeviceSentMessage: &waE2E.DeviceSentMessage{
				DestinationJID: proto.String("628111@s.whatsapp.net"),
				Phash:          proto.String("phash123"),
				Message:        baseTextMsg,
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		require.NotNil(t, unwrapped.Info.DeviceSentMeta)
		assert.Equal(t, "628111@s.whatsapp.net", unwrapped.Info.DeviceSentMeta.DestinationJID)
		assert.Equal(t, "phash123", unwrapped.Info.DeviceSentMeta.Phash)
	})

	t.Run("BotInvokeMessage", func(t *testing.T) {
		raw := &waE2E.Message{
			BotInvokeMessage: &waE2E.FutureProofMessage{
				Message: baseTextMsg,
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		assert.True(t, unwrapped.IsBotInvoke)
	})

	t.Run("EphemeralMessage", func(t *testing.T) {
		raw := &waE2E.Message{
			EphemeralMessage: &waE2E.FutureProofMessage{
				Message: baseTextMsg,
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		assert.True(t, unwrapped.IsEphemeral)
	})

	t.Run("ViewOnceMessage", func(t *testing.T) {
		raw := &waE2E.Message{
			ViewOnceMessage: &waE2E.FutureProofMessage{
				Message: baseTextMsg,
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		assert.True(t, unwrapped.IsViewOnce)
	})

	t.Run("ViewOnceMessageV2", func(t *testing.T) {
		raw := &waE2E.Message{
			ViewOnceMessageV2: &waE2E.FutureProofMessage{
				Message: baseTextMsg,
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		assert.True(t, unwrapped.IsViewOnce)
		assert.True(t, unwrapped.IsViewOnceV2)
	})

	t.Run("ViewOnceMessageV2Extension", func(t *testing.T) {
		raw := &waE2E.Message{
			ViewOnceMessageV2Extension: &waE2E.FutureProofMessage{
				Message: baseTextMsg,
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		assert.True(t, unwrapped.IsViewOnce)
		assert.True(t, unwrapped.IsViewOnceV2)
		assert.True(t, unwrapped.IsViewOnceV2Extension)
	})

	t.Run("LottieStickerMessage", func(t *testing.T) {
		raw := &waE2E.Message{
			LottieStickerMessage: &waE2E.FutureProofMessage{
				Message: baseTextMsg,
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		assert.True(t, unwrapped.IsLottieSticker)
	})

	t.Run("DocumentWithCaptionMessage", func(t *testing.T) {
		raw := &waE2E.Message{
			DocumentWithCaptionMessage: &waE2E.FutureProofMessage{
				Message: baseTextMsg,
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		assert.True(t, unwrapped.IsDocumentWithCaption)
	})

	t.Run("EditedMessage", func(t *testing.T) {
		raw := &waE2E.Message{
			EditedMessage: &waE2E.FutureProofMessage{
				Message: baseTextMsg,
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "hello world", unwrapped.Message.GetConversation())
		assert.True(t, unwrapped.IsEdit)
	})

	t.Run("Preserves MessageContextInfo from RawMessage", func(t *testing.T) {
		ctxInfo := &waE2E.MessageContextInfo{
			DeviceListMetadataVersion: proto.Int32(2),
		}
		raw := &waE2E.Message{
			MessageContextInfo: ctxInfo,
			EphemeralMessage: &waE2E.FutureProofMessage{
				Message: &waE2E.Message{
					Conversation: proto.String("nested"),
				},
			},
		}
		msg := &Message{RawMessage: raw}
		unwrapped := msg.UnwrapRaw()
		assert.Equal(t, "nested", unwrapped.Message.GetConversation())
		require.NotNil(t, unwrapped.Message.MessageContextInfo)
		assert.Equal(t, int32(2), unwrapped.Message.MessageContextInfo.GetDeviceListMetadataVersion())
	})
}
