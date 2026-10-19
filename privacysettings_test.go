// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestPrivacySettings_TryFetchPrivacySettings(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		settings, err := nilCli.TryFetchPrivacySettings(ctx, false)
		require.ErrorIs(t, err, ErrClientIsNil)
		assert.Nil(t, settings)
	})

	t.Run("Cached settings", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cached := &types.PrivacySettings{
			GroupAdd:     types.PrivacySettingContactBlacklist,
			LastSeen:     types.PrivacySettingNone,
			Status:       types.PrivacySettingContacts,
			Profile:      types.PrivacySettingAll,
			ReadReceipts: types.PrivacySettingNone,
			Online:       types.PrivacySettingMatchLastSeen,
			CallAdd:      types.PrivacySettingKnown,
		}
		cli.privacySettingsCache.Store(cached)

		res, err := cli.TryFetchPrivacySettings(ctx, false)
		require.NoError(t, err)
		assert.Equal(t, cached, res)

		// With ignoreCache=true, will fetch from server
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("network failure")
		}
		_, err = cli.TryFetchPrivacySettings(ctx, true)
		require.Error(t, err)
	})

	t.Run("Fetch from server success", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "privacy", query.Namespace)
			assert.Equal(t, iqGet, query.Type)
			assert.Equal(t, types.ServerJID, query.To)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "privacy",
					Content: []waBinary.Node{
						{Tag: "category", Attrs: waBinary.Attrs{"name": "groupadd", "value": "all"}},
						{Tag: "category", Attrs: waBinary.Attrs{"name": "last", "value": "contacts"}},
						{Tag: "category", Attrs: waBinary.Attrs{"name": "status", "value": "contact_blacklist"}},
						{Tag: "category", Attrs: waBinary.Attrs{"name": "profile", "value": "none"}},
						{Tag: "category", Attrs: waBinary.Attrs{"name": "readreceipts", "value": "all"}},
						{Tag: "category", Attrs: waBinary.Attrs{"name": "online", "value": "all"}},
						{Tag: "category", Attrs: waBinary.Attrs{"name": "calladd", "value": "known"}},
						{Tag: "category", Attrs: waBinary.Attrs{"name": "messages", "value": "all"}},
						{Tag: "category", Attrs: waBinary.Attrs{"name": "defense", "value": "on"}},
						{Tag: "category", Attrs: waBinary.Attrs{"name": "stickers", "value": "all"}},
						{Tag: "other", Attrs: waBinary.Attrs{"name": "unknown"}},
					},
				}},
			}, nil
		}

		settings, err := cli.TryFetchPrivacySettings(ctx, true)
		require.NoError(t, err)
		assert.Equal(t, types.PrivacySettingAll, settings.GroupAdd)
		assert.Equal(t, types.PrivacySettingContacts, settings.LastSeen)
		assert.Equal(t, types.PrivacySettingContactBlacklist, settings.Status)
		assert.Equal(t, types.PrivacySettingNone, settings.Profile)
		assert.Equal(t, types.PrivacySettingAll, settings.ReadReceipts)
		assert.Equal(t, types.PrivacySettingAll, settings.Online)
		assert.Equal(t, types.PrivacySettingKnown, settings.CallAdd)
		assert.Equal(t, types.PrivacySettingAll, settings.Messages)
		assert.Equal(t, types.PrivacySetting("on"), settings.Defense)
		assert.Equal(t, types.PrivacySettingAll, settings.Stickers)
	})

	t.Run("Fetch from server missing privacy tag", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		settings, err := cli.TryFetchPrivacySettings(ctx, true)
		require.Error(t, err)
		assert.Nil(t, settings)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
	})
}

func TestPrivacySettings_GetPrivacySettings(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		settings := nilCli.GetPrivacySettings(ctx)
		assert.Equal(t, types.PrivacySettings{}, settings)
	})

	t.Run("MessengerConfig present", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.MessengerConfig = &MessengerConfig{}
		settings := cli.GetPrivacySettings(ctx)
		assert.Equal(t, types.PrivacySettings{}, settings)
	})

	t.Run("Fetch error logs and returns empty", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("server error")
		}
		settings := cli.GetPrivacySettings(ctx)
		assert.Equal(t, types.PrivacySettings{}, settings)
	})

	t.Run("Fetch success returns settings", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "privacy",
					Content: []waBinary.Node{
						{Tag: "category", Attrs: waBinary.Attrs{"name": "groupadd", "value": "contacts"}},
					},
				}},
			}, nil
		}
		settings := cli.GetPrivacySettings(ctx)
		assert.Equal(t, types.PrivacySettingContacts, settings.GroupAdd)
	})
}

