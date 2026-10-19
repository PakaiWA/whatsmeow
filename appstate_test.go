// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/appstate"
	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/proto/waServerSync"
	"github.com/PakaiWA/whatsmeow/proto/waSyncAction"
	"github.com/PakaiWA/whatsmeow/proto/waSyncdSnapshotRecovery"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockFullAppStateStore struct {
	lock        sync.RWMutex
	versions    map[string]uint64
	hashes      map[string][128]byte
	mutationMAC map[string][]byte

	putVerErr    error
	getVerErr    error
	deleteVerErr error
}

func newMockFullAppStateStore() *mockFullAppStateStore {
	return &mockFullAppStateStore{
		versions:    make(map[string]uint64),
		hashes:      make(map[string][128]byte),
		mutationMAC: make(map[string][]byte),
	}
}

func (m *mockFullAppStateStore) PutAppStateVersion(_ context.Context, name string, version uint64, hash [128]byte) error {
	if m.putVerErr != nil {
		return m.putVerErr
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.versions[name] = version
	m.hashes[name] = hash
	return nil
}

func (m *mockFullAppStateStore) GetAppStateVersion(_ context.Context, name string) (uint64, [128]byte, error) {
	if m.getVerErr != nil {
		return 0, [128]byte{}, m.getVerErr
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.versions[name], m.hashes[name], nil
}

func (m *mockFullAppStateStore) DeleteAppStateVersion(_ context.Context, name string) error {
	if m.deleteVerErr != nil {
		return m.deleteVerErr
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.versions, name)
	delete(m.hashes, name)
	return nil
}

func (m *mockFullAppStateStore) PutAppStateMutationMACs(_ context.Context, _ string, _ uint64, _ []store.AppStateMutationMAC) error {
	return nil
}

func (m *mockFullAppStateStore) DeleteAppStateMutationMACs(_ context.Context, _ string, _ [][]byte) error {
	return nil
}

func (m *mockFullAppStateStore) GetAppStateMutationMAC(_ context.Context, name string, indexMAC []byte) ([]byte, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.mutationMAC[name+string(indexMAC)], nil
}

type mockAppStateSyncKeyStore struct {
	lock      sync.RWMutex
	keys      map[string]*store.AppStateSyncKey
	latestKey []byte
}

func newMockAppStateSyncKeyStore() *mockAppStateSyncKeyStore {
	return &mockAppStateSyncKeyStore{
		keys: make(map[string]*store.AppStateSyncKey),
	}
}

func (m *mockAppStateSyncKeyStore) PutAppStateSyncKey(_ context.Context, id []byte, key store.AppStateSyncKey) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.keys[string(id)] = &key
	m.latestKey = id
	return nil
}

func (m *mockAppStateSyncKeyStore) GetAppStateSyncKey(_ context.Context, id []byte) (*store.AppStateSyncKey, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.keys[string(id)], nil
}

func (m *mockAppStateSyncKeyStore) GetLatestAppStateSyncKeyID(_ context.Context) ([]byte, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.latestKey, nil
}

func (m *mockAppStateSyncKeyStore) GetAllAppStateSyncKeys(_ context.Context) ([]*store.AppStateSyncKey, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	res := make([]*store.AppStateSyncKey, 0, len(m.keys))
	for _, k := range m.keys {
		res = append(res, k)
	}
	return res, nil
}

type mockChatSettingsStore struct {
	lock       sync.RWMutex
	mutedUntil map[types.JID]time.Time
	pinned     map[types.JID]bool
	archived   map[types.JID]bool
	wasaRootID map[types.JID]types.MessageID
}

func newMockChatSettingsStore() *mockChatSettingsStore {
	return &mockChatSettingsStore{
		mutedUntil: make(map[types.JID]time.Time),
		pinned:     make(map[types.JID]bool),
		archived:   make(map[types.JID]bool),
		wasaRootID: make(map[types.JID]types.MessageID),
	}
}

func (m *mockChatSettingsStore) PutMutedUntil(_ context.Context, chat types.JID, mutedUntil time.Time) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.mutedUntil[chat] = mutedUntil
	return nil
}

func (m *mockChatSettingsStore) PutPinned(_ context.Context, chat types.JID, pinned bool) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.pinned[chat] = pinned
	return nil
}

