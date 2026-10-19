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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/libsignal/ecc"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waAdv"
	"github.com/PakaiWA/whatsmeow/proto/waCompanionReg"
	"github.com/PakaiWA/whatsmeow/proto/waWa6"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockCustomContainer struct {
	putDeviceErr    error
	deleteDeviceErr error
	putCalls        int
	deleteCalls     int
}

func (m *mockCustomContainer) PutDevice(_ context.Context, _ *store.Device) error {
	m.putCalls++
	return m.putDeviceErr
}

func (m *mockCustomContainer) DeleteDevice(_ context.Context, _ *store.Device) error {
	m.deleteCalls++
	return m.deleteDeviceErr
}

type mockCustomIdentityStore struct {
	putErr       error
	storedAddrs  []string
	storedKeys   [][32]byte
	trustedAddrs map[string]bool
}

func newMockCustomIdentityStore() *mockCustomIdentityStore {
	return &mockCustomIdentityStore{
		trustedAddrs: make(map[string]bool),
	}
}

func (m *mockCustomIdentityStore) PutIdentity(_ context.Context, address string, key [32]byte) error {
	if m.putErr != nil {
		return m.putErr
	}
	m.storedAddrs = append(m.storedAddrs, address)
	m.storedKeys = append(m.storedKeys, key)
	return nil
}

func (m *mockCustomIdentityStore) IsTrustedIdentity(_ context.Context, address string, _ [32]byte) (bool, error) {
	return m.trustedAddrs[address], nil
}

func (m *mockCustomIdentityStore) SaveIdentity(_ context.Context, _ string, _ [32]byte) (bool, error) {
	return true, nil
}

func (m *mockCustomIdentityStore) DeleteAllIdentities(_ context.Context, _ string) error {
	return nil
}

func (m *mockCustomIdentityStore) DeleteIdentity(_ context.Context, _ string) error {
	return nil
}

// Helper to create a valid signed device identity container for tests
func createTestDeviceIdentityContainer(
	t *testing.T,
	advSecretKey []byte,
	companionIdentity *keys.KeyPair,
	primaryIdentity *keys.KeyPair,
	isHosted bool,
	tamperHMAC bool,
	tamperSig bool,
) []byte {
	t.Helper()

	deviceType := waAdv.ADVEncryptionType_E2EE
	if isHosted {
		deviceType = waAdv.ADVEncryptionType_HOSTED
	}

	details := &waAdv.ADVDeviceIdentity{
		RawID:      proto.Uint32(1),
		Timestamp:  proto.Uint64(uint64(time.Now().Unix())),
		KeyIndex:   proto.Uint32(0),
		DeviceType: &deviceType,
	}
	detailsBytes, err := proto.Marshal(details)
	require.NoError(t, err)

	prefix := AdvAccountSignaturePrefix
	if isHosted {
		prefix = AdvHostedAccountSignaturePrefix
	}
	msgToSign := concatBytes(prefix, detailsBytes, companionIdentity.Pub[:])
	accountSig := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*primaryIdentity.Priv), msgToSign)

	if tamperSig {
		accountSig[0] ^= 0xFF
	}

	signedIdentity := &waAdv.ADVSignedDeviceIdentity{
		Details:             detailsBytes,
		AccountSignatureKey: primaryIdentity.Pub[:],
		AccountSignature:    accountSig[:],
	}
	signedIdentityBytes, err := proto.Marshal(signedIdentity)
	require.NoError(t, err)

	h := hmac.New(sha256.New, advSecretKey)
	if isHosted {
		h.Write(AdvHostedAccountSignaturePrefix)
	}
	h.Write(signedIdentityBytes)
	mac := h.Sum(nil)

	if tamperHMAC {
		mac[0] ^= 0xFF
	}

	container := &waAdv.ADVSignedDeviceIdentityHMAC{
		Details:     signedIdentityBytes,
		HMAC:        mac,
		AccountType: &deviceType,
	}
	containerBytes, err := proto.Marshal(container)
	require.NoError(t, err)

	return containerBytes
}