func TestPrivacySettings_SetPrivacySetting(t *testing.T) {
	ctx := context.Background()

	testCategories := []struct {
		name     types.PrivacySettingType
		val      types.PrivacySetting
		verifyFn func(s types.PrivacySettings) bool
	}{
		{types.PrivacySettingTypeGroupAdd, types.PrivacySettingNone, func(s types.PrivacySettings) bool { return s.GroupAdd == types.PrivacySettingNone }},
		{types.PrivacySettingTypeLastSeen, types.PrivacySettingNone, func(s types.PrivacySettings) bool { return s.LastSeen == types.PrivacySettingNone }},
		{types.PrivacySettingTypeStatus, types.PrivacySettingContactBlacklist, func(s types.PrivacySettings) bool { return s.Status == types.PrivacySettingContactBlacklist }},
		{types.PrivacySettingTypeProfile, types.PrivacySettingNone, func(s types.PrivacySettings) bool { return s.Profile == types.PrivacySettingNone }},
		{types.PrivacySettingTypeReadReceipts, types.PrivacySettingNone, func(s types.PrivacySettings) bool { return s.ReadReceipts == types.PrivacySettingNone }},
		{types.PrivacySettingTypeOnline, types.PrivacySettingMatchLastSeen, func(s types.PrivacySettings) bool { return s.Online == types.PrivacySettingMatchLastSeen }},
		{types.PrivacySettingTypeCallAdd, types.PrivacySettingKnown, func(s types.PrivacySettings) bool { return s.CallAdd == types.PrivacySettingKnown }},
	}

	for _, tc := range testCategories {
		t.Run(string(tc.name), func(t *testing.T) {
			cli := NewClient(&store.Device{}, waLog.Noop)
			// Seed cache
			cli.privacySettingsCache.Store(&types.PrivacySettings{})

			cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
				assert.Equal(t, "privacy", query.Namespace)
				assert.Equal(t, iqSet, query.Type)
				return &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"type": "result"}}, nil
			}

			updated, err := cli.SetPrivacySetting(ctx, tc.name, tc.val)
			require.NoError(t, err)
			assert.True(t, tc.verifyFn(updated))
		})
	}

	t.Run("Fetch initial error", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("initial fetch fail")
		}
		_, err := cli.SetPrivacySetting(ctx, types.PrivacySettingTypeOnline, types.PrivacySettingAll)
		require.Error(t, err)
	})

	t.Run("Set IQ error", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.privacySettingsCache.Store(&types.PrivacySettings{})
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("set iq fail")
		}
		_, err := cli.SetPrivacySetting(ctx, types.PrivacySettingTypeOnline, types.PrivacySettingAll)
		require.Error(t, err)
	})
}

func TestPrivacySettings_SetDefaultDisappearingTimer(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("Success", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "disappearing_mode", query.Namespace)
			assert.Equal(t, iqSet, query.Type)
			nodes, ok := query.Content.([]waBinary.Node)
			require.True(t, ok)
			require.Len(t, nodes, 1)
			assert.Equal(t, "86400", nodes[0].Attrs["duration"])
			return &waBinary.Node{Tag: "iq"}, nil
		}

		err := cli.SetDefaultDisappearingTimer(ctx, 24*time.Hour)
		require.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("timeout")
		}
		err := cli.SetDefaultDisappearingTimer(ctx, 24*time.Hour)
		require.Error(t, err)
	})
}

func TestPrivacySettings_Notification(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("Handle notification error in TryFetch", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("fetch fail")
		}
		cli.handlePrivacySettingsNotification(ctx, &waBinary.Node{Tag: "privacy"})
	})

	t.Run("Handle notification success and event dispatch", func(t *testing.T) {
		cli.privacySettingsCache.Store(&types.PrivacySettings{})
		var receivedEvt *events.PrivacySettings
		cli.AddEventHandler(func(evt any) {
			if ps, ok := evt.(*events.PrivacySettings); ok {
				receivedEvt = ps
			}
		})

		privacyNode := &waBinary.Node{
			Tag: "privacy",
			Content: []waBinary.Node{
				{Tag: "category", Attrs: waBinary.Attrs{"name": "calladd", "value": "known"}},
				{Tag: "category", Attrs: waBinary.Attrs{"name": "messages", "value": "contacts"}},
				{Tag: "category", Attrs: waBinary.Attrs{"name": "defense", "value": "off"}},
				{Tag: "category", Attrs: waBinary.Attrs{"name": "stickers", "value": "contacts"}},
			},
		}

		cli.handlePrivacySettingsNotification(ctx, privacyNode)
		require.NotNil(t, receivedEvt)
		assert.True(t, receivedEvt.CallAddChanged)
		assert.True(t, receivedEvt.MessagesChanged)
		assert.True(t, receivedEvt.DefenseChanged)
		assert.True(t, receivedEvt.StickersChanged)
	})
}
