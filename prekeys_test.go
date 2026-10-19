// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockCustomPreKeyStore struct {
	lock         sync.Mutex
	preKeys      []*keys.PreKey
	uploadedUpTo uint32
	genErr       error
	uploadErr    error
}

func newMockCustomPreKeyStore() *mockCustomPreKeyStore {
	return &mockCustomPreKeyStore{
		preKeys: make([]*keys.PreKey, 0),
	}
}

func (m *mockCustomPreKeyStore) GetOrGenPreKeys(_ context.Context, count uint32) ([]*keys.PreKey, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.genErr != nil {
		return nil, m.genErr
	}
	startID := uint32(len(m.preKeys) + 1)
	gen := make([]*keys.PreKey, count)
	for i := uint32(0); i < count; i++ {
		gen[i] = keys.NewPreKey(startID + i)
	}
	m.preKeys = append(m.preKeys, gen...)
	return gen, nil
}

func (m *mockCustomPreKeyStore) GenOnePreKey(_ context.Context) (*keys.PreKey, error) {
	return keys.NewPreKey(1), nil
}

func (m *mockCustomPreKeyStore) GetPreKey(_ context.Context, id uint32) (*keys.PreKey, error) {
	return nil, nil
}

func (m *mockCustomPreKeyStore) RemovePreKey(_ context.Context, id uint32) error {
	return nil
}

func (m *mockCustomPreKeyStore) MarkPreKeysAsUploaded(_ context.Context, upToID uint32) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.uploadErr != nil {
		return m.uploadErr
	}
	m.uploadedUpTo = upToID
	return nil
}

func (m *mockCustomPreKeyStore) UploadedPreKeyCount(_ context.Context) (int, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.preKeys), nil
}

func TestPrekeys_GetServerPreKeyCount(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("Success", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "encrypt", query.Namespace)
			assert.Equal(t, iqGet, query.Type)
			assert.Equal(t, types.ServerJID, query.To)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:   "count",
					Attrs: waBinary.Attrs{"value": "42"},
				}},
			}, nil
		}

		count, err := cli.getServerPreKeyCount(ctx)
		require.NoError(t, err)
		assert.Equal(t, 42, count)
	})

	t.Run("Error sendIQ", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("network failure")
		}
		_, err := cli.getServerPreKeyCount(ctx)
		require.Error(t, err)
	})
}

func TestPrekeys_UploadPreKeys(t *testing.T) {
	ctx := context.Background()

	t.Run("Skip upload if recently uploaded and server has >= WantedPreKeyCount", func(t *testing.T) {
		dev := &store.Device{
			RegistrationID: 1234,
			IdentityKey:    keys.NewKeyPair(),
		}
		dev.SignedPreKey = dev.IdentityKey.CreateSignedPreKey(1)
		mockPK := newMockCustomPreKeyStore()
		dev.PreKeys = mockPK
		cli := NewClient(dev, waLog.Noop)
		cli.lastPreKeyUpload = time.Now()

		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			// getServerPreKeyCount
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:   "count",
					Attrs: waBinary.Attrs{"value": "55"},
				}},
			}, nil
		}

		cli.uploadPreKeys(ctx, false)
		assert.False(t, cli.PreKeysUploaded.IsSet())
	})

	t.Run("GetOrGenPreKeys error", func(t *testing.T) {
		dev := &store.Device{
			RegistrationID: 1234,
			IdentityKey:    keys.NewKeyPair(),
		}
		mockPK := newMockCustomPreKeyStore()
		mockPK.genErr = errors.New("gen prekeys failed")
		dev.PreKeys = mockPK
		cli := NewClient(dev, waLog.Noop)

		cli.uploadPreKeys(ctx, false)
		assert.False(t, cli.PreKeysUploaded.IsSet())
	})

	t.Run("SendIQ error during upload", func(t *testing.T) {
		dev := &store.Device{
			RegistrationID: 1234,
			IdentityKey:    keys.NewKeyPair(),
		}
		dev.SignedPreKey = dev.IdentityKey.CreateSignedPreKey(1)
		mockPK := newMockCustomPreKeyStore()
		dev.PreKeys = mockPK
		cli := NewClient(dev, waLog.Noop)

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("iq error")
		}

		cli.uploadPreKeys(ctx, true)
		assert.False(t, cli.PreKeysUploaded.IsSet())
	})

	t.Run("MarkPreKeysAsUploaded error", func(t *testing.T) {
		dev := &store.Device{
			RegistrationID: 1234,
			IdentityKey:    keys.NewKeyPair(),
		}
		dev.SignedPreKey = dev.IdentityKey.CreateSignedPreKey(1)
		mockPK := newMockCustomPreKeyStore()
		mockPK.uploadErr = errors.New("db error")
		dev.PreKeys = mockPK
		cli := NewClient(dev, waLog.Noop)

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}

		cli.uploadPreKeys(ctx, false)
		assert.False(t, cli.PreKeysUploaded.IsSet())
	})

	t.Run("Upload success sets PreKeysUploaded and timestamp", func(t *testing.T) {
		dev := &store.Device{
			RegistrationID: 1234,
			IdentityKey:    keys.NewKeyPair(),
		}
		dev.SignedPreKey = dev.IdentityKey.CreateSignedPreKey(1)
		mockPK := newMockCustomPreKeyStore()
		dev.PreKeys = mockPK
		cli := NewClient(dev, waLog.Noop)

		var capturedContent []waBinary.Node
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			if nodes, ok := query.Content.([]waBinary.Node); ok {
				capturedContent = nodes
			}
			return &waBinary.Node{Tag: "iq"}, nil
		}

		cli.uploadPreKeys(ctx, false)
		assert.True(t, cli.PreKeysUploaded.IsSet())
		assert.False(t, cli.lastPreKeyUpload.IsZero())
		require.Len(t, capturedContent, 5)
		assert.Equal(t, "registration", capturedContent[0].Tag)
		assert.Equal(t, "type", capturedContent[1].Tag)
		assert.Equal(t, "identity", capturedContent[2].Tag)
		assert.Equal(t, "list", capturedContent[3].Tag)
		assert.Equal(t, "skey", capturedContent[4].Tag)
	})
}

