// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/store"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetLatestVersion(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36", req.Header.Get("User-Agent"))
				assert.Equal(t, "document", req.Header.Get("Sec-Fetch-Dest"))
				body := `<html><head><script>var config = {"client_revision":24151234, "foo":"bar"};</script></head></html>`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		}

		ver, err := GetLatestVersion(ctx, client)
		require.NoError(t, err)
		assert.Equal(t, &store.WAVersionContainer{2, 3000, 24151234}, ver)
	})

	t.Run("HTTP client error", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("network timeout")
			}),
		}
		ver, err := GetLatestVersion(ctx, client)
		require.Error(t, err)
		assert.Nil(t, ver)
		assert.Contains(t, err.Error(), "failed to send request")
	})

	t.Run("Non-200 status code", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(strings.NewReader("bad gateway")),
				}, nil
			}),
		}
		ver, err := GetLatestVersion(ctx, client)
		require.Error(t, err)
		assert.Nil(t, ver)
		assert.Contains(t, err.Error(), "unexpected response with status 502")
	})

	t.Run("Version number not found", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("<html>no revision here</html>")),
				}, nil
			}),
		}
		ver, err := GetLatestVersion(ctx, client)
		require.Error(t, err)
		assert.Nil(t, ver)
		assert.Contains(t, err.Error(), "version number not found")
	})

	t.Run("Version number parse error", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"client_revision":999999999999999999999999999999,}`)),
				}, nil
			}),
		}
		ver, err := GetLatestVersion(ctx, client)
		require.Error(t, err)
		assert.Nil(t, ver)
		assert.Contains(t, err.Error(), "failed to parse version number")
	})

	t.Run("Invalid context", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, req.Context().Err()
			}),
		}
		ver, err := GetLatestVersion(canceledCtx, client)
		require.Error(t, err)
		assert.Nil(t, ver)
	})

	t.Run("Nil HTTP client fallback", func(t *testing.T) {
		oldTransport := http.DefaultTransport
		defer func() { http.DefaultTransport = oldTransport }()
		http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"client_revision":999999,}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		ver, err := GetLatestVersion(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, &store.WAVersionContainer{2, 3000, 999999}, ver)
	})
}
