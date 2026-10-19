// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/proto/waMediaTransport"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/util/cbcutil"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

// Helper to construct encrypted media payload with MAC
func makeTestEncryptedMedia(mediaKey []byte, appInfo MediaType, plaintext []byte) (ciphertextWithMAC []byte, encSHA256, origSHA256 []byte) {
	iv, cipherKey, macKey, _ := getMediaKeys(mediaKey, appInfo)
	ciphertext, err := cbcutil.Encrypt(cipherKey, iv, plaintext)
	if err != nil {
		panic(err)
	}

	h := hmac.New(sha256.New, macKey)
	h.Write(iv)
	h.Write(ciphertext)
	mac := h.Sum(nil)[:mediaHMACLength]

	ciphertextWithMAC = append(ciphertext, mac...)
	encSum := sha256.Sum256(ciphertextWithMAC)
	encSHA256 = encSum[:]
	plainSum := sha256.Sum256(plaintext)
	origSHA256 = plainSum[:]
	return
}

func newTestDownloadClient(handler func(req *http.Request) (*http.Response, error)) *Client {
	cli := NewClient(&store.Device{}, waLog.Noop)
	cli.mediaConnCache = &MediaConn{
		Auth:      "mock-auth-token",
		TTL:       3600,
		FetchedAt: time.Now(),
		Hosts: []MediaConnHost{
			{Hostname: "mmg.whatsapp.net"},
			{Hostname: "backup.whatsapp.net"},
		},
	}
	cli.mediaHTTP = &http.Client{
		Transport: roundTripFunc(handler),
	}
	return cli
}

func TestDownload_GetMediaType(t *testing.T) {
	t.Run("Known protobuf types", func(t *testing.T) {
		assert.Equal(t, MediaImage, GetMediaType(&waE2E.ImageMessage{}))
		assert.Equal(t, MediaAudio, GetMediaType(&waE2E.AudioMessage{}))
		assert.Equal(t, MediaVideo, GetMediaType(&waE2E.VideoMessage{}))
		assert.Equal(t, MediaDocument, GetMediaType(&waE2E.DocumentMessage{}))
		assert.Equal(t, MediaImage, GetMediaType(&waE2E.StickerMessage{}))
	})

	t.Run("StickerPackItem", func(t *testing.T) {
		assert.Equal(t, MediaImage, GetMediaType(&types.StickerPackItem{}))
	})

	t.Run("Unknown type", func(t *testing.T) {
		assert.Empty(t, GetMediaType(nil))
	})
}

func TestDownload_DownloadAny(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil message returns ErrNothingDownloadableFound", func(t *testing.T) {
		cli := newTestDownloadClient(nil)
		_, err := cli.DownloadAny(ctx, nil)
		assert.ErrorIs(t, err, ErrNothingDownloadableFound)
	})

	t.Run("Empty message returns ErrNothingDownloadableFound", func(t *testing.T) {
		cli := newTestDownloadClient(nil)
		_, err := cli.DownloadAny(ctx, &waE2E.Message{})
		assert.ErrorIs(t, err, ErrNothingDownloadableFound)
	})

	t.Run("Dispatches to respective non-nil message types", func(t *testing.T) {
		mediaKey := []byte("01234567890123456789012345678901")
		plaintext := []byte("hello media payload")

		for _, mt := range []struct {
			name    string
			makeMsg func(raw []byte, encHash, plainHash []byte) *waE2E.Message
			appInfo MediaType
		}{
			{
				name:    "ImageMessage",
				appInfo: MediaImage,
				makeMsg: func(raw, enc, plain []byte) *waE2E.Message {
					return &waE2E.Message{
						ImageMessage: &waE2E.ImageMessage{
							DirectPath:    proto.String("/v/t62.7118-24/img"),
							MediaKey:      mediaKey,
							FileEncSHA256: enc,
							FileSHA256:    plain,
						},
					}
				},
			},
			{
				name:    "VideoMessage",
				appInfo: MediaVideo,
				makeMsg: func(raw, enc, plain []byte) *waE2E.Message {
					return &waE2E.Message{
						VideoMessage: &waE2E.VideoMessage{
							DirectPath:    proto.String("/v/t62.7118-24/vid"),
							MediaKey:      mediaKey,
							FileEncSHA256: enc,
							FileSHA256:    plain,
						},
					}
				},
			},
			{
				name:    "AudioMessage",
				appInfo: MediaAudio,
				makeMsg: func(raw, enc, plain []byte) *waE2E.Message {
					return &waE2E.Message{
						AudioMessage: &waE2E.AudioMessage{
							DirectPath:    proto.String("/v/t62.7118-24/aud"),
							MediaKey:      mediaKey,
							FileEncSHA256: enc,
							FileSHA256:    plain,
						},
					}
				},
			},
			{
				name:    "DocumentMessage",
				appInfo: MediaDocument,
				makeMsg: func(raw, enc, plain []byte) *waE2E.Message {
					return &waE2E.Message{
						DocumentMessage: &waE2E.DocumentMessage{
							DirectPath:    proto.String("/v/t62.7118-24/doc"),
							MediaKey:      mediaKey,
							FileEncSHA256: enc,
							FileSHA256:    plain,
						},
					}
				},
			},
			{
				name:    "StickerMessage",
				appInfo: MediaImage,
				makeMsg: func(raw, enc, plain []byte) *waE2E.Message {
					return &waE2E.Message{
						StickerMessage: &waE2E.StickerMessage{
							DirectPath:    proto.String("/v/t62.7118-24/stk"),
							MediaKey:      mediaKey,
							FileEncSHA256: enc,
							FileSHA256:    plain,
						},
					}
				},
			},
		} {
			t.Run(mt.name, func(t *testing.T) {
				rawEnc, encSHA, plainSHA := makeTestEncryptedMedia(mediaKey, mt.appInfo, plaintext)
				cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(rawEnc)),
					}, nil
				})

				msg := mt.makeMsg(rawEnc, encSHA, plainSHA)
				data, err := cli.DownloadAny(ctx, msg)
				require.NoError(t, err)
				assert.Equal(t, plaintext, data)
			})
		}
	})
}

