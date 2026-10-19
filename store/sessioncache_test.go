// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/libsignal/state/record"
)

type mockSessionStore struct {
	SessionStore
	getManySessionsFunc func(ctx context.Context, addresses []string) (map[string][]byte, error)
	putManySessionsFunc func(ctx context.Context, sessions map[string][]byte) error
}

func (m *mockSessionStore) GetManySessions(ctx context.Context, addresses []string) (map[string][]byte, error) {
	if m.getManySessionsFunc != nil {
		return m.getManySessionsFunc(ctx, addresses)
	}
	return nil, nil
}

func (m *mockSessionStore) PutManySessions(ctx context.Context, sessions map[string][]byte) error {
	if m.putManySessionsFunc != nil {
		return m.putManySessionsFunc(ctx, sessions)
	}
	return nil
}

func TestSessionCache_InternalHelpers(t *testing.T) {
	t.Run("getSessionCache nil context", func(t *testing.T) {
		nilContext := func() context.Context { return nil }
		assert.Nil(t, getSessionCache(nilContext()))
	})

	t.Run("getSessionCache no cache in context", func(t *testing.T) {
		assert.Nil(t, getSessionCache(context.Background()))
	})

	t.Run("getSessionCache invalid type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), contextKeySessionCache, "not a session cache")
		assert.Nil(t, getSessionCache(ctx))
	})

	t.Run("getCachedSession and putCachedSession with nil cache", func(t *testing.T) {
		ctx := context.Background()
		assert.Nil(t, getCachedSession(ctx, "addr1"))
		assert.False(t, putCachedSession(ctx, "addr1", record.NewSession(SignalProtobufSerializer.Session, SignalProtobufSerializer.State)))
	})
}

func TestDevice_WithCachedSessions_And_PutCachedSessions(t *testing.T) {
	t.Run("WithCachedSessions empty addresses", func(t *testing.T) {
		device := &Device{}
		existing, ctx, err := device.WithCachedSessions(context.Background(), nil)
		require.NoError(t, err)
		assert.Nil(t, existing)
		assert.Nil(t, getSessionCache(ctx))
	})

	t.Run("WithCachedSessions fetch error", func(t *testing.T) {
		mockErr := errors.New("db error")
		device := &Device{
			Sessions: &mockSessionStore{
				getManySessionsFunc: func(ctx context.Context, addresses []string) (map[string][]byte, error) {
					return nil, mockErr
				},
			},
		}

		existing, _, err := device.WithCachedSessions(context.Background(), []string{"addr1"})
		require.Error(t, err)
		assert.Nil(t, existing)
		assert.ErrorIs(t, err, mockErr)
	})

	t.Run("WithCachedSessions roundtrip success and PutCachedSessions", func(t *testing.T) {
		sessRecord := createRealSession()
		validBytes := sessRecord.Serialize()

		storedSessions := make(map[string][]byte)

		device := &Device{
			Sessions: &mockSessionStore{
				getManySessionsFunc: func(ctx context.Context, addresses []string) (map[string][]byte, error) {
					return map[string][]byte{
						"existing_addr": validBytes,
						"new_addr":      nil,
						"corrupt_addr":  []byte("not a valid session record protobuf"),
					}, nil
				},
				putManySessionsFunc: func(ctx context.Context, sessions map[string][]byte) error {
					for k, v := range sessions {
						storedSessions[k] = v
					}
					return nil
				},
			},
		}

		existing, ctx, err := device.WithCachedSessions(context.Background(), []string{"existing_addr", "new_addr", "corrupt_addr"})
		require.NoError(t, err)
		require.NotNil(t, existing)

		assert.True(t, existing["existing_addr"])
		assert.False(t, existing["new_addr"])
		assert.False(t, existing["corrupt_addr"])

		// getCachedSession
		sess := getCachedSession(ctx, "existing_addr")
		require.NotNil(t, sess)

		sessNew := getCachedSession(ctx, "new_addr")
		require.NotNil(t, sessNew)

		assert.Nil(t, getCachedSession(ctx, "unknown_addr"))
		assert.Nil(t, getCachedSession(ctx, "corrupt_addr"))

		// putCachedSession updates dirty
		realSess := createRealSession()
		updated := putCachedSession(ctx, "real_addr", realSess)
		assert.True(t, updated)

		// PutCachedSessions flushes dirty sessions
		err = device.PutCachedSessions(ctx)
		require.NoError(t, err)
		assert.Contains(t, storedSessions, "real_addr")

		// Subsequent PutCachedSessions has no dirty sessions
		err = device.PutCachedSessions(ctx)
		require.NoError(t, err)
	})

	t.Run("PutCachedSessions nil cache is no-op", func(t *testing.T) {
		device := &Device{}
		err := device.PutCachedSessions(context.Background())
		require.NoError(t, err)
	})

	t.Run("PutCachedSessions store error", func(t *testing.T) {
		storeErr := errors.New("put failed")
		device := &Device{
			Sessions: &mockSessionStore{
				getManySessionsFunc: func(ctx context.Context, addresses []string) (map[string][]byte, error) {
					return map[string][]byte{
						"addr1": nil,
					}, nil
				},
				putManySessionsFunc: func(ctx context.Context, sessions map[string][]byte) error {
					return storeErr
				},
			},
		}

		_, ctx, err := device.WithCachedSessions(context.Background(), []string{"addr1"})
		require.NoError(t, err)

		realSess := createRealSession()
		putCachedSession(ctx, "addr1", realSess)

		err = device.PutCachedSessions(ctx)
		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
	})
}
