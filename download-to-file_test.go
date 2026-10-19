// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/proto/waMediaTransport"
)

// memFile implements File interface purely in-memory
type memFile struct {
	buf  []byte
	pos  int64
	lock sync.RWMutex
}

func newMemFile(initData []byte) *memFile {
	return &memFile{
		buf: append([]byte(nil), initData...),
	}
}

func (m *memFile) Read(p []byte) (n int, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.pos >= int64(len(m.buf)) {
		return 0, io.EOF
	}
	n = copy(p, m.buf[m.pos:])
	m.pos += int64(n)
	return n, nil
}

func (m *memFile) Write(p []byte) (n int, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	end := m.pos + int64(len(p))
	if end > int64(len(m.buf)) {
		newBuf := make([]byte, end)
		copy(newBuf, m.buf)
		m.buf = newBuf
	}
	n = copy(m.buf[m.pos:], p)
	m.pos += int64(n)
	return n, nil
}

func (m *memFile) Seek(offset int64, whence int) (int64, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = m.pos + offset
	case io.SeekEnd:
		newPos = int64(len(m.buf)) + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if newPos < 0 {
		return 0, errors.New("negative position")
	}
	m.pos = newPos
	return m.pos, nil
}

func (m *memFile) ReadAt(p []byte, off int64) (n int, err error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if off < 0 || off >= int64(len(m.buf)) {
		return 0, io.EOF
	}
	n = copy(p, m.buf[off:])
	if n < len(p) {
		err = io.EOF
	}
	return n, err
}

func (m *memFile) WriteAt(p []byte, off int64) (n int, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	end := off + int64(len(p))
	if end > int64(len(m.buf)) {
		newBuf := make([]byte, end)
		copy(newBuf, m.buf)
		m.buf = newBuf
	}
	n = copy(m.buf[off:], p)
	return n, nil
}

func (m *memFile) Truncate(size int64) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if size < 0 {
		return errors.New("negative size")
	}
	if size <= int64(len(m.buf)) {
		m.buf = m.buf[:size]
	} else {
		newBuf := make([]byte, size)
		copy(newBuf, m.buf)
		m.buf = newBuf
	}
	if m.pos > size {
		m.pos = size
	}
	return nil
}

type memFileInfo struct {
	size int64
}

func (m memFileInfo) Name() string       { return "memfile" }
func (m memFileInfo) Size() int64        { return m.size }
func (m memFileInfo) Mode() os.FileMode  { return 0644 }
func (m memFileInfo) ModTime() time.Time { return time.Now() }
func (m memFileInfo) IsDir() bool        { return false }
func (m memFileInfo) Sys() any           { return nil }

func (m *memFile) Stat() (os.FileInfo, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return memFileInfo{size: int64(len(m.buf))}, nil
}

func (m *memFile) Bytes() []byte {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return append([]byte(nil), m.buf...)
}

func TestDownloadToFile_Basics_And_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client returns ErrClientIsNil", func(t *testing.T) {
		var nilCli *Client
		mf := newMemFile(nil)
		err := nilCli.DownloadToFile(ctx, &waE2E.ImageMessage{}, mf)
		assert.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Unknown media type returns error", func(t *testing.T) {
		cli := newTestDownloadClient(nil)
		mf := newMemFile(nil)
		err := cli.DownloadToFile(ctx, nil, mf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown media type")
	})

	t.Run("Empty direct path returns ErrNoURLPresent", func(t *testing.T) {
		cli := newTestDownloadClient(nil)
		mf := newMemFile(nil)
		img := &waE2E.ImageMessage{DirectPath: proto.String("")}
		err := cli.DownloadToFile(ctx, img, mf)
		assert.ErrorIs(t, err, ErrNoURLPresent)
	})

	t.Run("Path without leading slash returns error", func(t *testing.T) {
		cli := newTestDownloadClient(nil)
		mf := newMemFile(nil)
		img := &waE2E.ImageMessage{DirectPath: proto.String("no-leading-slash")}
		err := cli.DownloadToFile(ctx, img, mf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "media download path does not start with slash")
	})
}