func TestPrekeys_FetchPreKeys(t *testing.T) {
	ctx := context.Background()
	userJID := types.NewJID("123456", types.DefaultUserServer)
	userJID.Device = 1
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("fetchPreKeysNoError with empty slice", func(t *testing.T) {
		res := cli.fetchPreKeysNoError(ctx, nil)
		assert.Nil(t, res)
	})

	t.Run("fetchPreKeys error response", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("iq fail")
		}
		res, err := cli.fetchPreKeys(ctx, []types.JID{userJID})
		require.Error(t, err)
		assert.Nil(t, res)

		noErrRes := cli.fetchPreKeysNoError(ctx, []types.JID{userJID})
		assert.Nil(t, noErrRes)
	})

	t.Run("fetchPreKeys empty response node children", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.fetchPreKeys(ctx, []types.JID{userJID})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "got empty response to prekey request")
	})

	t.Run("fetchPreKeys with valid bundle and error bundle", func(t *testing.T) {
		user1 := types.NewJID("1111", types.DefaultUserServer)
		user1.Device = 1
		user2 := types.NewJID("2222", types.DefaultUserServer)
		user2.Device = 2

		// Construct valid bundle XML for user1
		keyPair1 := keys.NewKeyPair()
		signedPK := keyPair1.CreateSignedPreKey(5)
		preKeyGen := keys.NewPreKey(1)

		var regBytes [4]byte
		binary.BigEndian.PutUint32(regBytes[:], 9999)

		var pkIDBytes [4]byte
		binary.BigEndian.PutUint32(pkIDBytes[:], preKeyGen.KeyID)
		var spkIDBytes [4]byte
		binary.BigEndian.PutUint32(spkIDBytes[:], signedPK.KeyID)

		validUserNode := waBinary.Node{
			Tag:   "user",
			Attrs: waBinary.Attrs{"jid": user1},
			Content: []waBinary.Node{
				{Tag: "registration", Content: regBytes[:]},
				{
					Tag: "keys",
					Content: []waBinary.Node{
						{Tag: "identity", Content: keyPair1.Pub[:]},
						{
							Tag: "key",
							Content: []waBinary.Node{
								{Tag: "id", Content: pkIDBytes[1:]},
								{Tag: "value", Content: preKeyGen.Pub[:]},
							},
						},
						{
							Tag: "skey",
							Content: []waBinary.Node{
								{Tag: "id", Content: spkIDBytes[1:]},
								{Tag: "value", Content: signedPK.Pub[:]},
								{Tag: "signature", Content: signedPK.Signature[:]},
							},
						},
					},
				},
			},
		}

		errUserNode := waBinary.Node{
			Tag:   "user",
			Attrs: waBinary.Attrs{"jid": user2},
			Content: []waBinary.Node{
				{Tag: "error", Attrs: waBinary.Attrs{"code": "404"}},
			},
		}

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "list",
					Content: []waBinary.Node{
						validUserNode,
						errUserNode,
						{Tag: "non-user-tag"},
					},
				}},
			}, nil
		}

		res, err := cli.fetchPreKeys(ctx, []types.JID{user1, user2})
		require.NoError(t, err)
		require.Len(t, res, 2)
		assert.NoError(t, res[user1].err)
		assert.NotNil(t, res[user1].bundle)
		assert.Error(t, res[user2].err)
		assert.Nil(t, res[user2].bundle)

		// Also verify fetchPreKeysNoError ignores the user2 error and returns user1 bundle
		noErrMap := cli.fetchPreKeysNoError(ctx, []types.JID{user1, user2})
		require.NotNil(t, noErrMap)
		assert.NotNil(t, noErrMap[user1])
		assert.Nil(t, noErrMap[user2])
	})
}