func TestDownload_DownloadThumbnail(t *testing.T) {
	ctx := context.Background()

	t.Run("No URL present returns ErrNoURLPresent", func(t *testing.T) {
		cli := newTestDownloadClient(nil)
		extMsg := &waE2E.ExtendedTextMessage{}
		_, err := cli.DownloadThumbnail(ctx, extMsg)
		assert.ErrorIs(t, err, ErrNoURLPresent)
	})

	t.Run("Valid thumbnail download", func(t *testing.T) {
		mediaKey := []byte("01234567890123456789012345678901")
		plaintext := []byte("thumbnail jpg data")
		rawEnc, encSHA, plainSHA := makeTestEncryptedMedia(mediaKey, MediaLinkThumbnail, plaintext)

		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(rawEnc)),
			}, nil
		})

		extMsg := &waE2E.ExtendedTextMessage{
			ThumbnailDirectPath: proto.String("/v/t62.7118-24/thumb"),
			MediaKey:            mediaKey,
			ThumbnailEncSHA256:  encSHA,
			ThumbnailSHA256:     plainSHA,
		}

		data, err := cli.DownloadThumbnail(ctx, extMsg)
		require.NoError(t, err)
		assert.Equal(t, plaintext, data)
	})
}

func TestDownload_FetchStickerPack(t *testing.T) {
	ctx := context.Background()

	t.Run("Successful sticker pack fetch", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			assert.Contains(t, req.URL.String(), "cat=sticker_pack_data&id=pack123")
			jsonPayload := `[{"sticker-pack-id":"pack123","name":"Cute Cat Stickers"}]`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(jsonPayload)),
			}, nil
		})

		pack, err := cli.FetchStickerPack(ctx, "pack123")
		require.NoError(t, err)
		assert.Equal(t, "pack123", pack.StickerPackID)
		assert.Equal(t, "Cute Cat Stickers", pack.Name)
	})

	t.Run("Empty array in response returns error", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("[]")),
			}, nil
		})

		_, err := cli.FetchStickerPack(ctx, "pack123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no sticker pack found in response")
	})

	t.Run("Invalid JSON returns decode error", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("invalid-json")),
			}, nil
		})

		_, err := cli.FetchStickerPack(ctx, "pack123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode response")
	})
}