func (m *mockChatSettingsStore) PutArchived(_ context.Context, chat types.JID, archived bool) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.archived[chat] = archived
	return nil
}

func (m *mockChatSettingsStore) PutWASARootSecretID(_ context.Context, chat types.JID, id types.MessageID) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.wasaRootID[chat] = id
	return nil
}

func (m *mockChatSettingsStore) GetWASARootSecretID(_ context.Context, chat types.JID) (types.MessageID, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.wasaRootID[chat], nil
}

func (m *mockChatSettingsStore) GetChatSettings(_ context.Context, chat types.JID) (types.LocalChatSettings, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return types.LocalChatSettings{
		MutedUntil: m.mutedUntil[chat],
		Pinned:     m.pinned[chat],
		Archived:   m.archived[chat],
	}, nil
}

func setupTestAppStateClient() (*Client, *mockFullAppStateStore, *mockChatSettingsStore, *mockContactStore) {
	appStateStore := newMockFullAppStateStore()
	appStateKeys := newMockAppStateSyncKeyStore()
	chatSettings := newMockChatSettingsStore()
	contacts := newMockContactStore()

	device := &store.Device{
		ID:           &types.JID{User: "12345", Server: types.DefaultUserServer},
		AppState:     appStateStore,
		AppStateKeys: appStateKeys,
		ChatSettings: chatSettings,
		Contacts:     contacts,
		MsgSecrets:   &mockMsgSecretStore{},
	}
	cli := NewClient(device, waLog.Noop)
	return cli, appStateStore, chatSettings, contacts
}

func TestAppState_MarkNotDirty(t *testing.T) {
	ctx := context.Background()
	cli, _, _, _ := setupTestAppStateClient()

	var sentQuery infoQuery
	cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
		sentQuery = query
		return &waBinary.Node{Tag: "iq"}, nil
	}

	ts := time.Unix(1700000000, 0)
	err := cli.MarkNotDirty(ctx, "account_sync", ts)
	require.NoError(t, err)
	assert.Equal(t, "urn:xmpp:whatsapp:dirty", sentQuery.Namespace)
	assert.Equal(t, iqSet, sentQuery.Type)
	assert.Equal(t, types.ServerJID, sentQuery.To)

	nodes, ok := sentQuery.Content.([]waBinary.Node)
	require.True(t, ok)
	require.Len(t, nodes, 1)
	assert.Equal(t, "clean", nodes[0].Tag)
	assert.Equal(t, "account_sync", nodes[0].Attrs["type"])
	assert.Equal(t, ts.Unix(), nodes[0].Attrs["timestamp"])
}