func TestPair_VerifyAccountSignature_And_GenerateDeviceSignature(t *testing.T) {
	companionKeys := keys.NewKeyPair()
	primaryKeys := keys.NewKeyPair()

	t.Run("Valid E2EE signature", func(t *testing.T) {
		details := []byte("test-device-identity-details")
		msg := concatBytes(AdvAccountSignaturePrefix, details, companionKeys.Pub[:])
		sig := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*primaryKeys.Priv), msg)

		devIdentity := &waAdv.ADVSignedDeviceIdentity{
			Details:             details,
			AccountSignatureKey: primaryKeys.Pub[:],
			AccountSignature:    sig[:],
		}

		assert.True(t, verifyAccountSignature(devIdentity, companionKeys, false))
		assert.False(t, verifyAccountSignature(devIdentity, companionKeys, true)) // wrong isHosted flag
	})

	t.Run("Valid Hosted signature", func(t *testing.T) {
		details := []byte("test-device-identity-details-hosted")
		msg := concatBytes(AdvHostedAccountSignaturePrefix, details, companionKeys.Pub[:])
		sig := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*primaryKeys.Priv), msg)

		devIdentity := &waAdv.ADVSignedDeviceIdentity{
			Details:             details,
			AccountSignatureKey: primaryKeys.Pub[:],
			AccountSignature:    sig[:],
		}

		assert.True(t, verifyAccountSignature(devIdentity, companionKeys, true))
		assert.False(t, verifyAccountSignature(devIdentity, companionKeys, false))
	})

	t.Run("Invalid signature key length", func(t *testing.T) {
		devIdentity := &waAdv.ADVSignedDeviceIdentity{
			Details:             []byte("details"),
			AccountSignatureKey: []byte("short"),
			AccountSignature:    make([]byte, 64),
		}
		assert.False(t, verifyAccountSignature(devIdentity, companionKeys, false))
	})

	t.Run("Invalid signature length", func(t *testing.T) {
		devIdentity := &waAdv.ADVSignedDeviceIdentity{
			Details:             []byte("details"),
			AccountSignatureKey: make([]byte, 32),
			AccountSignature:    []byte("short"),
		}
		assert.False(t, verifyAccountSignature(devIdentity, companionKeys, false))
	})

	t.Run("GenerateDeviceSignature produces valid signature", func(t *testing.T) {
		details := []byte("test-device-identity-details")
		devIdentity := &waAdv.ADVSignedDeviceIdentity{
			Details:             details,
			AccountSignatureKey: primaryKeys.Pub[:],
		}

		devSig := generateDeviceSignature(devIdentity, companionKeys)
		require.NotNil(t, devSig)

		expectedMsg := concatBytes(AdvDeviceSignaturePrefix, details, companionKeys.Pub[:], primaryKeys.Pub[:])
		verified := ecc.VerifySignature(
			ecc.NewDjbECPublicKey(*companionKeys.Pub),
			expectedMsg,
			*devSig,
		)
		assert.True(t, verified)
	})
}