func TestDownloadToFile_DownloadMediaWithPathToFile_Success(t *testing.T) {
	ctx := context.Background()
	mediaKey := []byte("01234567890123456789012345678901")
	plaintext := []byte("this is a test file download to file content")
	rawEnc, encSHA, plainSHA := makeTestEncryptedMedia(mediaKey, MediaImage, plaintext)

	t.Run("Download encrypted image to file successfully", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			assert.Contains(t, req.URL.String(), "mms-type=image")
			return &http.Response{
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(bytes.NewReader(rawEnc)),
				ContentLength: int64(len(rawEnc)),
			}, nil
		})

		mf := newMemFile(nil)
		img := &waE2E.ImageMessage{
			DirectPath:    proto.String("/v/t62.7118-24/img-test"),
			MediaKey:      mediaKey,
			FileEncSHA256: encSHA,
			FileSHA256:    plainSHA,
		}

		err := cli.DownloadToFile(ctx, img, mf)
		require.NoError(t, err)
		assert.Equal(t, plaintext, mf.Bytes())
	})

	t.Run("DownloadFBToFile and DownloadMediaWithOnlyPathToFile", func(t *testing.T) {
		unencrypted := []byte("unencrypted file data")
		unencryptedSHA := sha256.Sum256(unencrypted)

		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(bytes.NewReader(unencrypted)),
				ContentLength: int64(len(unencrypted)),
			}, nil
		})

		// DownloadMediaWithOnlyPathToFile
		mf1 := newMemFile(nil)
		err := cli.DownloadMediaWithOnlyPathToFile(ctx, "/v/t62.7118-24/only-path", mf1)
		require.NoError(t, err)
		assert.Equal(t, unencrypted, mf1.Bytes())

		// DownloadFBToFile
		mf2 := newMemFile(nil)
		transport := &waMediaTransport.WAMediaTransport_Integral{
			DirectPath: proto.String("/v/t62.7118-24/fb-file"),
			FileSHA256: unencryptedSHA[:],
		}
		err = cli.DownloadFBToFile(ctx, transport, MediaDocument, mf2)
		require.NoError(t, err)
		assert.Equal(t, unencrypted, mf2.Bytes())
	})
}

func TestDownloadToFile_IntegrityErrors(t *testing.T) {
	ctx := context.Background()
	mediaKey := []byte("01234567890123456789012345678901")
	plaintext := []byte("integrity file payload")
	rawEnc, encSHA, plainSHA := makeTestEncryptedMedia(mediaKey, MediaImage, plaintext)

	t.Run("Short checksum length returns error", func(t *testing.T) {
		cli := newTestDownloadClient(nil)
		mf := newMemFile(nil)
		_, err := cli.downloadEncryptedMediaToFile(ctx, "http://localhost", []byte("short"), mf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid checksum length")
	})

	t.Run("File too short returns ErrTooShortFile", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("short"))),
			}, nil
		})

		mf := newMemFile(nil)
		_, err := cli.downloadEncryptedMediaToFile(ctx, "http://localhost", encSHA, mf)
		assert.ErrorIs(t, err, ErrTooShortFile)
	})

	t.Run("Invalid enc sha256 returns ErrInvalidMediaEncSHA256", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(rawEnc)),
			}, nil
		})

		mf := newMemFile(nil)
		wrongEncSHA := make([]byte, 32)
		_, err := cli.downloadEncryptedMediaToFile(ctx, "http://localhost", wrongEncSHA, mf)
		assert.ErrorIs(t, err, ErrInvalidMediaEncSHA256)
	})

	t.Run("Corrupted MAC returns ErrInvalidMediaHMAC", func(t *testing.T) {
		corruptedEnc := make([]byte, len(rawEnc))
		copy(corruptedEnc, rawEnc)
		corruptedEnc[len(corruptedEnc)-1] ^= 0xFF
		corruptedEncSHA := sha256.Sum256(corruptedEnc)

		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(corruptedEnc)),
			}, nil
		})

		mf := newMemFile(nil)
		err := cli.DownloadMediaWithPathToFile(ctx, "/v/t62.7118-24/badmac", corruptedEncSHA[:], plainSHA, mediaKey, MediaImage, "", false, mf)
		assert.ErrorIs(t, err, ErrInvalidMediaHMAC)
	})

	t.Run("Invalid plaintext sha256 returns ErrInvalidMediaSHA256", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(rawEnc)),
			}, nil
		})

		mf := newMemFile(nil)
		wrongPlainSHA := make([]byte, 32)
		err := cli.DownloadMediaWithPathToFile(ctx, "/v/t62.7118-24/badplain", encSHA, wrongPlainSHA, mediaKey, MediaImage, "", false, mf)
		assert.ErrorIs(t, err, ErrInvalidMediaSHA256)
	})

	t.Run("Unencrypted invalid sha256 returns ErrInvalidUnencryptedMediaSHA256", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("raw data"))),
			}, nil
		})

		mf := newMemFile(nil)
		wrongSHA := make([]byte, 32)
		err := cli.DownloadMediaWithPathToFile(ctx, "/v/t62.7118-24/unencbad", nil, wrongSHA, nil, "", "", false, mf)
		assert.ErrorIs(t, err, ErrInvalidUnencryptedMediaSHA256)
	})

	t.Run("Host fallback on network error and fail on last host", func(t *testing.T) {
		attempt := 0
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			attempt++
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("server error")),
			}, nil
		})

		mf := newMemFile(nil)
		err := cli.DownloadMediaWithPathToFile(ctx, "/v/t62.7118-24/allfail", encSHA, plainSHA, mediaKey, MediaImage, "", false, mf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download media from last host")
		assert.Equal(t, 2, attempt)
	})
}