func TestAppState_DispatchAppState_AllCases(t *testing.T) {
	ctx := context.Background()
	cli, _, chatSettings, contacts := setupTestAppStateClient()

	chatJID := types.NewJID("62811111", types.DefaultUserServer)
	senderJID := types.NewJID("62822222", types.DefaultUserServer)
	tsMilli := int64(1700000000123)

	t.Run("Non-SET operation returns nil", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_REMOVE,
			Index:     []string{appstate.IndexMute, chatJID.String()},
			Action:    &waSyncAction.SyncActionValue{},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		assert.Nil(t, evt)
	})

	t.Run("IndexMute - Muted forever", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexMute, chatJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				MuteAction: &waSyncAction.MuteAction{
					Muted:            proto.Bool(true),
					MuteEndTimestamp: proto.Int64(-1),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		muteEvt, ok := evt.(*events.Mute)
		require.True(t, ok)
		assert.Equal(t, chatJID, muteEvt.JID)
		assert.True(t, muteEvt.Action.GetMuted())
		assert.Equal(t, store.MutedForever, chatSettings.mutedUntil[chatJID])
	})

	t.Run("IndexMute - Muted with timestamp", func(t *testing.T) {
		endTs := tsMilli + 86400000
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexMute, chatJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				MuteAction: &waSyncAction.MuteAction{
					Muted:            proto.Bool(true),
					MuteEndTimestamp: proto.Int64(endTs),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		require.IsType(t, &events.Mute{}, evt)
		assert.Equal(t, time.UnixMilli(endTs), chatSettings.mutedUntil[chatJID])
	})

	t.Run("IndexPin", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexPin, chatJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				PinAction: &waSyncAction.PinAction{
					Pinned: proto.Bool(true),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		pinEvt, ok := evt.(*events.Pin)
		require.True(t, ok)
		assert.Equal(t, chatJID, pinEvt.JID)
		assert.True(t, pinEvt.Action.GetPinned())
		assert.True(t, chatSettings.pinned[chatJID])
	})

	t.Run("IndexArchive", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexArchive, chatJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				ArchiveChatAction: &waSyncAction.ArchiveChatAction{
					Archived: proto.Bool(true),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularLow, mut, false)
		archEvt, ok := evt.(*events.Archive)
		require.True(t, ok)
		assert.Equal(t, chatJID, archEvt.JID)
		assert.True(t, archEvt.Action.GetArchived())
		assert.True(t, chatSettings.archived[chatJID])
	})

	t.Run("IndexContact", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexContact, chatJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				ContactAction: &waSyncAction.ContactAction{
					FirstName: proto.String("Budi"),
					FullName:  proto.String("Budi Santoso"),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchCriticalUnblockLow, mut, false)
		contactEvt, ok := evt.(*events.Contact)
		require.True(t, ok)
		assert.Equal(t, chatJID, contactEvt.JID)
		assert.Equal(t, "Budi", contactEvt.Action.GetFirstName())
		assert.Equal(t, "Budi Santoso", contacts.allContacts[chatJID].FullName)
	})

	t.Run("IndexClearChat with deleteMedia", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexClearChat, chatJID.String(), "0", "1"},
			Action: &waSyncAction.SyncActionValue{
				Timestamp:       proto.Int64(tsMilli),
				ClearChatAction: &waSyncAction.ClearChatAction{},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		clearEvt, ok := evt.(*events.ClearChat)
		require.True(t, ok)
		assert.Equal(t, chatJID, clearEvt.JID)
		assert.True(t, clearEvt.DeleteMedia)
	})

	t.Run("IndexDeleteChat with deleteMedia", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexDeleteChat, chatJID.String(), "1"},
			Action: &waSyncAction.SyncActionValue{
				Timestamp:        proto.Int64(tsMilli),
				DeleteChatAction: &waSyncAction.DeleteChatAction{},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		delEvt, ok := evt.(*events.DeleteChat)
		require.True(t, ok)
		assert.Equal(t, chatJID, delEvt.JID)
		assert.True(t, delEvt.DeleteMedia)
	})

	t.Run("IndexStar valid and short", func(t *testing.T) {
		// Short index should return nil
		shortMut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexStar, chatJID.String()},
			Action:    &waSyncAction.SyncActionValue{},
		}
		assert.Nil(t, cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, shortMut, false))

		// Valid Star
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexStar, chatJID.String(), "MSG-123", "1", senderJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp:  proto.Int64(tsMilli),
				StarAction: &waSyncAction.StarAction{Starred: proto.Bool(true)},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		starEvt, ok := evt.(*events.Star)
		require.True(t, ok)
		assert.Equal(t, chatJID, starEvt.ChatJID)
		assert.Equal(t, "MSG-123", starEvt.MessageID)
		assert.True(t, starEvt.IsFromMe)
		assert.Equal(t, senderJID, starEvt.SenderJID)
	})

	t.Run("IndexDeleteMessageForMe valid and short", func(t *testing.T) {
		// Short index returns nil
		shortMut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexDeleteMessageForMe, chatJID.String()},
			Action:    &waSyncAction.SyncActionValue{},
		}
		assert.Nil(t, cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, shortMut, false))

		// Valid DeleteMessageForMe
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexDeleteMessageForMe, chatJID.String(), "MSG-999", "0", "0"},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				DeleteMessageForMeAction: &waSyncAction.DeleteMessageForMeAction{
					DeleteMedia: proto.Bool(true),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		delForMe, ok := evt.(*events.DeleteForMe)
		require.True(t, ok)
		assert.Equal(t, chatJID, delForMe.ChatJID)
		assert.Equal(t, "MSG-999", delForMe.MessageID)
		assert.False(t, delForMe.IsFromMe)
		assert.True(t, delForMe.SenderJID.IsEmpty())
	})

	t.Run("IndexMarkChatAsRead", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexMarkChatAsRead, chatJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				MarkChatAsReadAction: &waSyncAction.MarkChatAsReadAction{
					Read: proto.Bool(true),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularLow, mut, false)
		readEvt, ok := evt.(*events.MarkChatAsRead)
		require.True(t, ok)
		assert.Equal(t, chatJID, readEvt.JID)
		assert.True(t, readEvt.Action.GetRead())
	})

	t.Run("IndexSettingPushName updates store", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexSettingPushName},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				PushNameSetting: &waSyncAction.PushNameSetting{
					Name: proto.String("My New PushName"),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchCriticalBlock, mut, false)
		pushEvt, ok := evt.(*events.PushNameSetting)
		require.True(t, ok)
		assert.Equal(t, "My New PushName", pushEvt.Action.GetName())
		assert.Equal(t, "My New PushName", cli.Store.PushName)
	})

	t.Run("IndexSettingUnarchiveChats", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexSettingUnarchiveChats},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				UnarchiveChatsSetting: &waSyncAction.UnarchiveChatsSetting{
					UnarchiveChats: proto.Bool(true),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularLow, mut, false)
		unarchEvt, ok := evt.(*events.UnarchiveChatsSetting)
		require.True(t, ok)
		assert.True(t, unarchEvt.Action.GetUnarchiveChats())
	})

	t.Run("IndexUserStatusMute", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexUserStatusMute, chatJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				UserStatusMuteAction: &waSyncAction.UserStatusMuteAction{
					Muted: proto.Bool(true),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		usmEvt, ok := evt.(*events.UserStatusMute)
		require.True(t, ok)
		assert.Equal(t, chatJID, usmEvt.JID)
		assert.True(t, usmEvt.Action.GetMuted())
	})

	t.Run("IndexLabelEdit", func(t *testing.T) {
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexLabelEdit, "LABEL-1"},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				LabelEditAction: &waSyncAction.LabelEditAction{
					Name: proto.String("Important"),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularLow, mut, false)
		leEvt, ok := evt.(*events.LabelEdit)
		require.True(t, ok)
		assert.Equal(t, "LABEL-1", leEvt.LabelID)
		assert.Equal(t, "Important", leEvt.Action.GetName())
	})

	t.Run("IndexLabelAssociationChat", func(t *testing.T) {
		// Short index
		shortMut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexLabelAssociationChat, "LABEL-1"},
			Action:    &waSyncAction.SyncActionValue{},
		}
		assert.Nil(t, cli.dispatchAppState(ctx, appstate.WAPatchRegularLow, shortMut, false))

		// Valid
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexLabelAssociationChat, "LABEL-1", chatJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				LabelAssociationAction: &waSyncAction.LabelAssociationAction{
					Labeled: proto.Bool(true),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularLow, mut, false)
		lacEvt, ok := evt.(*events.LabelAssociationChat)
		require.True(t, ok)
		assert.Equal(t, "LABEL-1", lacEvt.LabelID)
		assert.Equal(t, chatJID, lacEvt.JID)
		assert.True(t, lacEvt.Action.GetLabeled())
	})

	t.Run("IndexLabelAssociationMessage", func(t *testing.T) {
		// Short index
		shortMut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexLabelAssociationMessage, "LABEL-1"},
			Action:    &waSyncAction.SyncActionValue{},
		}
		assert.Nil(t, cli.dispatchAppState(ctx, appstate.WAPatchRegularLow, shortMut, false))

		// Valid
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexLabelAssociationMessage, "LABEL-1", chatJID.String(), "MSG-555", "1", "0"},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				LabelAssociationAction: &waSyncAction.LabelAssociationAction{
					Labeled: proto.Bool(true),
				},
			},
		}
		evt := cli.dispatchAppState(ctx, appstate.WAPatchRegularLow, mut, false)
		lamEvt, ok := evt.(*events.LabelAssociationMessage)
		require.True(t, ok)
		assert.Equal(t, "LABEL-1", lamEvt.LabelID)
		assert.Equal(t, chatJID, lamEvt.JID)
		assert.Equal(t, "MSG-555", lamEvt.MessageID)
	})

	t.Run("IndexWasaRootSecretAction", func(t *testing.T) {
		// Short index
		shortMut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexWasaRootSecretAction},
			Action:    &waSyncAction.SyncActionValue{},
		}
		assert.Nil(t, cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, shortMut, false))

		// Valid
		botJID := types.NewJID("muse_bot", types.BotServer)
		mut := appstate.Mutation{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{appstate.IndexWasaRootSecretAction, botJID.String()},
			Action: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(tsMilli),
				WasaRootSecretAction: &waSyncAction.WASARootSecretAction{
					Secrets: []*waSyncAction.WASARootSecretAction_RootSecretEntry{
						{
							ID:         proto.String("SECRET-1"),
							RootSecret: []byte("root-secret-bytes"),
							Status:     waSyncAction.WASARootSecretAction_RootSecretEntry_ACTIVE.Enum(),
							Epoch:      proto.Int64(10),
						},
					},
				},
			},
		}
		_ = cli.dispatchAppState(ctx, appstate.WAPatchRegularHigh, mut, false)
		assert.Equal(t, types.MessageID("SECRET-1"), chatSettings.wasaRootID[botJID])
	})
}

