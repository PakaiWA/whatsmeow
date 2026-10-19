// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type customMockNCTSaltStore struct {
	sync.RWMutex
	salt   []byte
	getErr error
	putErr error
	delErr error
}

func (m *customMockNCTSaltStore) PutNCTSalt(_ context.Context, salt []byte) error {
	m.Lock()
	defer m.Unlock()
	if m.putErr != nil {
		return m.putErr
	}
	m.salt = append([]byte(nil), salt...)
	return nil
}

func (m *customMockNCTSaltStore) GetNCTSalt(_ context.Context) ([]byte, error) {
	m.RLock()
	defer m.RUnlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.salt, nil
}

func (m *customMockNCTSaltStore) DeleteNCTSalt(_ context.Context) error {
	m.Lock()
	defer m.Unlock()
	if m.delErr != nil {
		return m.delErr
	}
	m.salt = nil
	return nil
}

func TestCSToken_ShouldSendCsToken_Comprehensive(t *testing.T) {
	testCases := []struct {
		name     string
		jid      types.JID
		expected bool
	}{
		{
			name:     "Valid PN user",
			jid:      types.NewJID("628123456789", types.DefaultUserServer),
			expected: true,
		},
		{
			name:     "Valid LID user",
			jid:      types.NewJID("10001", types.HiddenUserServer),
			expected: true,
		},
		{
			name:     "Valid PN with AD device",
			jid:      types.NewADJID("628123456789", 0, 1),
			expected: true,
		},
		{
			name:     "Group server returns false",
			jid:      types.NewJID("12345-67890", types.GroupServer),
			expected: false,
		},
		{
			name:     "Broadcast server returns false",
			jid:      types.NewJID("status", types.BroadcastServer),
			expected: false,
		},
		{
			name:     "Newsletter server returns false",
			jid:      types.NewJID("123456", types.NewsletterServer),
			expected: false,
		},
		{
			name:     "PSA JID returns false",
			jid:      types.PSAJID,
			expected: false,
		},
		{
			name:     "Bot JID returns false",
			jid:      types.NewJID("13135550002", types.BotServer),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, shouldSendCsToken(tc.jid))
		})
	}
}

func TestCSToken_GenerateCsToken(t *testing.T) {
	ctx := context.Background()

	t.Run("Non-applicable JID returns nil", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		token := cli.generateCsToken(ctx, types.NewJID("12345", types.GroupServer))
		assert.Nil(t, token)
	})

	t.Run("NCT salt get error returns nil", func(t *testing.T) {
		nctStore := &customMockNCTSaltStore{getErr: errors.New("salt read err")}
		dev := &store.Device{NCTSalt: nctStore}
		cli := NewClient(dev, waLog.Noop)

		token := cli.generateCsToken(ctx, types.NewJID("628123456789", types.DefaultUserServer))
		assert.Nil(t, token)
	})

	t.Run("Empty NCT salt returns nil", func(t *testing.T) {
		nctStore := &customMockNCTSaltStore{salt: []byte{}}
		dev := &store.Device{NCTSalt: nctStore}
		cli := NewClient(dev, waLog.Noop)

		token := cli.generateCsToken(ctx, types.NewJID("628123456789", types.DefaultUserServer))
		assert.Nil(t, token)
	})

	t.Run("LID recipient successfully calculates HMAC", func(t *testing.T) {
		salt := []byte("secret-salt-1234567890123456")
		nctStore := &customMockNCTSaltStore{salt: salt}
		dev := &store.Device{NCTSalt: nctStore}
		cli := NewClient(dev, waLog.Noop)

		lid := types.NewJID("10001", types.HiddenUserServer)
		token := cli.generateCsToken(ctx, lid)
		require.NotNil(t, token)

		h := hmac.New(sha256.New, salt)
		h.Write([]byte(lid.String()))
		expected := h.Sum(nil)

		assert.Equal(t, expected, token)
	})

	t.Run("PN recipient resolves LID and calculates HMAC", func(t *testing.T) {
		salt := []byte("secret-salt-1234567890123456")
		nctStore := &customMockNCTSaltStore{salt: salt}
		lidStore := newMockLIDStore()

		pn := types.NewJID("628123456789", types.DefaultUserServer)
		lid := types.NewJID("10002", types.HiddenUserServer)
		_ = lidStore.PutLIDMapping(ctx, lid, pn)

		dev := &store.Device{
			NCTSalt: nctStore,
			LIDs:    lidStore,
		}
		cli := NewClient(dev, waLog.Noop)

		token := cli.generateCsToken(ctx, pn)
		require.NotNil(t, token)

		h := hmac.New(sha256.New, salt)
		h.Write([]byte(lid.String()))
		expected := h.Sum(nil)

		assert.Equal(t, expected, token)
	})

	t.Run("PN recipient GetLIDForPN error returns nil", func(t *testing.T) {
		salt := []byte("secret-salt-1234567890123456")
		nctStore := &customMockNCTSaltStore{salt: salt}
		lidStore := newMockLIDStore()
		lidStore.getLIDErr = errors.New("db error")

		pn := types.NewJID("628123456789", types.DefaultUserServer)
		dev := &store.Device{
			NCTSalt: nctStore,
			LIDs:    lidStore,
		}
		cli := NewClient(dev, waLog.Noop)

		token := cli.generateCsToken(ctx, pn)
		assert.Nil(t, token)
	})

	t.Run("PN recipient unresolved LID (empty) returns nil", func(t *testing.T) {
		salt := []byte("secret-salt-1234567890123456")
		nctStore := &customMockNCTSaltStore{salt: salt}
		lidStore := newMockLIDStore() // empty

		pn := types.NewJID("628123456789", types.DefaultUserServer)
		dev := &store.Device{
			NCTSalt: nctStore,
			LIDs:    lidStore,
		}
		cli := NewClient(dev, waLog.Noop)

		token := cli.generateCsToken(ctx, pn)
		assert.Nil(t, token)
	})

	t.Run("PN recipient resolved to non-HiddenUserServer returns nil", func(t *testing.T) {
		salt := []byte("secret-salt-1234567890123456")
		nctStore := &customMockNCTSaltStore{salt: salt}
		lidStore := newMockLIDStore()

		pn := types.NewJID("628123456789", types.DefaultUserServer)
		// invalid mapping to DefaultUserServer
		_ = lidStore.PutLIDMapping(ctx, types.NewJID("10003", types.DefaultUserServer), pn)

		dev := &store.Device{
			NCTSalt: nctStore,
			LIDs:    lidStore,
		}
		cli := NewClient(dev, waLog.Noop)

		token := cli.generateCsToken(ctx, pn)
		assert.Nil(t, token)
	})
}

