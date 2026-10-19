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
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestMediaConn_Expiry(t *testing.T) {
	now := time.Now()
	mc := &MediaConn{
		FetchedAt: now,
		TTL:       60,
	}
	assert.Equal(t, now.Add(60*time.Second), mc.Expiry())
}

func TestMediaConn_RefreshMediaConn(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		mc, err := nilCli.refreshMediaConn(ctx, false)
		require.ErrorIs(t, err, ErrClientIsNil)
		assert.Nil(t, mc)
	})

	t.Run("Cache hit when valid and not forced", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cached := &MediaConn{
			Auth:      "cached-auth",
			TTL:       3600,
			FetchedAt: time.Now(),
		}
		cli.mediaConnCache = cached

		mc, err := cli.refreshMediaConn(ctx, false)
		require.NoError(t, err)
		assert.Equal(t, "cached-auth", mc.Auth)
	})

	t.Run("Refreshes when expired or forced", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.mediaConnCache = &MediaConn{
			Auth:      "expired-auth",
			TTL:       1,
			FetchedAt: time.Now().Add(-10 * time.Second),
		}

		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "w:m", query.Namespace)
			assert.Equal(t, iqSet, query.Type)
			assert.Equal(t, types.ServerJID, query.To)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "media_conn",
					Attrs: waBinary.Attrs{
						"auth":        "new-auth-token",
						"ttl":         "300",
						"auth_ttl":    "120",
						"max_buckets": "4",
					},
					Content: []waBinary.Node{
						{Tag: "host", Attrs: waBinary.Attrs{"hostname": "mmg.whatsapp.net"}},
						{Tag: "other_tag"},
					},
				}},
			}, nil
		}

		mc, err := cli.refreshMediaConn(ctx, false)
		require.NoError(t, err)
		assert.Equal(t, "new-auth-token", mc.Auth)
		assert.Equal(t, 300, mc.TTL)
		assert.Equal(t, 120, mc.AuthTTL)
		assert.Equal(t, 4, mc.MaxBuckets)
		require.Len(t, mc.Hosts, 1)
		assert.Equal(t, "mmg.whatsapp.net", mc.Hosts[0].Hostname)

		// Force refresh
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "media_conn",
					Attrs: waBinary.Attrs{
						"auth":        "forced-auth-token",
						"ttl":         "300",
						"auth_ttl":    "120",
						"max_buckets": "4",
					},
				}},
			}, nil
		}
		mcForced, err := cli.refreshMediaConn(ctx, true)
		require.NoError(t, err)
		assert.Equal(t, "forced-auth-token", mcForced.Auth)
	})

	t.Run("Query error propagates", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("network failure")
		}
		_, err := cli.refreshMediaConn(ctx, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to query media connections")
	})
}

func TestMediaConn_QueryMediaConn_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty response children", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.queryMediaConn(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected child tag")
	})

	t.Run("Wrong child tag", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "wrong_tag",
				}},
			}, nil
		}
		_, err := cli.queryMediaConn(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected child tag")
	})

	t.Run("Missing required attributes in media_conn", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "media_conn",
					Attrs: waBinary.Attrs{
						"auth": "auth-only",
						// missing ttl, auth_ttl, max_buckets
					},
				}},
			}, nil
		}
		_, err := cli.queryMediaConn(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse media connections")
	})

	t.Run("Missing hostname in host child", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "media_conn",
					Attrs: waBinary.Attrs{
						"auth":        "auth",
						"ttl":         "10",
						"auth_ttl":    "10",
						"max_buckets": "1",
					},
					Content: []waBinary.Node{
						{Tag: "host", Attrs: waBinary.Attrs{}}, // missing hostname
					},
				}},
			}, nil
		}
		_, err := cli.queryMediaConn(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse media connection host")
	})
}