func TestAppState_FilterContacts_And_CollectEvents(t *testing.T) {
	ctx := context.Background()
	cli, _, _, contacts := setupTestAppStateClient()

	user1 := types.NewJID("628111", types.DefaultUserServer)
	user2 := types.NewJID("628222", types.DefaultUserServer)

	mutations := []appstate.Mutation{
		{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{"contact", user1.String()},
			Action: &waSyncAction.SyncActionValue{
				ContactAction: &waSyncAction.ContactAction{
					FirstName: proto.String("User"),
					FullName:  proto.String("User One"),
				},
			},
		},
		{
			Operation: waServerSync.SyncdMutation_SET,
			Index:     []string{"pin", user2.String()},
			Action: &waSyncAction.SyncActionValue{
				PinAction: &waSyncAction.PinAction{Pinned: proto.Bool(true)},
			},
		},
	}

	t.Run("filterContacts separates contacts and other mutations", func(t *testing.T) {
		mutsCopy := make([]appstate.Mutation, len(mutations))
		copy(mutsCopy, mutations)
		filtered, contactEntries := cli.filterContacts(mutsCopy)
		require.Len(t, contactEntries, 1)
		assert.Equal(t, user1, contactEntries[0].JID)
		assert.Equal(t, "User One", contactEntries[0].FullName)

		require.Len(t, filtered, 1)
		assert.Equal(t, "pin", filtered[0].Index[0])
	})

	t.Run("collectEventsToDispatch mass inserts contacts on fullSync of CriticalUnblockLow", func(t *testing.T) {
		var eventsToDispatch []any
		mutsCopy := make([]appstate.Mutation, len(mutations))
		copy(mutsCopy, mutations)
		err := cli.collectEventsToDispatch(ctx, appstate.WAPatchCriticalUnblockLow, mutsCopy, true, &eventsToDispatch)
		require.NoError(t, err)
		assert.Equal(t, "User One", contacts.allContacts[user1].FullName)
		require.NotEmpty(t, eventsToDispatch)
	})
}

