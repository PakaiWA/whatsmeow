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

	"github.com/PakaiWA/whatsmeow/types"
)

type mockContainer struct {
	DeviceContainer
	putDeviceFunc    func(ctx context.Context, store *Device) error
	deleteDeviceFunc func(ctx context.Context, store *Device) error
}

func (m *mockContainer) PutDevice(ctx context.Context, store *Device) error {
	if m.putDeviceFunc != nil {
		return m.putDeviceFunc(ctx, store)
	}
	return nil
}

func (m *mockContainer) DeleteDevice(ctx context.Context, store *Device) error {
	if m.deleteDeviceFunc != nil {
		return m.deleteDeviceFunc(ctx, store)
	}
	return nil
}

type mockLIDStore struct {
	LIDStore
	getLIDForPNFunc func(ctx context.Context, pn types.JID) (types.JID, error)
	getPNForLIDFunc func(ctx context.Context, lid types.JID) (types.JID, error)
}

func (m *mockLIDStore) GetLIDForPN(ctx context.Context, pn types.JID) (types.JID, error) {
	if m.getLIDForPNFunc != nil {
		return m.getLIDForPNFunc(ctx, pn)
	}
	return types.EmptyJID, nil
}

func (m *mockLIDStore) GetPNForLID(ctx context.Context, lid types.JID) (types.JID, error) {
	if m.getPNForLIDFunc != nil {
		return m.getPNForLIDFunc(ctx, lid)
	}
	return types.EmptyJID, nil
}

func TestStore_MassInsertValues(t *testing.T) {
	jid := types.NewJID("6281234", types.DefaultUserServer)

	ce := ContactEntry{
		JID:       jid,
		FirstName: "John",
		FullName:  "John Doe",
	}
	assert.Equal(t, [...]any{jid.String(), "John", "John Doe"}, ce.GetMassInsertValues())

	rpe := RedactedPhoneEntry{
		JID:           jid,
		RedactedPhone: "+62812***",
	}
	assert.Equal(t, [...]any{jid.String(), "+62812***"}, rpe.GetMassInsertValues())

	lid := types.NewJID("10001", types.HiddenUserServer)
	lm := LIDMapping{
		LID: lid,
		PN:  jid,
	}
	assert.Equal(t, [...]any{"10001", "6281234"}, lm.GetMassInsertValues())
}

func TestDevice_GetJID_And_GetLID(t *testing.T) {
	var nilDevice *Device
	assert.Equal(t, types.EmptyJID, nilDevice.GetJID())
	assert.Equal(t, types.EmptyJID, nilDevice.GetLID())

	d := &Device{}
	assert.Equal(t, types.EmptyJID, d.GetJID())
	assert.Equal(t, types.EmptyJID, d.GetLID())

	jid := types.NewJID("6281234", types.DefaultUserServer)
	lid := types.NewJID("10001", types.HiddenUserServer)
	d.ID = &jid
	d.LID = lid

	assert.Equal(t, jid, d.GetJID())
	assert.Equal(t, lid, d.GetLID())
}

func TestDevice_Save_And_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("Save deleted device returns error", func(t *testing.T) {
		d := &Device{Deleted: true}
		err := d.Save(ctx)
		assert.ErrorIs(t, err, ErrDeviceDeleted)
	})

	t.Run("Save normal device delegates to Container", func(t *testing.T) {
		saved := false
		d := &Device{
			Container: &mockContainer{
				putDeviceFunc: func(_ context.Context, _ *Device) error {
					saved = true
					return nil
				},
			},
		}
		err := d.Save(ctx)
		require.NoError(t, err)
		assert.True(t, saved)
	})

	t.Run("Delete already deleted device is no-op", func(t *testing.T) {
		d := &Device{Deleted: true}
		err := d.Delete(ctx)
		require.NoError(t, err)
	})

	t.Run("Delete error returns error", func(t *testing.T) {
		delErr := errors.New("delete failed")
		d := &Device{
			Container: &mockContainer{
				deleteDeviceFunc: func(_ context.Context, _ *Device) error {
					return delErr
				},
			},
		}
		err := d.Delete(ctx)
		assert.ErrorIs(t, err, delErr)
	})

	t.Run("Delete success resets fields and sets NoopStore", func(t *testing.T) {
		jid := types.NewJID("6281234", types.DefaultUserServer)
		lid := types.NewJID("10001", types.HiddenUserServer)
		d := &Device{
			ID:  &jid,
			LID: lid,
			Container: &mockContainer{
				deleteDeviceFunc: func(_ context.Context, _ *Device) error {
					return nil
				},
			},
		}

		err := d.Delete(ctx)
		require.NoError(t, err)
		assert.True(t, d.Deleted)
		assert.Nil(t, d.ID)
		assert.Equal(t, types.EmptyJID, d.LID)
		assert.NotNil(t, d.Identities)
	})
}

func TestDevice_SetAllStores(t *testing.T) {
	d := &Device{}
	noop := &NoopStore{}
	d.SetAllStores(noop)

	assert.Equal(t, noop, d.Identities)
	assert.Equal(t, noop, d.Sessions)
	assert.Equal(t, noop, d.PreKeys)
	assert.Equal(t, noop, d.SenderKeys)
	assert.Equal(t, noop, d.AppStateKeys)
	assert.Equal(t, noop, d.AppState)
	assert.Equal(t, noop, d.Contacts)
	assert.Equal(t, noop, d.ChatSettings)
	assert.Equal(t, noop, d.MsgSecrets)
	assert.Equal(t, noop, d.PrivacyTokens)
	assert.Equal(t, noop, d.NCTSalt)
	assert.Equal(t, noop, d.EventBuffer)
}

func TestDevice_GetAltJID(t *testing.T) {
	ctx := context.Background()

	var nilDevice *Device
	alt, err := nilDevice.GetAltJID(ctx, types.NewJID("123", types.DefaultUserServer))
	require.NoError(t, err)
	assert.Equal(t, types.EmptyJID, alt)

	pn := types.NewJID("6281234", types.DefaultUserServer)
	lid := types.NewJID("10001", types.HiddenUserServer)
	group := types.NewJID("123-456", types.GroupServer)

	mockLID := &mockLIDStore{
		getLIDForPNFunc: func(_ context.Context, reqPN types.JID) (types.JID, error) {
			assert.Equal(t, pn, reqPN)
			return lid, nil
		},
		getPNForLIDFunc: func(_ context.Context, reqLID types.JID) (types.JID, error) {
			assert.Equal(t, lid, reqLID)
			return pn, nil
		},
	}

	d := &Device{LIDs: mockLID}

	// From PN to LID
	resLID, err := d.GetAltJID(ctx, pn)
	require.NoError(t, err)
	assert.Equal(t, lid, resLID)

	// From LID to PN
	resPN, err := d.GetAltJID(ctx, lid)
	require.NoError(t, err)
	assert.Equal(t, pn, resPN)

	// Other server returns EmptyJID
	resOther, err := d.GetAltJID(ctx, group)
	require.NoError(t, err)
	assert.Equal(t, types.EmptyJID, resOther)
}