func TestDownload_Download_And_DownloadFB(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client returns ErrClientIsNil", func(t *testing.T) {
		var nilCli *Client
		_, err := nilCli.Download(ctx, &waE2E.ImageMessage{})
		assert.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("No URL returns ErrNoURLPresent", func(t *testing.T) {
		cli := newTestDownloadClient(nil)
		_, err := cli.Download(ctx, &waE2E.ImageMessage{})
		assert.ErrorIs(t, err, ErrNoURLPresent)
	})

	t.Run("Path without leading slash returns error", func(t *testing.T) {
		cli := newTestDownloadClient(nil)
		img := &waE2E.ImageMessage{DirectPath: proto.String("no-slash/path")}
		_, err := cli.Download(ctx, img)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "media download path does not start with slash")
	})

	t.Run("DownloadFB and DownloadMediaWithOnlyPath", func(t *testing.T) {
		unencryptedPayload := []byte("plain unencrypted blob")
		unencryptedSum := sha256.Sum256(unencryptedPayload)

		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(unencryptedPayload)),
			}, nil
		})

		// DownloadMediaWithOnlyPath
		data, err := cli.DownloadMediaWithOnlyPath(ctx, "/v/t62.7118-24/plain")
		require.NoError(t, err)
		assert.Equal(t, unencryptedPayload, data)

		// DownloadFB unencrypted
		transport := &waMediaTransport.WAMediaTransport_Integral{
			DirectPath: proto.String("/v/t62.7118-24/fb-doc"),
			FileSHA256: unencryptedSum[:],
		}
		fbData, err := cli.DownloadFB(ctx, transport, MediaDocument)
		require.NoError(t, err)
		assert.Equal(t, unencryptedPayload, fbData)
	})

	t.Run("Fallback to next host on retryable error", func(t *testing.T) {
		mediaKey := []byte("01234567890123456789012345678901")
		plaintext := []byte("fallback host data")
		rawEnc, encSHA, plainSHA := makeTestEncryptedMedia(mediaKey, MediaImage, plaintext)

		attempt := 0
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			attempt++
			if attempt == 1 {
				// First host fails with 500
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("server error")),
				}, nil
			}
			// Second host succeeds
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(rawEnc)),
			}, nil
		})

		img := &waE2E.ImageMessage{
			DirectPath:    proto.String("/v/t62.7118-24/img-fallback"),
			MediaKey:      mediaKey,
			FileEncSHA256: encSHA,
			FileSHA256:    plainSHA,
		}
		data, err := cli.Download(ctx, img)
		require.NoError(t, err)
		assert.Equal(t, plaintext, data)
		assert.Equal(t, 2, attempt)
	})

	t.Run("Non-retryable 404 error returns immediately", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
			}, nil
		})

		img := &waE2E.ImageMessage{
			DirectPath:    proto.String("/v/t62.7118-24/img-404"),
			MediaKey:      make([]byte, 32),
			FileEncSHA256: make([]byte, 32),
			FileSHA256:    make([]byte, 32),
		}
		_, err := cli.Download(ctx, img)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrMediaDownloadFailedWith404))
	})
}

func TestDownload_Validation_And_Integrity(t *testing.T) {
	ctx := context.Background()
	mediaKey := []byte("01234567890123456789012345678901")
	plaintext := []byte("validation payload")
	rawEnc, encSHA, plainSHA := makeTestEncryptedMedia(mediaKey, MediaImage, plaintext)

	t.Run("File too short returns ErrTooShortFile", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("short"))),
			}, nil
		})

		_, err := cli.DownloadMediaWithPath(ctx, "/v/t62.7118-24/short", encSHA, plainSHA, mediaKey, MediaImage, "", false)
		assert.ErrorIs(t, err, ErrTooShortFile)
	})

	t.Run("Invalid enc sha256 returns ErrInvalidMediaEncSHA256", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(rawEnc)),
			}, nil
		})

		wrongEncSHA := make([]byte, 32)
		_, err := cli.DownloadMediaWithPath(ctx, "/v/t62.7118-24/badenc", wrongEncSHA, plainSHA, mediaKey, MediaImage, "", false)
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

		_, err := cli.DownloadMediaWithPath(ctx, "/v/t62.7118-24/badmac", corruptedEncSHA[:], plainSHA, mediaKey, MediaImage, "", false)
		assert.ErrorIs(t, err, ErrInvalidMediaHMAC)
	})

	t.Run("Invalid plaintext sha256 returns ErrInvalidMediaSHA256", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(rawEnc)),
			}, nil
		})

		wrongPlainSHA := make([]byte, 32)
		_, err := cli.DownloadMediaWithPath(ctx, "/v/t62.7118-24/badplain", encSHA, wrongPlainSHA, mediaKey, MediaImage, "", false)
		assert.ErrorIs(t, err, ErrInvalidMediaSHA256)
	})

	t.Run("Unencrypted invalid sha256 returns ErrInvalidUnencryptedMediaSHA256", func(t *testing.T) {
		cli := newTestDownloadClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("raw content"))),
			}, nil
		})

		wrongSHA := make([]byte, 32)
		_, err := cli.DownloadMediaWithPath(ctx, "/v/t62.7118-24/unencbad", nil, wrongSHA, nil, "", "", false)
		assert.ErrorIs(t, err, ErrInvalidUnencryptedMediaSHA256)
	})
}