func TestAppState_FetchAppState_ClientValidation_And_Reset(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client returns ErrClientIsNil", func(t *testing.T) {
		var nilCli *Client
		err := nilCli.FetchAppState(ctx, appstate.WAPatchRegularHigh, false, false)
		assert.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("onlyIfNotSynced skips when version > 0", func(t *testing.T) {
		cli, appStateStore, _, _ := setupTestAppStateClient()
		_ = appStateStore.PutAppStateVersion(ctx, string(appstate.WAPatchRegularHigh), 5, [128]byte{1, 2, 3})

		err := cli.FetchAppState(ctx, appstate.WAPatchRegularHigh, false, true)
		require.NoError(t, err)
	})

	t.Run("Full sync resets app state version", func(t *testing.T) {
		cli, appStateStore, _, _ := setupTestAppStateClient()
		_ = appStateStore.PutAppStateVersion(ctx, string(appstate.WAPatchRegularHigh), 5, [128]byte{1, 2, 3})

		// Mock IQ error to verify version reset happened before fetch
		expectedErr := errors.New("network error on fetch")
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, expectedErr
		}

		err := cli.FetchAppState(ctx, appstate.WAPatchRegularHigh, true, false)
		assert.ErrorContains(t, err, "failed to fetch app state")

		ver, _, _ := appStateStore.GetAppStateVersion(ctx, string(appstate.WAPatchRegularHigh))
		assert.Equal(t, uint64(0), ver)
	})
}

