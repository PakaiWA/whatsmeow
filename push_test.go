// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestPush_Configs(t *testing.T) {
	t.Run("FCMPushConfig", func(t *testing.T) {
		fcm := &FCMPushConfig{Token: "fcm-test-token"}
		attrs := fcm.GetPushConfigAttrs()
		assert.Equal(t, "fcm-test-token", attrs["id"])
		assert.Equal(t, 1, attrs["num_acc"])
		assert.Equal(t, "gcm", attrs["platform"])
	})

	t.Run("APNsPushConfig with custom key and voip token", func(t *testing.T) {
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i)
		}
		apns := &APNsPushConfig{
			Token:       "apns-token",
			VoIPToken:   "voip-token",
			MsgIDEncKey: key,
		}
		attrs := apns.GetPushConfigAttrs()
		assert.Equal(t, "apns-token", attrs["id"])
		assert.Equal(t, "voip-token", attrs["voip"])
		assert.Equal(t, "apple", attrs["platform"])
		assert.Equal(t, 2, attrs["version"])
		assert.Equal(t, 1, attrs["reg_push"])
		assert.Equal(t, 1, attrs["preview"])
		assert.Equal(t, base64.RawURLEncoding.EncodeToString(key), attrs["pkey"])
	})

	t.Run("APNsPushConfig generates key when not 32 bytes and no voip token", func(t *testing.T) {
		apns := &APNsPushConfig{
			Token:       "apns-token-2",
			MsgIDEncKey: []byte{1, 2, 3},
		}
		attrs := apns.GetPushConfigAttrs()
		assert.Len(t, apns.MsgIDEncKey, 32)
		assert.Equal(t, "apns-token-2", attrs["id"])
		assert.Nil(t, attrs["voip"])
		assert.NotEmpty(t, attrs["pkey"])
	})

	t.Run("WebPushConfig", func(t *testing.T) {
		auth := []byte("auth-secret-16bytes!")
		p256dh := []byte("p256dh-public-key-32bytes-123456")
		wpc := &WebPushConfig{
			Endpoint: "https://push.example.com",
			Auth:     auth,
			P256DH:   p256dh,
		}
		attrs := wpc.GetPushConfigAttrs()
		assert.Equal(t, "web", attrs["platform"])
		assert.Equal(t, "https://push.example.com", attrs["endpoint"])
		assert.Equal(t, base64.StdEncoding.EncodeToString(auth), attrs["auth"])
		assert.Equal(t, base64.StdEncoding.EncodeToString(p256dh), attrs["p256dh"])
	})
}

func TestPush_ClientMethods(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("GetServerPushNotificationConfig success", func(t *testing.T) {
		expectedResp := &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{{
				Tag: "settings",
			}},
		}
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "urn:xmpp:whatsapp:push", query.Namespace)
			assert.Equal(t, iqGet, query.Type)
			assert.Equal(t, types.ServerJID, query.To)
			nodes, ok := query.Content.([]waBinary.Node)
			require.True(t, ok)
			require.Len(t, nodes, 1)
			assert.Equal(t, "settings", nodes[0].Tag)
			return expectedResp, nil
		}

		resp, err := cli.GetServerPushNotificationConfig(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedResp, resp)
	})

	t.Run("GetServerPushNotificationConfig error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("network error")
		}

		resp, err := cli.GetServerPushNotificationConfig(ctx)
		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("RegisterForPushNotifications success", func(t *testing.T) {
		fcm := &FCMPushConfig{Token: "token-123"}
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "urn:xmpp:whatsapp:push", query.Namespace)
			assert.Equal(t, iqSet, query.Type)
			assert.Equal(t, types.ServerJID, query.To)
			nodes, ok := query.Content.([]waBinary.Node)
			require.True(t, ok)
			require.Len(t, nodes, 1)
			assert.Equal(t, "config", nodes[0].Tag)
			assert.Equal(t, "token-123", nodes[0].Attrs["id"])
			return &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"type": "result"}}, nil
		}

		err := cli.RegisterForPushNotifications(ctx, fcm)
		require.NoError(t, err)
	})

	t.Run("RegisterForPushNotifications error", func(t *testing.T) {
		fcm := &FCMPushConfig{Token: "token-err"}
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("registration failed")
		}

		err := cli.RegisterForPushNotifications(ctx, fcm)
		require.Error(t, err)
	})
}
