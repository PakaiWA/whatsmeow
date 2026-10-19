// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/store"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func newTestUploadClient(handler func(req *http.Request) (*http.Response, error)) *Client {
	cli := NewClient(&store.Device{
		CompanionMetaNonce: "nonce-123",
	}, waLog.Noop)
	cli.mediaConnCache = &MediaConn{
		Auth:      "mock-auth-token",
		TTL:       3600,
		FetchedAt: time.Now(),
		Hosts: []MediaConnHost{
			{Hostname: "mmg.whatsapp.net"},
			{Hostname: "rupload.facebook.com"},
		},
	}
	cli.mediaHTTP = &http.Client{
		Transport: roundTripFunc(handler),
	}
	return cli
}

func mockUploadSuccessResponse() *http.Response {
	body := `{
		"url": "https://mmg.whatsapp.net/mms/image/xyz",
		"direct_path": "/v/t62.7118-24/xyz",
		"handle": "handle-abc",
		"object_id": "obj-123"
	}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestUpload_Upload(t *testing.T) {
	ctx := context.Background()
	plaintext := []byte("hello media world, image payload simulation")

	t.Run("Standard upload success", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, http.MethodPost, req.Method)
			assert.Contains(t, req.URL.Path, "/mms/image/")
			assert.Equal(t, "mock-auth-token", req.URL.Query().Get("auth"))
			return mockUploadSuccessResponse(), nil
		})

		resp, err := cli.Upload(ctx, plaintext, MediaImage)
		require.NoError(t, err)
		assert.Equal(t, "https://mmg.whatsapp.net/mms/image/xyz", resp.URL)
		assert.Equal(t, "/v/t62.7118-24/xyz", resp.DirectPath)
		assert.NotEmpty(t, resp.MediaKey)
		assert.NotEmpty(t, resp.FileSHA256)
		assert.NotEmpty(t, resp.FileEncSHA256)
		assert.Equal(t, uint64(len(plaintext)), resp.FileLength)
	})

	t.Run("MessengerConfig enabled audio becomes ptt", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "rupload.facebook.com", req.URL.Host)
			assert.Contains(t, req.URL.Path, "/wa-msgr/mms/ptt/")
			return mockUploadSuccessResponse(), nil
		})
		cli.MessengerConfig = &MessengerConfig{}

		resp, err := cli.Upload(ctx, plaintext, MediaAudio)
		require.NoError(t, err)
		assert.Equal(t, "handle-abc", resp.Handle)
	})

	t.Run("HTTP execution error", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network disconnect")
		})

		_, err := cli.Upload(ctx, plaintext, MediaImage)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute request")
	})

	t.Run("HTTP non-200 status code", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("server error")),
			}, nil
		})

		_, err := cli.Upload(ctx, plaintext, MediaImage)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "upload failed with status code 500")
	})

	t.Run("Malformed JSON response", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{invalid json")),
			}, nil
		})

		_, err := cli.Upload(ctx, plaintext, MediaImage)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse upload response")
	})
}

func TestUpload_UploadReader(t *testing.T) {
	ctx := context.Background()
	data := []byte("stream upload test data")

	t.Run("Auto temp file creation and deletion", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			return mockUploadSuccessResponse(), nil
		})

		resp, err := cli.UploadReader(ctx, bytes.NewReader(data), nil, MediaDocument)
		require.NoError(t, err)
		assert.Equal(t, "obj-123", resp.ObjectID)
		assert.Equal(t, uint64(len(data)), resp.FileLength)
	})

	t.Run("User provided temp file", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			return mockUploadSuccessResponse(), nil
		})

		tmp, err := os.CreateTemp("", "user-temp-*")
		require.NoError(t, err)
		defer func() {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}()

		resp, err := cli.UploadReader(ctx, bytes.NewReader(data), tmp, MediaDocument)
		require.NoError(t, err)
		assert.Equal(t, "handle-abc", resp.Handle)
	})
}

func TestUpload_UploadNewsletter(t *testing.T) {
	ctx := context.Background()
	data := []byte("newsletter unencrypted image data")

	t.Run("UploadNewsletter byte slice", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			assert.Contains(t, req.URL.Path, "/newsletter/newsletter-image/")
			return mockUploadSuccessResponse(), nil
		})

		resp, err := cli.UploadNewsletter(ctx, data, MediaImage)
		require.NoError(t, err)
		assert.Equal(t, "handle-abc", resp.Handle)
		assert.NotEmpty(t, resp.FileSHA256)
		assert.Nil(t, resp.MediaKey)
		assert.Nil(t, resp.FileEncSHA256)
	})

	t.Run("UploadNewsletterReader", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			return mockUploadSuccessResponse(), nil
		})

		resp, err := cli.UploadNewsletterReader(ctx, bytes.NewReader(data), MediaImage)
		require.NoError(t, err)
		assert.Equal(t, "obj-123", resp.ObjectID)
	})
}

func TestUpload_DeleteMedia(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty directPath returns nil immediately", func(t *testing.T) {
		cli := newTestUploadClient(nil)
		err := cli.DeleteMedia(ctx, MediaImage, "", []byte("hash"), "handle")
		require.NoError(t, err)
	})

	t.Run("Success with encHandle and query trimming", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, http.MethodDelete, req.Method)
			assert.Equal(t, "nonce-123", req.Header.Get("Companion_User_Secret"))
			assert.Equal(t, "my-handle", req.URL.Query().Get("e_handle"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})

		directPath := "/v/t62.7118-24/xyz?auth=oldauth"
		err := cli.DeleteMedia(ctx, MediaImage, directPath, []byte("file-hash"), "my-handle")
		require.NoError(t, err)
	})

	t.Run("HTTP error response", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
			}, nil
		})

		err := cli.DeleteMedia(ctx, MediaImage, "/v/t62/path", []byte("hash"), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "media delete failed with status code 404")
	})

	t.Run("Network error", func(t *testing.T) {
		cli := newTestUploadClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("delete net error")
		})

		err := cli.DeleteMedia(ctx, MediaImage, "/v/t62/path", []byte("hash"), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute request")
	})
}