func TestCSToken_StoreAndClearNCTSalt(t *testing.T) {
	ctx := context.Background()

	t.Run("Store empty salt is no-op", func(t *testing.T) {
		nctStore := &customMockNCTSaltStore{}
		dev := &store.Device{NCTSalt: nctStore}
		cli := NewClient(dev, waLog.Noop)

		err := cli.storeNCTSalt(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, nctStore.salt)

		err = cli.storeNCTSalt(ctx, []byte{})
		require.NoError(t, err)
		assert.Nil(t, nctStore.salt)
	})

	t.Run("Store valid salt saves to store", func(t *testing.T) {
		nctStore := &customMockNCTSaltStore{}
		dev := &store.Device{NCTSalt: nctStore}
		cli := NewClient(dev, waLog.Noop)

		salt := []byte("new-salt-32-bytes-abcdefghijklmn")
		err := cli.storeNCTSalt(ctx, salt)
		require.NoError(t, err)
		assert.Equal(t, salt, nctStore.salt)
	})

	t.Run("Store error returns error", func(t *testing.T) {
		nctStore := &customMockNCTSaltStore{putErr: errors.New("write error")}
		dev := &store.Device{NCTSalt: nctStore}
		cli := NewClient(dev, waLog.Noop)

		err := cli.storeNCTSalt(ctx, []byte("salt"))
		assert.ErrorContains(t, err, "write error")
	})

	t.Run("Clear salt delegates to DeleteNCTSalt", func(t *testing.T) {
		nctStore := &customMockNCTSaltStore{salt: []byte("existing")}
		dev := &store.Device{NCTSalt: nctStore}
		cli := NewClient(dev, waLog.Noop)

		err := cli.clearNCTSalt(ctx)
		require.NoError(t, err)
		assert.Nil(t, nctStore.salt)
	})

	t.Run("Clear salt error propagates", func(t *testing.T) {
		nctStore := &customMockNCTSaltStore{delErr: errors.New("delete error")}
		dev := &store.Device{NCTSalt: nctStore}
		cli := NewClient(dev, waLog.Noop)

		err := cli.clearNCTSalt(ctx)
		assert.ErrorContains(t, err, "delete error")
	})
}