func TestPair_GetQRClientType(t *testing.T) {
	t.Run("Explicit QRClientType takes precedence", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.QRClientType = PairClientFirefox
		assert.Equal(t, PairClientFirefox, cli.getQRClientType())
	})

	t.Run("DeviceProps platform type mapping", func(t *testing.T) {
		origProps := store.DeviceProps
		defer func() { store.DeviceProps = origProps }()

		testCases := []struct {
			platform waCompanionReg.DeviceProps_PlatformType
			expected PairClientType
		}{
			{waCompanionReg.DeviceProps_CHROME, PairClientChrome},
			{waCompanionReg.DeviceProps_FIREFOX, PairClientFirefox},
			{waCompanionReg.DeviceProps_EDGE, PairClientEdge},
			{waCompanionReg.DeviceProps_IE, PairClientIE},
			{waCompanionReg.DeviceProps_OPERA, PairClientOpera},
			{waCompanionReg.DeviceProps_SAFARI, PairClientSafari},
			{waCompanionReg.DeviceProps_UWP, PairClientUWP},
			{waCompanionReg.DeviceProps_ANDROID_PHONE, PairClientAndroid},
		}

		for _, tc := range testCases {
			p := tc.platform
			store.DeviceProps = &waCompanionReg.DeviceProps{
				PlatformType: &p,
			}
			cli := NewClient(&store.Device{}, waLog.Noop)
			assert.Equal(t, tc.expected, cli.getQRClientType())
		}
	})

	t.Run("Fallback to BaseClientPayload UserAgent platform", func(t *testing.T) {
		origProps := store.DeviceProps
		origPayload := store.BaseClientPayload
		defer func() {
			store.DeviceProps = origProps
			store.BaseClientPayload = origPayload
		}()

		unknownPlatform := waCompanionReg.DeviceProps_PlatformType(999)
		store.DeviceProps = &waCompanionReg.DeviceProps{
			PlatformType: &unknownPlatform,
		}

		cli := NewClient(&store.Device{}, waLog.Noop)

		// Test Web
		webPlatform := waWa6.ClientPayload_UserAgent_WEB
		store.BaseClientPayload = &waWa6.ClientPayload{
			UserAgent: &waWa6.ClientPayload_UserAgent{
				Platform: &webPlatform,
			},
		}
		assert.Equal(t, PairClientOtherWebClient, cli.getQRClientType())

		// Test MacOS
		macPlatform := waWa6.ClientPayload_UserAgent_MACOS
		store.BaseClientPayload = &waWa6.ClientPayload{
			UserAgent: &waWa6.ClientPayload_UserAgent{
				Platform: &macPlatform,
			},
		}
		assert.Equal(t, PairClientMacOS, cli.getQRClientType())

		// Test Unknown fallback
		otherPlatform := waWa6.ClientPayload_UserAgent_Platform(999)
		store.BaseClientPayload = &waWa6.ClientPayload{
			UserAgent: &waWa6.ClientPayload_UserAgent{
				Platform: &otherPlatform,
			},
		}
		assert.Equal(t, PairClientUnknown, cli.getQRClientType())
	})
}

func TestPair_RotateADVSecret_And_MakeQRData(t *testing.T) {
	ctx := context.Background()
	dev := &store.Device{
		AdvSecretKey: []byte("old-secret-key-1234567890123456"),
		NoiseKey:     keys.NewKeyPair(),
		IdentityKey:  keys.NewKeyPair(),
	}
	cli := NewClient(dev, waLog.Noop)

	var rotatedEvt *events.RotateADVSecret
	cli.AddEventHandler(func(evt any) {
		if e, ok := evt.(*events.RotateADVSecret); ok {
			rotatedEvt = e
		}
	})

	cli.rotateADVSecret(ctx)
	require.NotNil(t, rotatedEvt)
	assert.NotEmpty(t, rotatedEvt.OldSecret)
	assert.NotEmpty(t, rotatedEvt.NewSecret)
	assert.NotEqual(t, rotatedEvt.OldSecret, rotatedEvt.NewSecret)
	assert.Len(t, cli.Store.AdvSecretKey, 32)

	qr := cli.makeQRData([]byte("test-ref"), PairClientChrome)
	assert.Contains(t, qr, "https://wa.me/settings/linked_devices#test-ref,")
	assert.Contains(t, qr, ","+string(PairClientChrome))
}