func TestPrekeys_NodeToPreKey_Errors(t *testing.T) {
	t.Run("Missing id tag", func(t *testing.T) {
		node := waBinary.Node{Tag: "key"}
		_, err := nodeToPreKey(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "doesn't contain ID tag")
	})

	t.Run("ID unexpected content type", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "key",
			Content: []waBinary.Node{
				{Tag: "id", Content: 123},
			},
		}
		_, err := nodeToPreKey(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected content")
	})

	t.Run("ID unexpected length", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "key",
			Content: []waBinary.Node{
				{Tag: "id", Content: []byte{1, 2}},
			},
		}
		_, err := nodeToPreKey(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected number of bytes")
	})

	t.Run("Missing value tag", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "key",
			Content: []waBinary.Node{
				{Tag: "id", Content: []byte{1, 2, 3}},
			},
		}
		_, err := nodeToPreKey(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "doesn't contain value tag")
	})

	t.Run("Value unexpected content type", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "key",
			Content: []waBinary.Node{
				{Tag: "id", Content: []byte{1, 2, 3}},
				{Tag: "value", Content: "not-bytes"},
			},
		}
		_, err := nodeToPreKey(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected content")
	})

	t.Run("Value unexpected length", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "key",
			Content: []waBinary.Node{
				{Tag: "id", Content: []byte{1, 2, 3}},
				{Tag: "value", Content: []byte{1, 2, 3}},
			},
		}
		_, err := nodeToPreKey(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected number of bytes")
	})

	t.Run("skey missing signature", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "skey",
			Content: []waBinary.Node{
				{Tag: "id", Content: []byte{1, 2, 3}},
				{Tag: "value", Content: make([]byte, 32)},
			},
		}
		_, err := nodeToPreKey(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "doesn't contain signature tag")
	})

	t.Run("skey signature invalid content type", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "skey",
			Content: []waBinary.Node{
				{Tag: "id", Content: []byte{1, 2, 3}},
				{Tag: "value", Content: make([]byte, 32)},
				{Tag: "signature", Content: 123},
			},
		}
		_, err := nodeToPreKey(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected content")
	})

	t.Run("skey signature invalid length", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "skey",
			Content: []waBinary.Node{
				{Tag: "id", Content: []byte{1, 2, 3}},
				{Tag: "value", Content: make([]byte, 32)},
				{Tag: "signature", Content: make([]byte, 30)},
			},
		}
		_, err := nodeToPreKey(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected number of bytes")
	})
}

func TestPrekeys_NodeToPreKeyBundle_Errors(t *testing.T) {
	t.Run("Invalid registration bytes", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "user",
			Content: []waBinary.Node{
				{Tag: "registration", Content: []byte{1, 2}},
			},
		}
		_, err := nodeToPreKeyBundle(1, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid registration ID")
	})

	t.Run("Invalid identity key", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "user",
			Content: []waBinary.Node{
				{Tag: "registration", Content: []byte{1, 2, 3, 4}},
				{Tag: "identity", Content: []byte{1, 2}},
			},
		}
		_, err := nodeToPreKeyBundle(1, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid identity key")
	})

	t.Run("Invalid prekey node inside bundle", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "user",
			Content: []waBinary.Node{
				{Tag: "registration", Content: []byte{1, 2, 3, 4}},
				{Tag: "identity", Content: make([]byte, 32)},
				{Tag: "key", Content: []waBinary.Node{}}, // empty key node causes error
			},
		}
		_, err := nodeToPreKeyBundle(1, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid prekey in prekey response")
	})

	t.Run("Invalid signed prekey node inside bundle", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "user",
			Content: []waBinary.Node{
				{Tag: "registration", Content: []byte{1, 2, 3, 4}},
				{Tag: "identity", Content: make([]byte, 32)},
				{Tag: "skey", Content: []waBinary.Node{}},
			},
		}
		_, err := nodeToPreKeyBundle(1, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signed prekey in prekey response")
	})

	t.Run("Bundle without optional preKey (only signedPreKey)", func(t *testing.T) {
		keyPair := keys.NewKeyPair()
		signedPK := keyPair.CreateSignedPreKey(10)
		var spkIDBytes [4]byte
		binary.BigEndian.PutUint32(spkIDBytes[:], signedPK.KeyID)

		node := waBinary.Node{
			Tag: "user",
			Content: []waBinary.Node{
				{Tag: "registration", Content: []byte{0, 0, 1, 0}},
				{Tag: "identity", Content: keyPair.Pub[:]},
				{
					Tag: "skey",
					Content: []waBinary.Node{
						{Tag: "id", Content: spkIDBytes[1:]},
						{Tag: "value", Content: signedPK.Pub[:]},
						{Tag: "signature", Content: signedPK.Signature[:]},
					},
				},
			},
		}
		bundle, err := nodeToPreKeyBundle(1, node)
		require.NoError(t, err)
		assert.NotNil(t, bundle)
		assert.Nil(t, bundle.PreKey())
	})
}
