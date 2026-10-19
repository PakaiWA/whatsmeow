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

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockPrivacyTokenStore struct {
	sync.RWMutex
	tokens map[types.JID]*store.PrivacyToken
	getErr error
	putErr error
	delErr error
}

func newMockPrivacyTokenStore() *mockPrivacyTokenStore {
	return &mockPrivacyTokenStore{
		tokens: make(map[types.JID]*store.PrivacyToken),
	}
}

func (m *mockPrivacyTokenStore) setToken(user types.JID, tok *store.PrivacyToken) {
	m.Lock()
	defer m.Unlock()
	m.tokens[user] = tok
}

func (m *mockPrivacyTokenStore) getToken(user types.JID) *store.PrivacyToken {
	m.RLock()
	defer m.RUnlock()
	return m.tokens[user]
}

func (m *mockPrivacyTokenStore) PutPrivacyTokens(_ context.Context, tokens ...store.PrivacyToken) error {
	m.Lock()
	defer m.Unlock()
	if m.putErr != nil {
		return m.putErr
	}
	for _, tok := range tokens {
		tCopy := tok
		m.tokens[tok.User] = &tCopy
	}
	return nil
}

func (m *mockPrivacyTokenStore) GetPrivacyToken(_ context.Context, user types.JID) (*store.PrivacyToken, error) {
	m.RLock()
	defer m.RUnlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.tokens[user], nil
}

func (m *mockPrivacyTokenStore) DeleteExpiredPrivacyTokens(_ context.Context, cutoff time.Time) (int64, error) {
	m.Lock()
	defer m.Unlock()
	if m.delErr != nil {
		return 0, m.delErr
	}
	var count int64
	for k, v := range m.tokens {
		if v.Timestamp.Before(cutoff) {
			delete(m.tokens, k)
			count++
		}
	}
	return count, nil
}

func TestTCToken_CutoffAndExpiry(t *testing.T) {
	cutoff := currentTCTokenCutoffTimestamp()
	assert.False(t, cutoff.IsZero())

	// Zero time is always expired
	assert.True(t, isTCTokenExpired(time.Time{}))

	// Old timestamp is expired
	assert.True(t, isTCTokenExpired(cutoff.Add(-time.Hour)))

	// Future or recent timestamp is not expired
	assert.False(t, isTCTokenExpired(time.Now()))
}

func TestTCToken_ShouldSendNewTCToken(t *testing.T) {
	// Zero time should return true
	assert.True(t, shouldSendNewTCToken(time.Time{}))

	// Timestamp in current bucket should return false
	assert.False(t, shouldSendNewTCToken(time.Now()))

	// Timestamp older than 1 bucket duration should return true
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	assert.True(t, shouldSendNewTCToken(oldTime))
}

func TestTCToken_ShouldSendTCTokenInChatAction(t *testing.T) {
	userJID := types.NewJID("12345", types.DefaultUserServer)
	assert.True(t, shouldSendTCTokenInChatAction(userJID))

	hiddenUserJID := types.NewJID("lid123", types.HiddenUserServer)
	assert.True(t, shouldSendTCTokenInChatAction(hiddenUserJID))

	groupJID := types.NewJID("group123", types.GroupServer)
	assert.False(t, shouldSendTCTokenInChatAction(groupJID))

	psaJID := types.PSAJID
	assert.False(t, shouldSendTCTokenInChatAction(psaJID))

	botJID := types.NewJID("123", types.BotServer)
	assert.False(t, shouldSendTCTokenInChatAction(botJID))
}

func TestTCToken_ResolveStorageLID(t *testing.T) {
	ctx := context.Background()
	lidStore := newMockLIDStore()
	device := &store.Device{
		LIDs: lidStore,
	}
	cli := NewClient(device, waLog.Noop)

	userPN := types.NewJID("12345", types.DefaultUserServer)
	userLID := types.NewJID("lid-12345", types.HiddenUserServer)

	// No mapping -> returns storageJID
	resolved := cli.resolveTCTokenStorageLID(ctx, userPN)
	assert.Equal(t, userPN, resolved)

	// With LID mapping
	lidStore.pnToLID[userPN] = userLID
	resolved = cli.resolveTCTokenStorageLID(ctx, userPN)
	assert.Equal(t, userLID, resolved)

	// LID server -> returns directly without lookup
	resolved = cli.resolveTCTokenStorageLID(ctx, userLID)
	assert.Equal(t, userLID, resolved)

	// Error branch in GetLIDForPN
	lidStore.getLIDErr = errors.New("db error")
	resolved = cli.resolveTCTokenStorageLID(ctx, userPN)
	assert.Equal(t, userPN, resolved)
}

func TestTCToken_SenderTSMethods(t *testing.T) {
	device := &store.Device{}
	cli := NewClient(device, waLog.Noop)
	user := types.NewJID("12345", types.DefaultUserServer)

	// Initial is zero
	assert.True(t, cli.getTCTokenSenderTS(user).IsZero())

	// Setting timestamp
	now := time.Now()
	cli.setTCTokenSenderTS(user, now)
	assert.Equal(t, now, cli.getTCTokenSenderTS(user))

	// validateAndSet: already present returns true
	assert.True(t, cli.validateAndSetTCTokenSenderTS(user, now))

	// validateAndSet on new user:
	user2 := types.NewJID("67890", types.DefaultUserServer)
	// Zero timestamp returns false
	assert.False(t, cli.validateAndSetTCTokenSenderTS(user2, time.Time{}))
	// Expired timestamp returns false
	assert.False(t, cli.validateAndSetTCTokenSenderTS(user2, time.Now().Add(-60*24*time.Hour)))
	// Valid recent timestamp returns true and sets it
	assert.True(t, cli.validateAndSetTCTokenSenderTS(user2, now))
	assert.Equal(t, now, cli.getTCTokenSenderTS(user2))

	// Test unlockedCleanupTCTokenSenderTSMap
	// Force cleanup by setting lastTCTokenSenderTSCleanup far in past
	cli.lastTCTokenSenderTSCleanup = time.Now().Add(-10 * 24 * time.Hour)
	expiredUser := types.NewJID("expired", types.DefaultUserServer)
	cli.tcTokenSenderTS[expiredUser] = time.Now().Add(-60 * 24 * time.Hour)
	cli.unlockedCleanupTCTokenSenderTSMap()
	assert.True(t, cli.tcTokenSenderTS[expiredUser].IsZero())
}