func TestPair_HandleIQ(t *testing.T) {
	ctx := context.Background()

	t.Run("Ignore node from non-server", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		called := false
		cli.AddEventHandler(func(_ any) { called = true })

		node := &waBinary.Node{
			Tag: "iq",
			Attrs: waBinary.Attrs{
				"from": types.NewJID("123", types.DefaultUserServer),
			},
			Content: []waBinary.Node{{Tag: "pair-device"}},
		}
		cli.handleIQ(ctx, node)
		assert.False(t, called)
	})

	t.Run("Ignore node with children count != 1", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		called := false
		cli.AddEventHandler(func(_ any) { called = true })

		node := &waBinary.Node{
			Tag: "iq",
			Attrs: waBinary.Attrs{
				"from": types.ServerJID,
			},
			Content: []waBinary.Node{
				{Tag: "pair-device"},
				{Tag: "pair-device"},
			},
		}
		cli.handleIQ(ctx, node)
		assert.False(t, called)
	})

	t.Run("Route to handlePairDevice", func(t *testing.T) {
		dev := &store.Device{
			NoiseKey:     keys.NewKeyPair(),
			IdentityKey:  keys.NewKeyPair(),
			AdvSecretKey: make([]byte, 32),
		}
		cli := NewClient(dev, waLog.Noop)

		var receivedQR *events.QR
		cli.AddEventHandler(func(evt any) {
			if q, ok := evt.(*events.QR); ok {
				receivedQR = q
			}
		})

		node := &waBinary.Node{
			Tag: "iq",
			Attrs: waBinary.Attrs{
				"from": types.ServerJID,
				"id":   "iq-pair-1",
			},
			Content: []waBinary.Node{{
				Tag: "pair-device",
				Content: []waBinary.Node{
					{Tag: "ref", Content: []byte("ref-1")},
					{Tag: "invalid_tag", Content: []byte("ref-invalid")},
					{Tag: "ref", Content: "not-bytes"},
					{Tag: "ref", Content: []byte("ref-2")},
				},
			}},
		}

		cli.handleIQ(ctx, node)
		require.NotNil(t, receivedQR)
		require.Len(t, receivedQR.Codes, 2)
		assert.Contains(t, receivedQR.Codes[0], "ref-1")
		assert.Contains(t, receivedQR.Codes[1], "ref-2")
	})
}

func TestPair_SendPairError(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	// sendPairError should not panic and execute cleanly over socket
	cli.sendPairError(ctx, "err-id-123", 401, "unauthorized")
}