func TestAppState_SendAppState_Validation_And_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client returns ErrClientIsNil", func(t *testing.T) {
		var nilCli *Client
		err := nilCli.SendAppState(ctx, appstate.PatchInfo{})
		assert.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("No app state keys found error", func(t *testing.T) {
		cli, _, _, _ := setupTestAppStateClient()
		// AppStateKeys has no latest key set

		err := cli.SendAppState(ctx, appstate.PatchInfo{Type: appstate.WAPatchRegularHigh})
		assert.ErrorContains(t, err, "no app state keys found")
	})

	t.Run("Server returns error collection node without conflict", func(t *testing.T) {
		cli, _, _, _ := setupTestAppStateClient()
		latestKeyID := []byte("key-id-123")
		_ = cli.Store.AppStateKeys.PutAppStateSyncKey(ctx, latestKeyID, store.AppStateSyncKey{})

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "sync",
					Content: []waBinary.Node{{
						Tag: "collection",
						Attrs: waBinary.Attrs{
							"type": "error",
						},
						Content: []waBinary.Node{{
							Tag:   "error",
							Attrs: waBinary.Attrs{"code": "500"},
						}},
					}},
				}},
			}, nil
		}

		err := cli.SendAppState(ctx, appstate.PatchInfo{Type: appstate.WAPatchRegularHigh})
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrAppStateUpdate)
	})
}

func TestAppState_HandleAppStateRecovery(t *testing.T) {
	ctx := context.Background()
	cli, appStateStore, _, _ := setupTestAppStateClient()

	t.Run("Empty result returns true", func(t *testing.T) {
		res := cli.handleAppStateRecovery(ctx, "REQ-1", nil)
		assert.True(t, res)
	})

	t.Run("Result with nil SyncdSnapshotFatalRecoveryResponse returns true", func(t *testing.T) {
		res := cli.handleAppStateRecovery(ctx, "REQ-2", []*waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult{
			{},
		})
		assert.True(t, res)
	})

	t.Run("Invalid recovery blob returns true without crash", func(t *testing.T) {
		res := cli.handleAppStateRecovery(ctx, "REQ-3", []*waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult{
			{
				SyncdSnapshotFatalRecoveryResponse: &waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult_SyncDSnapshotFatalRecoveryResponse{
					CollectionSnapshot: []byte("not-a-valid-protobuf-syncd-snapshot"),
				},
			},
		})
		assert.True(t, res)
	})

	t.Run("Store get version error returns true", func(t *testing.T) {
		snapshot := &waSyncdSnapshotRecovery.SyncdSnapshotRecovery{
			CollectionName: proto.String(string(appstate.WAPatchRegularHigh)),
			Version: &waSyncdSnapshotRecovery.SyncdVersion{
				Version: proto.Uint64(10),
			},
		}
		raw, err := proto.Marshal(snapshot)
		require.NoError(t, err)

		appStateStore.getVerErr = errors.New("db error")
		defer func() { appStateStore.getVerErr = nil }()

		res := cli.handleAppStateRecovery(ctx, "REQ-4", []*waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult{
			{
				SyncdSnapshotFatalRecoveryResponse: &waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult_SyncDSnapshotFatalRecoveryResponse{
					CollectionSnapshot: raw,
				},
			},
		})
		assert.True(t, res)
	})

	t.Run("Current version >= recovery version is ignored", func(t *testing.T) {
		snapshot := &waSyncdSnapshotRecovery.SyncdSnapshotRecovery{
			CollectionName: proto.String(string(appstate.WAPatchRegularHigh)),
			Version: &waSyncdSnapshotRecovery.SyncdVersion{
				Version: proto.Uint64(5),
			},
		}
		raw, err := proto.Marshal(snapshot)
		require.NoError(t, err)

		_ = appStateStore.PutAppStateVersion(ctx, string(appstate.WAPatchRegularHigh), 10, [128]byte{})

		res := cli.handleAppStateRecovery(ctx, "REQ-5", []*waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult{
			{
				SyncdSnapshotFatalRecoveryResponse: &waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult_SyncDSnapshotFatalRecoveryResponse{
					CollectionSnapshot: raw,
				},
			},
		})
		assert.True(t, res)
	})
}