func TestTCToken_EnsureTCToken(t *testing.T) {
	ctx := context.Background()
	user := types.NewJID("12345", types.DefaultUserServer)

	t.Run("Not logged in", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		tok, err := cli.ensureTCToken(ctx, user)
		require.ErrorIs(t, err, ErrNotLoggedIn)
		assert.Nil(t, tok)
	})

	t.Run("Store error", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		pStore.getErr = errors.New("db error")
		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			LIDs:          newMockLIDStore(),
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		tok, err := cli.ensureTCToken(ctx, user)
		require.Error(t, err)
		assert.Nil(t, tok)
	})

	t.Run("Existing token valid", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		now := time.Now()
		pStore.tokens[user] = &store.PrivacyToken{
			User:            user,
			Token:           []byte("valid-token-bytes"),
			Timestamp:       now,
			SenderTimestamp: now,
		}
		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			LIDs:          newMockLIDStore(),
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		tok, err := cli.ensureTCToken(ctx, user)
		require.NoError(t, err)
		assert.Equal(t, []byte("valid-token-bytes"), tok)
	})

	t.Run("Existing token expired or missing", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		pStore.setToken(user, &store.PrivacyToken{
			User:      user,
			Token:     []byte("expired-token"),
			Timestamp: time.Now().Add(-60 * 24 * time.Hour),
		})
		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			LIDs:          newMockLIDStore(),
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		tok, err := cli.ensureTCToken(ctx, user)
		require.NoError(t, err)
		assert.Nil(t, tok)
	})
}

func TestTCToken_IssuePrivacyTokenAndSave(t *testing.T) {
	user := types.NewJID("12345", types.DefaultUserServer)

	t.Run("Success", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		pStore.setToken(user, &store.PrivacyToken{
			User:      user,
			Token:     []byte("test-token"),
			Timestamp: time.Now(),
		})
		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			LIDs:          newMockLIDStore(),
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "privacy", query.Namespace)
			assert.Equal(t, iqSet, query.Type)
			return &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"type": "result"}}, nil
		}

		senderTS := time.Now()
		cli.issuePrivacyTokenAndSave(user, senderTS)

		assert.Equal(t, senderTS, cli.getTCTokenSenderTS(user))
		tok := pStore.getToken(user)
		require.NotNil(t, tok)
		assert.Equal(t, senderTS, tok.SenderTimestamp)
	})

	t.Run("IssuePrivacyToken error", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("network failure")
		}
		cli.issuePrivacyTokenAndSave(user, time.Now())
	})

	t.Run("GetPrivacyToken error", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		pStore.getErr = errors.New("get error")
		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		cli.issuePrivacyTokenAndSave(user, time.Now())
	})

	t.Run("PutPrivacyTokens error", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		pStore.setToken(user, &store.PrivacyToken{
			User:  user,
			Token: []byte("some-token"),
		})
		pStore.putErr = errors.New("put error")
		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		cli.issuePrivacyTokenAndSave(user, time.Now())
	})

	t.Run("Existing token is empty", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		pStore.setToken(user, &store.PrivacyToken{
			User:  user,
			Token: nil,
		})
		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		cli.issuePrivacyTokenAndSave(user, time.Now())
	})
}

func TestTCToken_DeleteExpiredPrivacyTokens(t *testing.T) {
	t.Run("Deleted count > 0", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		userOld := types.NewJID("old", types.DefaultUserServer)
		userNew := types.NewJID("new", types.DefaultUserServer)
		pStore.setToken(userOld, &store.PrivacyToken{
			User:      userOld,
			Timestamp: time.Now().Add(-60 * 24 * time.Hour),
		})
		pStore.setToken(userNew, &store.PrivacyToken{
			User:      userNew,
			Timestamp: time.Now(),
		})

		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		// Force prune
		cli.lastTCTokenDBPrune = time.Now().Add(-48 * time.Hour)
		cli.deleteExpiredPrivacyTokens()
		assert.Eventually(t, func() bool {
			return pStore.getToken(userOld) == nil && pStore.getToken(userNew) != nil
		}, 1*time.Second, 10*time.Millisecond)

		// Calling again immediately skips prune because interval hasn't passed
		cli.deleteExpiredPrivacyTokens()
	})

	t.Run("DB error during prune", func(t *testing.T) {
		pStore := newMockPrivacyTokenStore()
		pStore.delErr = errors.New("delete error")
		device := &store.Device{
			ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
			PrivacyTokens: pStore,
		}
		cli := NewClient(device, waLog.Noop)
		cli.lastTCTokenDBPrune = time.Now().Add(-48 * time.Hour)
		cli.deleteExpiredPrivacyTokens()
		time.Sleep(50 * time.Millisecond)
	})
}