func TestPair_HandlePair_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	companionKeys := keys.NewKeyPair()
	primaryKeys := keys.NewKeyPair()
	advSecretKey := make([]byte, 32)
	jid := types.NewJID("628123456", types.DefaultUserServer)
	lid := types.NewJID("1001", types.HiddenUserServer)

	setupClient := func() *Client {
		dev := &store.Device{
			NoiseKey:     keys.NewKeyPair(),
			IdentityKey:  companionKeys,
			AdvSecretKey: advSecretKey,
		}
		return NewClient(dev, waLog.Noop)
	}

	t.Run("Invalid container protobuf", func(t *testing.T) {
		cli := setupClient()
		err := cli.handlePair(ctx, []byte("invalid protobuf"), "req-1", "Biz", "Chrome", jid, lid)
		var protoErr *PairProtoError
		assert.ErrorAs(t, err, &protoErr)
		assert.Contains(t, err.Error(), "failed to parse device identity container")
	})

	t.Run("HMAC mismatch", func(t *testing.T) {
		cli := setupClient()
		tampered := createTestDeviceIdentityContainer(t, advSecretKey, companionKeys, primaryKeys, false, true, false)
		err := cli.handlePair(ctx, tampered, "req-2", "Biz", "Chrome", jid, lid)
		assert.ErrorIs(t, err, ErrPairInvalidDeviceIdentityHMAC)
	})

	t.Run("Signed device identity invalid protobuf", func(t *testing.T) {
		cli := setupClient()
		container := &waAdv.ADVSignedDeviceIdentityHMAC{
			Details: []byte("not-a-proto"),
		}
		h := hmac.New(sha256.New, advSecretKey)
		h.Write(container.Details)
		container.HMAC = h.Sum(nil)
		raw, err := proto.Marshal(container)
		require.NoError(t, err)

		err = cli.handlePair(ctx, raw, "req-3", "Biz", "Chrome", jid, lid)
		var protoErr *PairProtoError
		assert.ErrorAs(t, err, &protoErr)
		assert.Contains(t, err.Error(), "failed to parse signed device identity")
	})

	t.Run("Device identity details invalid protobuf", func(t *testing.T) {
		cli := setupClient()
		signedIdentity := &waAdv.ADVSignedDeviceIdentity{
			Details:             []byte("corrupt-details"),
			AccountSignatureKey: primaryKeys.Pub[:],
			AccountSignature:    make([]byte, 64),
		}
		signedBytes, err := proto.Marshal(signedIdentity)
		require.NoError(t, err)

		h := hmac.New(sha256.New, advSecretKey)
		h.Write(signedBytes)
		container := &waAdv.ADVSignedDeviceIdentityHMAC{
			Details: signedBytes,
			HMAC:    h.Sum(nil),
		}
		raw, err := proto.Marshal(container)
		require.NoError(t, err)

		err = cli.handlePair(ctx, raw, "req-4", "Biz", "Chrome", jid, lid)
		var protoErr *PairProtoError
		assert.ErrorAs(t, err, &protoErr)
		assert.Contains(t, err.Error(), "failed to parse device identity details")
	})

	t.Run("Signature mismatch", func(t *testing.T) {
		cli := setupClient()
		tamperedSig := createTestDeviceIdentityContainer(t, advSecretKey, companionKeys, primaryKeys, false, false, true)
		err := cli.handlePair(ctx, tamperedSig, "req-5", "Biz", "Chrome", jid, lid)
		assert.ErrorIs(t, err, ErrPairInvalidDeviceSignature)
	})

	t.Run("PrePairCallback rejects pairing", func(t *testing.T) {
		cli := setupClient()
		validBytes := createTestDeviceIdentityContainer(t, advSecretKey, companionKeys, primaryKeys, false, false, false)
		cli.PrePairCallback = func(cbJID types.JID, cbPlatform, cbBiz string) bool {
			return false
		}
		err := cli.handlePair(ctx, validBytes, "req-6", "Biz", "Chrome", jid, lid)
		assert.ErrorIs(t, err, ErrPairRejectedLocally)
	})
}

func TestPair_HandlePair_DatabaseAndNetworkErrors(t *testing.T) {
	ctx := context.Background()
	companionKeys := keys.NewKeyPair()
	primaryKeys := keys.NewKeyPair()
	advSecretKey := make([]byte, 32)
	jid := types.NewJID("628123456", types.DefaultUserServer)
	lid := types.NewJID("1001", types.HiddenUserServer)

	validBytes := createTestDeviceIdentityContainer(t, advSecretKey, companionKeys, primaryKeys, false, false, false)

	t.Run("Device Store.Save error", func(t *testing.T) {
		mockContainer := &mockCustomContainer{
			putDeviceErr: errors.New("db save failure"),
		}
		dev := &store.Device{
			NoiseKey:     keys.NewKeyPair(),
			IdentityKey:  companionKeys,
			AdvSecretKey: advSecretKey,
			Container:    mockContainer,
		}
		cli := NewClient(dev, waLog.Noop)

		err := cli.handlePair(ctx, validBytes, "req-save-err", "Biz", "Chrome", jid, lid)
		var dbErr *PairDatabaseError
		assert.ErrorAs(t, err, &dbErr)
		assert.Contains(t, err.Error(), "failed to save device store")
	})

	t.Run("Identities.PutIdentity error triggers Store.Delete", func(t *testing.T) {
		mockContainer := &mockCustomContainer{}
		mockIdentities := newMockCustomIdentityStore()
		mockIdentities.putErr = errors.New("identity store failure")

		dev := &store.Device{
			NoiseKey:     keys.NewKeyPair(),
			IdentityKey:  companionKeys,
			AdvSecretKey: advSecretKey,
			Container:    mockContainer,
			Identities:   mockIdentities,
			LIDs:         newMockLIDStore(),
		}
		cli := NewClient(dev, waLog.Noop)

		err := cli.handlePair(ctx, validBytes, "req-ident-err", "Biz", "Chrome", jid, lid)
		var dbErr *PairDatabaseError
		assert.ErrorAs(t, err, &dbErr)
		assert.Contains(t, err.Error(), "failed to store main device identity")
		assert.Equal(t, 1, mockContainer.deleteCalls)
		assert.True(t, dev.Deleted)
	})

	t.Run("Send confirmation failure triggers Store.Delete", func(t *testing.T) {
		mockContainer := &mockCustomContainer{}
		mockIdentities := newMockCustomIdentityStore()

		dev := &store.Device{
			NoiseKey:     keys.NewKeyPair(),
			IdentityKey:  companionKeys,
			AdvSecretKey: advSecretKey,
			Container:    mockContainer,
			Identities:   mockIdentities,
			LIDs:         newMockLIDStore(),
		}
		cli := NewClient(dev, waLog.Noop)
		// socket is nil, so sendNode will return ErrNotConnected
		err := cli.handlePair(ctx, validBytes, "req-send-err", "Biz", "Chrome", jid, lid)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send pairing confirmation")
		assert.Equal(t, 1, mockContainer.deleteCalls)
		assert.True(t, dev.Deleted)
	})
}

