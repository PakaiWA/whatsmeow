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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestBroadcast_GetStatusPrivacy(t *testing.T) {
	ctx := context.Background()

	t.Run("IQ error other than ErrIQNotFound", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		expectedErr := errors.New("iq network error")
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, expectedErr
		}

		res, err := cli.GetStatusPrivacy(ctx)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("ErrIQNotFound returns DefaultStatusPrivacy", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotFound
		}

		res, err := cli.GetStatusPrivacy(ctx)
		require.NoError(t, err)
		assert.Equal(t, DefaultStatusPrivacy, res)
	})

	t.Run("Empty privacy list returns DefaultStatusPrivacy", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "status", query.Namespace)
			assert.Equal(t, iqGet, query.Type)
			assert.Equal(t, types.ServerJID, query.To)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "privacy",
				}},
			}, nil
		}

		res, err := cli.GetStatusPrivacy(ctx)
		require.NoError(t, err)
		assert.Equal(t, DefaultStatusPrivacy, res)
	})

	t.Run("Success parsing multiple privacy lists with default reordering", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		user1 := types.NewJID("62811111", types.DefaultUserServer)
		user2 := types.NewJID("62822222", types.DefaultUserServer)

		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "privacy",
					Content: []waBinary.Node{
						{
							Tag: "non_list_tag",
						},
						{
							Tag: "list",
							Attrs: waBinary.Attrs{
								"type":    "contacts",
								"default": "false",
							},
						},
						{
							Tag: "list",
							Attrs: waBinary.Attrs{
								"type":    "whitelist",
								"default": "true",
							},
							Content: []waBinary.Node{
								{Tag: "user", Attrs: waBinary.Attrs{"jid": user1}},
								{Tag: "user", Attrs: waBinary.Attrs{"jid": user2}},
								{Tag: "unknown_child"},
							},
						},
					},
				}},
			}, nil
		}

		res, err := cli.GetStatusPrivacy(ctx)
		require.NoError(t, err)
		require.Len(t, res, 2)
		// Default should be moved to first place
		assert.True(t, res[0].IsDefault)
		assert.Equal(t, types.StatusPrivacyTypeWhitelist, res[0].Type)
		assert.Equal(t, []types.JID{user1, user2}, res[0].List)

		assert.False(t, res[1].IsDefault)
		assert.Equal(t, types.StatusPrivacyTypeContacts, res[1].Type)
	})
}

func TestBroadcast_GetBroadcastListParticipants(t *testing.T) {
	ctx := context.Background()
	ownUser := "62899999"
	ownJID := types.NewJID(ownUser, types.DefaultUserServer)
	userA := types.NewJID("6281111", types.DefaultUserServer)
	userB := types.NewJID("6282222", types.DefaultUserServer)

	setupBroadcastClient := func(own *types.JID) (*Client, *mockContactStore) {
		contactStore := newMockContactStore()
		device := &store.Device{
			ID:       own,
			Contacts: contactStore,
		}
		cli := NewClient(device, waLog.Noop)
		return cli, contactStore
	}

	t.Run("Unsupported non-status broadcast JID", func(t *testing.T) {
		cli, _ := setupBroadcastClient(&ownJID)
		otherBroadcastJID := types.NewJID("12345", types.BroadcastServer)

		list, err := cli.getBroadcastListParticipants(ctx, otherBroadcastJID)
		assert.Nil(t, list)
		assert.ErrorIs(t, err, ErrBroadcastListUnsupported)
	})

	t.Run("Error getting status privacy", func(t *testing.T) {
		cli, _ := setupBroadcastClient(&ownJID)
		expectedErr := errors.New("privacy error")
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, expectedErr
		}

		list, err := cli.getBroadcastListParticipants(ctx, types.StatusBroadcastJID)
		assert.Nil(t, list)
		assert.ErrorContains(t, err, "failed to get status privacy")
	})

	t.Run("Not logged in when ownID is empty", func(t *testing.T) {
		cli, _ := setupBroadcastClient(nil)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotFound // returns DefaultStatusPrivacy (contacts)
		}

		list, err := cli.getBroadcastListParticipants(ctx, types.StatusBroadcastJID)
		assert.Nil(t, list)
		assert.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("Whitelist mode returns whitelist recipients plus ownID", func(t *testing.T) {
		cli, _ := setupBroadcastClient(&ownJID)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "privacy",
					Content: []waBinary.Node{{
						Tag: "list",
						Attrs: waBinary.Attrs{
							"type":    "whitelist",
							"default": "true",
						},
						Content: []waBinary.Node{
							{Tag: "user", Attrs: waBinary.Attrs{"jid": userA}},
						},
					}},
				}},
			}, nil
		}

		list, err := cli.getBroadcastListParticipants(ctx, types.StatusBroadcastJID)
		require.NoError(t, err)
		assert.Equal(t, []types.JID{userA, ownJID}, list)
	})

	t.Run("Whitelist mode where ownID is already in list", func(t *testing.T) {
		cli, _ := setupBroadcastClient(&ownJID)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "privacy",
					Content: []waBinary.Node{{
						Tag: "list",
						Attrs: waBinary.Attrs{
							"type":    "whitelist",
							"default": "true",
						},
						Content: []waBinary.Node{
							{Tag: "user", Attrs: waBinary.Attrs{"jid": userA}},
							{Tag: "user", Attrs: waBinary.Attrs{"jid": ownJID}},
						},
					}},
				}},
			}, nil
		}

		list, err := cli.getBroadcastListParticipants(ctx, types.StatusBroadcastJID)
		require.NoError(t, err)
		assert.Equal(t, []types.JID{userA, ownJID}, list)
	})

	t.Run("Blacklist mode filters blacklisted contacts and contacts without full name", func(t *testing.T) {
		cli, contactStore := setupBroadcastClient(&ownJID)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "privacy",
					Content: []waBinary.Node{{
						Tag: "list",
						Attrs: waBinary.Attrs{
							"type":    "blacklist",
							"default": "true",
						},
						Content: []waBinary.Node{
							{Tag: "user", Attrs: waBinary.Attrs{"jid": userB}},
						},
					}},
				}},
			}, nil
		}

		_ = contactStore.PutContactName(ctx, userA, "User", "User A")
		_ = contactStore.PutContactName(ctx, userB, "User", "User B") // blacklisted
		userNoName := types.NewJID("6283333", types.DefaultUserServer)
		_ = contactStore.PutContactName(ctx, userNoName, "", "") // no full name

		list, err := cli.getBroadcastListParticipants(ctx, types.StatusBroadcastJID)
		require.NoError(t, err)
		// Should contain userA and ownJID, excluding userB (blacklisted) and userNoName
		assert.Contains(t, list, userA)
		assert.Contains(t, list, ownJID)
		assert.NotContains(t, list, userB)
		assert.NotContains(t, list, userNoName)
	})

	t.Run("Contact store error in getStatusBroadcastRecipients", func(t *testing.T) {
		cli, contactStore := setupBroadcastClient(&ownJID)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotFound
		}
		contactStore.getContactsErr = errors.New("db error")

		list, err := cli.getBroadcastListParticipants(ctx, types.StatusBroadcastJID)
		assert.Nil(t, list)
		assert.ErrorContains(t, err, "failed to get contact list from db")
	})
}