func TestPair_HandlePair_Success_E2EE_And_Hosted(t *testing.T) {
	ctx := context.Background()

	runSuccessTest := func(t *testing.T, isHosted bool) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		companionKeys := keys.NewKeyPair()
		primaryKeys := keys.NewKeyPair()
		advSecretKey := make([]byte, 32)
		jid := types.NewJID("628123456", types.DefaultUserServer)
		lid := types.NewJID("1001", types.HiddenUserServer)

		mockContainer := &mockCustomContainer{}
		mockIdentities := newMockCustomIdentityStore()
		mockLIDs := newMockLIDStore()

		cli.Store.IdentityKey = companionKeys
		cli.Store.AdvSecretKey = advSecretKey
		cli.Store.Container = mockContainer
		cli.Store.Identities = mockIdentities
		cli.Store.LIDs = mockLIDs

		validBytes := createTestDeviceIdentityContainer(t, advSecretKey, companionKeys, primaryKeys, isHosted, false, false)

		err := cli.handlePair(ctx, validBytes, "req-success", "MyBusiness", "Chrome", jid, lid)
		require.NoError(t, err)

		// Assertions on saved store state
		assert.True(t, cli.paired.Load())
		assert.Equal(t, &jid, cli.Store.ID)
		assert.Equal(t, lid, cli.Store.LID)
		assert.Equal(t, "MyBusiness", cli.Store.BusinessName)
		assert.Equal(t, "Chrome", cli.Store.Platform)
		assert.NotNil(t, cli.Store.Account)
		assert.NotEmpty(t, cli.Store.Account.DeviceSignature)

		// Assert main device identity stored
		require.Len(t, mockIdentities.storedAddrs, 1)
		mainLID := lid
		mainLID.Device = 0
		assert.Equal(t, mainLID.SignalAddress().String(), mockIdentities.storedAddrs[0])
		assert.Equal(t, *primaryKeys.Pub, mockIdentities.storedKeys[0])

		// Assert LID mapping stored
		mappedLID, err := mockLIDs.GetLIDForPN(ctx, jid)
		require.NoError(t, err)
		assert.Equal(t, lid, mappedLID)
	}

	t.Run("Standard E2EE Pairing Success", func(t *testing.T) {
		runSuccessTest(t, false)
	})

	t.Run("Hosted Pairing Success", func(t *testing.T) {
		runSuccessTest(t, true)
	})
}

func TestPair_HandlePairSuccess_EventDispatching(t *testing.T) {
	ctx := context.Background()

	t.Run("Pair error event dispatched on handlePair failure", func(t *testing.T) {
		dev := &store.Device{
			NoiseKey:     keys.NewKeyPair(),
			IdentityKey:  keys.NewKeyPair(),
			AdvSecretKey: make([]byte, 32),
		}
		cli := NewClient(dev, waLog.Noop)

		errChan := make(chan *events.PairError, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.PairError); ok {
				errChan <- e
			}
		})

		node := &waBinary.Node{
			Tag: "iq",
			Attrs: waBinary.Attrs{
				"id":   "req-pair-err",
				"from": types.ServerJID,
				"t":    time.Now().Unix(),
			},
			Content: []waBinary.Node{{
				Tag: "pair-success",
				Content: []waBinary.Node{
					{Tag: "device-identity", Content: []byte("invalid-identity")},
					{Tag: "biz", Attrs: waBinary.Attrs{"name": "BizName"}},
					{Tag: "device", Attrs: waBinary.Attrs{
						"jid": types.NewJID("62812345", types.DefaultUserServer),
						"lid": types.NewJID("1001", types.HiddenUserServer),
					}},
					{Tag: "platform", Attrs: waBinary.Attrs{"name": "Chrome"}},
					{Tag: "client-props", Content: []byte{}},
				},
			}},
		}

		cli.handlePairSuccess(ctx, node)

		select {
		case errEvt := <-errChan:
			assert.Equal(t, "BizName", errEvt.BusinessName)
			assert.Equal(t, "Chrome", errEvt.Platform)
			assert.NotNil(t, errEvt.Error)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for PairError event")
		}
	})

	t.Run("Pair success event dispatched on handlePair success", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		companionKeys := keys.NewKeyPair()
		primaryKeys := keys.NewKeyPair()
		advSecretKey := make([]byte, 32)
		jid := types.NewJID("628123456", types.DefaultUserServer)
		lid := types.NewJID("1001", types.HiddenUserServer)

		mockContainer := &mockCustomContainer{}
		mockIdentities := newMockCustomIdentityStore()
		mockLIDs := newMockLIDStore()

		cli.Store.IdentityKey = companionKeys
		cli.Store.AdvSecretKey = advSecretKey
		cli.Store.Container = mockContainer
		cli.Store.Identities = mockIdentities
		cli.Store.LIDs = mockLIDs

		validContainer := createTestDeviceIdentityContainer(t, advSecretKey, companionKeys, primaryKeys, false, false, false)

		props := &waCompanionReg.ClientPairingProps{
			IsChatDbLidMigrated: proto.Bool(true),
		}
		propsBytes, err := proto.Marshal(props)
		require.NoError(t, err)

		successChan := make(chan *events.PairSuccess, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.PairSuccess); ok {
				successChan <- e
			}
		})

		node := &waBinary.Node{
			Tag: "iq",
			Attrs: waBinary.Attrs{
				"id":   "req-pair-success",
				"from": types.ServerJID,
				"t":    time.Now().Unix(),
			},
			Content: []waBinary.Node{{
				Tag: "pair-success",
				Content: []waBinary.Node{
					{Tag: "device-identity", Content: validContainer},
					{Tag: "biz", Attrs: waBinary.Attrs{"name": "Company"}},
					{Tag: "device", Attrs: waBinary.Attrs{
						"jid": jid,
						"lid": lid,
					}},
					{Tag: "platform", Attrs: waBinary.Attrs{"name": "Chrome (Linux)"}},
					{Tag: "client-props", Content: propsBytes},
				},
			}},
		}

		cli.handlePairSuccess(ctx, node)

		select {
		case s := <-successChan:
			assert.Equal(t, jid, s.ID)
			assert.Equal(t, lid, s.LID)
			assert.Equal(t, "Company", s.BusinessName)
			assert.Equal(t, "Chrome (Linux)", s.Platform)
			assert.NotNil(t, s.Props)
			assert.True(t, s.Props.GetIsChatDbLidMigrated())
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for PairSuccess event")
		}
	})
}
