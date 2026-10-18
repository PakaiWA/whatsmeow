// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waHistorySync"
	"github.com/PakaiWA/whatsmeow/proto/waVnameCert"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockContactStore struct {
	pushNames     map[types.JID]string
	businessNames map[types.JID]string
	putPushErr    error
	putBizErr     error
}

func newMockContactStore() *mockContactStore {
	return &mockContactStore{
		pushNames:     make(map[types.JID]string),
		businessNames: make(map[types.JID]string),
	}
}

func (m *mockContactStore) PutPushName(_ context.Context, user types.JID, pushName string) (bool, string, error) {
	if m.putPushErr != nil {
		return false, "", m.putPushErr
	}
	old := m.pushNames[user]
	if old != pushName {
		m.pushNames[user] = pushName
		return true, old, nil
	}
	return false, old, nil
}

func (m *mockContactStore) PutBusinessName(_ context.Context, user types.JID, businessName string) (bool, string, error) {
	if m.putBizErr != nil {
		return false, "", m.putBizErr
	}
	old := m.businessNames[user]
	if old != businessName {
		m.businessNames[user] = businessName
		return true, old, nil
	}
	return false, old, nil
}

func (m *mockContactStore) PutContactName(_ context.Context, _ types.JID, _, _ string) error {
	return nil
}

func (m *mockContactStore) PutAllContactNames(_ context.Context, _ []store.ContactEntry) error {
	return nil
}

func (m *mockContactStore) PutManyRedactedPhones(_ context.Context, _ []store.RedactedPhoneEntry) error {
	return nil
}

func (m *mockContactStore) GetContact(_ context.Context, user types.JID) (types.ContactInfo, error) {
	return types.ContactInfo{PushName: m.pushNames[user], BusinessName: m.businessNames[user]}, nil
}

func (m *mockContactStore) GetAllContacts(_ context.Context) (map[types.JID]types.ContactInfo, error) {
	return nil, nil
}

type mockLIDStore struct {
	pnToLID        map[types.JID]types.JID
	lidToPN        map[types.JID]types.JID
	getLIDErr      error
	getPNErr       error
	putManyErr     error
	lastPutMapping []store.LIDMapping
}

func newMockLIDStore() *mockLIDStore {
	return &mockLIDStore{
		pnToLID: make(map[types.JID]types.JID),
		lidToPN: make(map[types.JID]types.JID),
	}
}

func (m *mockLIDStore) PutManyLIDMappings(_ context.Context, mappings []store.LIDMapping) error {
	if m.putManyErr != nil {
		return m.putManyErr
	}
	m.lastPutMapping = append(m.lastPutMapping, mappings...)
	for _, mapping := range mappings {
		m.pnToLID[mapping.PN] = mapping.LID
		m.lidToPN[mapping.LID] = mapping.PN
	}
	return nil
}

func (m *mockLIDStore) PutLIDMapping(_ context.Context, lid, jid types.JID) error {
	m.pnToLID[jid] = lid
	m.lidToPN[lid] = jid
	return nil
}

func (m *mockLIDStore) GetPNForLID(_ context.Context, lid types.JID) (types.JID, error) {
	if m.getPNErr != nil {
		return types.EmptyJID, m.getPNErr
	}
	return m.lidToPN[lid], nil
}

func (m *mockLIDStore) GetLIDForPN(_ context.Context, pn types.JID) (types.JID, error) {
	if m.getLIDErr != nil {
		return types.EmptyJID, m.getLIDErr
	}
	return m.pnToLID[pn], nil
}

func (m *mockLIDStore) GetManyLIDsForPNs(_ context.Context, pns []types.JID) (map[types.JID]types.JID, error) {
	res := make(map[types.JID]types.JID)
	for _, pn := range pns {
		if lid, ok := m.pnToLID[pn]; ok {
			res[pn] = lid
		}
	}
	return res, nil
}

func newTestUserClient() (*Client, *mockContactStore, *mockLIDStore) {
	contactStore := newMockContactStore()
	lidStore := newMockLIDStore()
	identityKey := keys.NewKeyPair()
	device := &store.Device{
		Log:          waLog.Noop,
		Contacts:     contactStore,
		LIDs:         lidStore,
		IdentityKey:  identityKey,
		SignedPreKey: identityKey.CreateSignedPreKey(1),
	}
	cli := NewClient(device, waLog.Noop)
	return cli, contactStore, lidStore
}

func createVerifiedNameNode(verifiedName string) waBinary.Node {
	details := &waVnameCert.VerifiedNameCertificate_Details{
		VerifiedName: proto.String(verifiedName),
	}
	detailsBytes, _ := proto.Marshal(details)
	cert := &waVnameCert.VerifiedNameCertificate{
		Details: detailsBytes,
	}
	certBytes, _ := proto.Marshal(cert)

	return waBinary.Node{
		Tag: "verified_name",
		Attrs: waBinary.Attrs{
			"verified_level":  "high",
			"v":               "2",
			"host_storage":    "1",
			"actual_actors":   "1",
			"privacy_mode_ts": "1700000000",
		},
		Content: certBytes,
	}
}

// ============================================================
// 1. Helper function tests (stripQuery, stripURLPrefix)
// ============================================================

func TestStripQuery(t *testing.T) {
	assert.Equal(t, "https://wa.me/message/ABCDE", stripQuery("https://wa.me/message/ABCDE?text=hello%20world"))
	assert.Equal(t, "https://wa.me/message/ABCDE", stripQuery("https://wa.me/message/ABCDE"))
	assert.Equal(t, "?queryonly", stripQuery("?queryonly")) // idx == 0, returns original
}

func TestStripURLPrefix(t *testing.T) {
	link1 := "https://wa.me/message/XYZ123?text=hi"
	assert.Equal(t, "XYZ123", stripURLPrefix(link1, BusinessMessageLinkPrefix, BusinessMessageLinkDirectPrefix))

	link2 := "https://api.whatsapp.com/message/XYZ123"
	assert.Equal(t, "XYZ123", stripURLPrefix(link2, BusinessMessageLinkPrefix, BusinessMessageLinkDirectPrefix))

	link3 := "XYZ123"
	assert.Equal(t, "XYZ123", stripURLPrefix(link3, BusinessMessageLinkPrefix, BusinessMessageLinkDirectPrefix))
}

// ============================================================
// 2. Business and Contact QR Link tests
// ============================================================

func TestResolveBusinessMessageLink(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	t.Run("IQ error not found", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotFound
		}
		_, err := cli.ResolveBusinessMessageLink(ctx, "https://wa.me/message/BADCODE")
		assert.ErrorIs(t, err, ErrBusinessMessageLinkNotFound)
	})

	t.Run("Generic IQ error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("network failure")
		}
		_, err := cli.ResolveBusinessMessageLink(ctx, "CODE")
		assert.EqualError(t, err, "network failure")
	})

	t.Run("Missing qr element", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.ResolveBusinessMessageLink(ctx, "CODE")
		var elemErr *ElementMissingError
		assert.True(t, errors.As(err, &elemErr))
		assert.Equal(t, "qr", elemErr.Tag)
	})

	t.Run("Successful resolve with full fields", func(t *testing.T) {
		targetJID := types.NewJID("628123456", types.DefaultUserServer)
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "w:qr", query.Namespace)
			assert.Equal(t, iqGet, query.Type)

			qrChild := waBinary.Node{
				Tag: "qr",
				Attrs: waBinary.Attrs{
					"jid":    targetJID,
					"notify": "PakaiWA Store",
				},
				Content: []waBinary.Node{
					{
						Tag:     "message",
						Content: []byte("Halo mau order"),
					},
					{
						Tag: "business",
						Attrs: waBinary.Attrs{
							"is_signed":      "true",
							"verified_name":  "PakaiWA Official Store",
							"verified_level": "high",
						},
					},
				},
			}
			return &waBinary.Node{
				Tag:     "iq",
				Content: []waBinary.Node{qrChild},
			}, nil
		}

		target, err := cli.ResolveBusinessMessageLink(ctx, "https://wa.me/message/CODE123?text=foo")
		require.NoError(t, err)
		assert.Equal(t, targetJID, target.JID)
		assert.Equal(t, "PakaiWA Store", target.PushName)
		assert.Equal(t, "Halo mau order", target.Message)
		assert.True(t, target.IsSigned)
		assert.Equal(t, "PakaiWA Official Store", target.VerifiedName)
		assert.Equal(t, "high", target.VerifiedLevel)
	})
}

func TestResolveContactQRLink(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	t.Run("IQ error not found", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotFound
		}
		_, err := cli.ResolveContactQRLink(ctx, "https://wa.me/qr/NOTFOUND")
		assert.ErrorIs(t, err, ErrContactQRLinkNotFound)
	})

	t.Run("Missing qr element", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.ResolveContactQRLink(ctx, "CODE")
		var elemErr *ElementMissingError
		assert.True(t, errors.As(err, &elemErr))
	})

	t.Run("Successful resolve", func(t *testing.T) {
		targetJID := types.NewJID("62899999", types.DefaultUserServer)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag: "qr",
						Attrs: waBinary.Attrs{
							"jid":    targetJID,
							"notify": "Alice",
							"type":   "contact",
						},
					},
				},
			}, nil
		}

		target, err := cli.ResolveContactQRLink(ctx, "https://api.whatsapp.com/qr/ALICE")
		require.NoError(t, err)
		assert.Equal(t, targetJID, target.JID)
		assert.Equal(t, "Alice", target.PushName)
		assert.Equal(t, "contact", target.Type)
	})
}

func TestGetContactQRLink(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	t.Run("Get link success", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, iqSet, query.Type)
			qrNode := query.Content.([]waBinary.Node)[0]
			assert.Equal(t, "get", qrNode.Attrs["action"])

			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag:   "qr",
						Attrs: waBinary.Attrs{"code": "QR_CODE_123"},
					},
				},
			}, nil
		}

		code, err := cli.GetContactQRLink(ctx, false)
		require.NoError(t, err)
		assert.Equal(t, "QR_CODE_123", code)
	})

	t.Run("Revoke link success", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			qrNode := query.Content.([]waBinary.Node)[0]
			assert.Equal(t, "revoke", qrNode.Attrs["action"])

			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag:   "qr",
						Attrs: waBinary.Attrs{"code": "QR_CODE_REVOKED"},
					},
				},
			}, nil
		}

		code, err := cli.GetContactQRLink(ctx, true)
		require.NoError(t, err)
		assert.Equal(t, "QR_CODE_REVOKED", code)
	})

	t.Run("Missing qr element error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.GetContactQRLink(ctx, false)
		assert.Error(t, err)
	})
}

// ============================================================
// 3. User Status and IsOnWhatsApp tests
// ============================================================

func TestSetStatusMessage(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
		assert.Equal(t, "w:mex", query.Namespace)
		return &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{
				{
					Tag:     "result",
					Content: []byte(`{"data":{"xwa2_status_update":true}}`),
				},
			},
		}, nil
	}

	err := cli.SetStatusMessage(ctx, types.SetStatusInput{Text: proto.String("Available on PakaiWA")})
	require.NoError(t, err)
}

func TestIsOnWhatsApp(t *testing.T) {
	ctx := context.Background()
	cli, _, lidStore := newTestUserClient()

	userLID := types.NewJID("111111", types.HiddenUserServer)
	userPN := types.NewJID("62812345678", types.DefaultUserServer)

	cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
		assert.Equal(t, "usync", query.Namespace)

		vnameNode := createVerifiedNameNode("Verified Biz")
		userNode := waBinary.Node{
			Tag: "user",
			Attrs: waBinary.Attrs{
				"jid":    userLID,
				"pn_jid": userPN,
			},
			Content: []waBinary.Node{
				{
					Tag: "contact",
					Attrs: waBinary.Attrs{
						"type": "in",
					},
					Content: []byte("+62812345678@" + types.LegacyUserServer),
				},
				{
					Tag:     "business",
					Content: []waBinary.Node{vnameNode},
				},
			},
		}

		listNode := waBinary.Node{
			Tag:     "list",
			Content: []waBinary.Node{userNode},
		}
		return &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{
				{
					Tag:     "usync",
					Content: []waBinary.Node{listNode},
				},
			},
		}, nil
	}

	res, err := cli.IsOnWhatsApp(ctx, []string{"+62812345678"})
	require.NoError(t, err)
	require.Len(t, res, 1)

	assert.Equal(t, userLID, res[0].JID)
	assert.Equal(t, userPN, res[0].PhoneNumber)
	assert.True(t, res[0].IsIn)
	assert.Equal(t, "+62812345678", res[0].Query)
	require.NotNil(t, res[0].VerifiedName)
	assert.Equal(t, "Verified Biz", res[0].VerifiedName.Details.GetVerifiedName())

	// Verified LID mapping stored
	assert.Len(t, lidStore.lastPutMapping, 1)
	assert.Equal(t, userLID, lidStore.lastPutMapping[0].LID)
	assert.Equal(t, userPN, lidStore.lastPutMapping[0].PN)
}

// ============================================================
// 4. GetUserInfo and Bot tests
// ============================================================

func TestGetUserInfo(t *testing.T) {
	ctx := context.Background()
	cli, contactStore, _ := newTestUserClient()

	userJID := types.NewJID("62811111", types.DefaultUserServer)
	userLID := types.NewJID("999999", types.HiddenUserServer)

	cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
		vnameNode := createVerifiedNameNode("Official Store")
		userNode := waBinary.Node{
			Tag: "user",
			Attrs: waBinary.Attrs{
				"jid": userJID,
			},
			Content: []waBinary.Node{
				{Tag: "status", Content: []byte("Hello there!")},
				{Tag: "picture", Attrs: waBinary.Attrs{"id": "pic_123"}},
				{
					Tag: "devices",
					Content: []waBinary.Node{
						{
							Tag: "device-list",
							Content: []waBinary.Node{
								{Tag: "device", Attrs: waBinary.Attrs{"id": "0"}},
								{Tag: "device", Attrs: waBinary.Attrs{"id": "1", "is_hosted": "true"}},
							},
						},
					},
				},
				{Tag: "lid", Attrs: waBinary.Attrs{"val": userLID}},
				{Tag: "business", Content: []waBinary.Node{vnameNode}},
			},
		}

		return &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{
				{
					Tag: "usync",
					Content: []waBinary.Node{
						{Tag: "list", Content: []waBinary.Node{userNode}},
					},
				},
			},
		}, nil
	}

	infoMap, err := cli.GetUserInfo(ctx, []types.JID{userJID})
	require.NoError(t, err)
	require.Contains(t, infoMap, userJID)

	info := infoMap[userJID]
	assert.Equal(t, "Hello there!", info.Status)
	assert.Equal(t, "pic_123", info.PictureID)
	assert.Equal(t, userLID, info.LID)
	require.Len(t, info.Devices, 2)
	assert.Equal(t, uint16(0), info.Devices[0].Device)
	assert.Equal(t, types.HostedServer, info.Devices[1].Server)

	// Business name updated
	assert.Equal(t, "Official Store", contactStore.businessNames[userJID])
}

func TestGetBotListV2(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	botJID := types.NewJID("bot-1", types.DefaultUserServer)

	t.Run("Success fetching bot list", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "bot", query.Namespace)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag: "bot",
						Content: []waBinary.Node{
							{
								Tag:   "section",
								Attrs: waBinary.Attrs{"type": "all"},
								Content: []waBinary.Node{
									{
										Tag: "bot",
										Attrs: waBinary.Attrs{
											"persona_id": "persona_42",
											"jid":        botJID,
										},
									},
								},
							},
						},
					},
				},
			}, nil
		}

		bots, err := cli.GetBotListV2(ctx)
		require.NoError(t, err)
		require.Len(t, bots, 1)
		assert.Equal(t, "persona_42", bots[0].PersonaID)
		assert.Equal(t, botJID, bots[0].BotJID)
	})

	t.Run("Missing bot element", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.GetBotListV2(ctx)
		assert.Error(t, err)
	})
}

func TestGetBotProfiles(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	botJID := types.NewJID("13135550001", types.DefaultUserServer)
	botInfo := []types.BotListInfo{
		{BotJID: botJID, PersonaID: "meta_persona"},
	}

	cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
		userNode := waBinary.Node{
			Tag:   "user",
			Attrs: waBinary.Attrs{"jid": botJID},
			Content: []waBinary.Node{
				{
					Tag: "bot",
					Content: []waBinary.Node{
						{
							Tag:   "profile",
							Attrs: waBinary.Attrs{"persona_id": "meta_persona"},
							Content: []waBinary.Node{
								{Tag: "name", Content: []byte("Meta AI")},
								{Tag: "attributes", Content: []byte("Creative")},
								{Tag: "description", Content: []byte("Assistant")},
								{Tag: "category", Content: []byte("AI")},
								{Tag: "default"},
								{
									Tag: "commands",
									Content: []waBinary.Node{
										{Tag: "description", Content: []byte("Commands list")},
										{
											Tag: "command",
											Content: []waBinary.Node{
												{Tag: "name", Content: []byte("/imagine")},
												{Tag: "description", Content: []byte("Create image")},
											},
										},
									},
								},
								{
									Tag: "prompts",
									Content: []waBinary.Node{
										{
											Tag: "prompt",
											Content: []waBinary.Node{
												{Tag: "emoji", Content: []byte("🎨")},
												{Tag: "text", Content: []byte("Draw a cat")},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		return &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{
				{
					Tag: "usync",
					Content: []waBinary.Node{
						{Tag: "list", Content: []waBinary.Node{userNode}},
					},
				},
			},
		}, nil
	}

	profiles, err := cli.GetBotProfiles(ctx, botInfo)
	require.NoError(t, err)
	require.Len(t, profiles, 1)

	p := profiles[0]
	assert.Equal(t, botJID, p.JID)
	assert.Equal(t, "Meta AI", p.Name)
	assert.Equal(t, "Creative", p.Attributes)
	assert.Equal(t, "Assistant", p.Description)
	assert.Equal(t, "AI", p.Category)
	assert.True(t, p.IsDefault)
	assert.Equal(t, "meta_persona", p.PersonaID)
	assert.Equal(t, "Commands list", p.CommandsDescription)
	require.Len(t, p.Commands, 1)
	assert.Equal(t, "/imagine", p.Commands[0].Name)
	require.Len(t, p.Prompts, 1)
	assert.Equal(t, "🎨 Draw a cat", p.Prompts[0])
}

// ============================================================
// 5. Business Profile parsing and querying
// ============================================================

func TestBusinessProfile(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	bizJID := types.NewJID("62811223344", types.DefaultUserServer)

	profileNode := waBinary.Node{
		Tag: "business_profile",
		Content: []waBinary.Node{
			{
				Tag:   "profile",
				Attrs: waBinary.Attrs{"jid": bizJID},
				Content: []waBinary.Node{
					{Tag: "address", Content: []byte("Jakarta, Indonesia")},
					{Tag: "email", Content: []byte("contact@pakaiwa.com")},
					{
						Tag:   "business_hours",
						Attrs: waBinary.Attrs{"timezone": "Asia/Jakarta"},
						Content: []waBinary.Node{
							{
								Tag: "business_hours_config",
								Attrs: waBinary.Attrs{
									"day_of_week": "mon",
									"mode":        "open_24h",
									"open_time":   "00:00",
									"close_time":  "23:59",
								},
							},
							{Tag: "other_tag_skip"},
						},
					},
					{
						Tag: "categories",
						Content: []waBinary.Node{
							{
								Tag:     "category",
								Attrs:   waBinary.Attrs{"id": "cat_1"},
								Content: []byte("Software Company"),
							},
							{Tag: "skip_category"},
						},
					},
					{
						Tag: "profile_options",
						Content: []waBinary.Node{
							{Tag: "website", Content: []byte("https://pakaiwa.com")},
						},
					},
				},
			},
		},
	}

	t.Run("parseBusinessProfile success", func(t *testing.T) {
		bp, err := cli.parseBusinessProfile(&profileNode)
		require.NoError(t, err)
		assert.Equal(t, bizJID, bp.JID)
		assert.Equal(t, "Jakarta, Indonesia", bp.Address)
		assert.Equal(t, "contact@pakaiwa.com", bp.Email)
		assert.Equal(t, "Asia/Jakarta", bp.BusinessHoursTimeZone)
		require.Len(t, bp.BusinessHours, 1)
		assert.Equal(t, "mon", bp.BusinessHours[0].DayOfWeek)
		require.Len(t, bp.Categories, 1)
		assert.Equal(t, "cat_1", bp.Categories[0].ID)
		assert.Equal(t, "Software Company", bp.Categories[0].Name)
		assert.Equal(t, "https://pakaiwa.com", bp.ProfileOptions["website"])
	})

	t.Run("parseBusinessProfile missing jid", func(t *testing.T) {
		badNode := waBinary.Node{
			Tag: "business_profile",
			Content: []waBinary.Node{
				{Tag: "profile"},
			},
		}
		_, err := cli.parseBusinessProfile(&badNode)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing jid")
	})

	t.Run("GetBusinessProfile success", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "w:biz", query.Namespace)
			return &waBinary.Node{
				Tag:     "iq",
				Content: []waBinary.Node{profileNode},
			}, nil
		}

		bp, err := cli.GetBusinessProfile(ctx, bizJID)
		require.NoError(t, err)
		assert.Equal(t, bizJID, bp.JID)
	})

	t.Run("GetBusinessProfile missing element", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.GetBusinessProfile(ctx, bizJID)
		assert.Error(t, err)
	})
}

// ============================================================
// 6. Device List and FB Devices tests
// ============================================================

func TestGetUserDevices(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	userJID := types.NewJID("6281000", types.DefaultUserServer)
	botJID := types.NewJID("13135550002", types.DefaultUserServer) // Meta AI bot user

	t.Run("Nil client returns error", func(t *testing.T) {
		var nilCli *Client
		_, err := nilCli.GetUserDevices(ctx, []types.JID{userJID})
		assert.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Context wrapper calls GetUserDevices", func(t *testing.T) {
		// Put in cache to test GetUserDevicesContext directly
		cli.userDevicesCache[userJID] = deviceCache{
			devices: []types.JID{userJID},
		}
		devices, err := cli.GetUserDevicesContext(ctx, []types.JID{userJID})
		require.NoError(t, err)
		assert.Equal(t, []types.JID{userJID}, devices)
	})

	t.Run("Bot JID adds directly without network call", func(t *testing.T) {
		devices, err := cli.GetUserDevices(ctx, []types.JID{botJID})
		require.NoError(t, err)
		assert.Equal(t, []types.JID{botJID}, devices)
	})

	t.Run("Messenger server calls getFBIDDevices", func(t *testing.T) {
		fbJID := types.NewJID("fb_user_1", types.MessengerServer)
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "fbid:devices", query.Namespace)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag: "users",
						Content: []waBinary.Node{
							{
								Tag:   "user",
								Attrs: waBinary.Attrs{"jid": fbJID},
								Content: []waBinary.Node{
									{
										Tag:   "devices",
										Attrs: waBinary.Attrs{"dhash": "fbhash"},
										Content: []waBinary.Node{
											{Tag: "device", Attrs: waBinary.Attrs{"id": "0"}},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		}

		devices, err := cli.GetUserDevices(ctx, []types.JID{fbJID})
		require.NoError(t, err)
		require.Len(t, devices, 1)
		assert.Equal(t, fbJID.User, devices[0].User)
	})

	t.Run("Usync devices query", func(t *testing.T) {
		regularJID := types.NewJID("628555", types.DefaultUserServer)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag: "usync",
						Content: []waBinary.Node{
							{
								Tag: "list",
								Content: []waBinary.Node{
									{
										Tag:   "user",
										Attrs: waBinary.Attrs{"jid": regularJID},
										Content: []waBinary.Node{
											{
												Tag: "devices",
												Content: []waBinary.Node{
													{
														Tag: "device-list",
														Content: []waBinary.Node{
															{Tag: "device", Attrs: waBinary.Attrs{"id": "0"}},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		}

		devices, err := cli.GetUserDevices(ctx, []types.JID{regularJID})
		require.NoError(t, err)
		require.Len(t, devices, 1)
		assert.Equal(t, uint16(0), devices[0].Device)
	})
}

// ============================================================
// 7. Profile Picture Info tests
// ============================================================

func TestGetProfilePictureInfo(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	userJID := types.NewJID("62812345", types.DefaultUserServer)
	groupJID := types.NewJID("group123", types.GroupServer)

	t.Run("Nil client error", func(t *testing.T) {
		var nilCli *Client
		_, err := nilCli.GetProfilePictureInfo(ctx, userJID, nil)
		assert.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Unauthorized error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotAuthorized
		}
		_, err := cli.GetProfilePictureInfo(ctx, userJID, nil)
		assert.ErrorIs(t, err, ErrProfilePictureUnauthorized)
	})

	t.Run("Not found error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotFound
		}
		_, err := cli.GetProfilePictureInfo(ctx, userJID, nil)
		assert.ErrorIs(t, err, ErrProfilePictureNotSet)
	})

	t.Run("Status 304 unchanged", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag:   "picture",
						Attrs: waBinary.Attrs{"status": "304"},
					},
				},
			}, nil
		}
		pic, err := cli.GetProfilePictureInfo(ctx, userJID, &GetProfilePictureParams{ExistingID: "pic_1"})
		require.NoError(t, err)
		assert.Nil(t, pic)
	})

	t.Run("Status 204 not set", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag:   "picture",
						Attrs: waBinary.Attrs{"status": "204"},
					},
				},
			}, nil
		}
		_, err := cli.GetProfilePictureInfo(ctx, userJID, nil)
		assert.ErrorIs(t, err, ErrProfilePictureNotSet)
	})

	t.Run("Community picture wrapped", func(t *testing.T) {
		rawHash := []byte("picturehash12345")
		b64Hash := base64.StdEncoding.EncodeToString(rawHash)

		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "w:g2", query.Namespace)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag: "pictures",
						Content: []waBinary.Node{
							{
								Tag: "picture",
								Attrs: waBinary.Attrs{
									"id":          "community_pic_1",
									"url":         "https://mmg.whatsapp.net/comm.jpg",
									"type":        "preview",
									"direct_path": "/comm.jpg",
									"hash":        b64Hash,
								},
							},
						},
					},
				},
			}, nil
		}

		pic, err := cli.GetProfilePictureInfo(ctx, groupJID, &GetProfilePictureParams{
			IsCommunity: true,
			Preview:     true,
		})
		require.NoError(t, err)
		require.NotNil(t, pic)
		assert.Equal(t, "community_pic_1", pic.ID)
		assert.Equal(t, "https://mmg.whatsapp.net/comm.jpg", pic.URL)
		assert.Equal(t, "preview", pic.Type)
		assert.Equal(t, "/comm.jpg", pic.DirectPath)
		assert.Equal(t, rawHash, pic.Hash)
	})

	t.Run("Missing picture element when existing ID is set returns nil", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		pic, err := cli.GetProfilePictureInfo(ctx, userJID, &GetProfilePictureParams{ExistingID: "old_pic"})
		require.NoError(t, err)
		assert.Nil(t, pic)
	})
}

// ============================================================
// 8. Historical and Update Push / Business Names
// ============================================================

func TestPushAndBusinessNames(t *testing.T) {
	ctx := context.Background()
	cli, contactStore, lidStore := newTestUserClient()

	userJID := types.NewJID("6281234", types.DefaultUserServer)
	userLID := types.NewJID("888888", types.HiddenUserServer)
	_ = lidStore.PutLIDMapping(ctx, userLID, userJID)

	t.Run("handleHistoricalPushNames", func(t *testing.T) {
		names := []*waHistorySync.Pushname{
			{ID: proto.String("6281234@s.whatsapp.net"), Pushname: proto.String("Budi")},
			{ID: proto.String("6289999@s.whatsapp.net"), Pushname: proto.String("-")}, // skipped
			{ID: proto.String("invalid-jid"), Pushname: proto.String("Skip")},         // invalid JID
		}
		cli.handleHistoricalPushNames(ctx, names)
		assert.Equal(t, "Budi", contactStore.pushNames[userJID])
	})

	t.Run("updatePushName with alt JID and event dispatch", func(t *testing.T) {
		var receivedEvt *events.PushName
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.PushName); ok {
				receivedEvt = e
			}
		})

		cli.updatePushName(ctx, userJID, types.EmptyJID, nil, "Budi Utomo")
		assert.Equal(t, "Budi Utomo", contactStore.pushNames[userJID])
		assert.Equal(t, "Budi Utomo", contactStore.pushNames[userLID])
		require.NotNil(t, receivedEvt)
		assert.Equal(t, "Budi Utomo", receivedEvt.NewPushName)
		assert.Equal(t, "Budi", receivedEvt.OldPushName)
	})

	t.Run("updateBusinessName with alt JID and event dispatch", func(t *testing.T) {
		var receivedEvt *events.BusinessName
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.BusinessName); ok {
				receivedEvt = e
			}
		})

		cli.updateBusinessName(ctx, userJID, types.EmptyJID, nil, "Budi Store")
		assert.Equal(t, "Budi Store", contactStore.businessNames[userJID])
		assert.Equal(t, "Budi Store", contactStore.businessNames[userLID])
		require.NotNil(t, receivedEvt)
		assert.Equal(t, "Budi Store", receivedEvt.NewBusinessName)
	})

	t.Run("nil contact store does not panic", func(t *testing.T) {
		cliNoStore := &Client{Store: &store.Device{}}
		cliNoStore.handleHistoricalPushNames(ctx, nil)
		cliNoStore.updatePushName(ctx, userJID, types.EmptyJID, nil, "Test")
		cliNoStore.updateBusinessName(ctx, userJID, types.EmptyJID, nil, "Test")
	})
}

// ============================================================
// 9. Blocklist tests
// ============================================================

func TestBlocklist(t *testing.T) {
	ctx := context.Background()
	cli, _, lidStore := newTestUserClient()

	userLID := types.NewJID("block_lid_1", types.HiddenUserServer)
	userPN := types.NewJID("628123000", types.DefaultUserServer)

	blocklistNode := waBinary.Node{
		Tag: "list",
		Attrs: waBinary.Attrs{
			"dhash":           "dhash_123",
			"addressing_mode": "lid",
		},
		Content: []waBinary.Node{
			{
				Tag: "item",
				Attrs: waBinary.Attrs{
					"jid":    userLID,
					"pn_jid": userPN,
					"active": "true",
				},
			},
			{
				Tag: "item",
				Attrs: waBinary.Attrs{
					"jid": "bad-jid-attrs",
				},
			},
		},
	}

	t.Run("GetBlocklist with dhash and without dhash", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "blocklist", query.Namespace)
			assert.Equal(t, iqGet, query.Type)
			return &waBinary.Node{
				Tag:     "iq",
				Content: []waBinary.Node{blocklistNode},
			}, nil
		}

		bl, err := cli.GetBlocklist(ctx, "dhash_123")
		require.NoError(t, err)
		require.NotNil(t, bl)
		assert.Equal(t, "dhash_123", bl.DHash)
		assert.Equal(t, types.AddressingMode("lid"), bl.AddressingMode)
		require.Len(t, bl.Items, 1)
		assert.Equal(t, userLID, bl.Items[0].LID)
		assert.Equal(t, userPN, bl.Items[0].PN)
		assert.True(t, bl.Items[0].Active)

		// Verified LID mapping stored
		assert.Equal(t, userPN, lidStore.lidToPN[userLID])
	})

	t.Run("GetBlocklist dhash match returns nil", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil // no list child
		}
		bl, err := cli.GetBlocklist(ctx, "dhash_matched")
		require.NoError(t, err)
		assert.Nil(t, bl)
	})

	t.Run("UpdateBlocklist block and unblock", func(t *testing.T) {
		// Populate store so GetLIDForPN succeeds
		_ = lidStore.PutLIDMapping(ctx, userLID, userPN)

		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "blocklist", query.Namespace)
			assert.Equal(t, iqSet, query.Type)
			return &waBinary.Node{
				Tag:     "iq",
				Content: []waBinary.Node{blocklistNode},
			}, nil
		}

		// Block PN
		bl, err := cli.UpdateBlocklist(ctx, userPN, events.BlocklistChangeActionBlock, "dhash_old")
		require.NoError(t, err)
		require.NotNil(t, bl)

		// Block LID
		bl2, err := cli.UpdateBlocklist(ctx, userLID, events.BlocklistChangeActionBlock, "")
		require.NoError(t, err)
		require.NotNil(t, bl2)

		// Unblock LID
		bl3, err := cli.UpdateBlocklist(ctx, userLID, events.BlocklistChangeActionUnblock, "")
		require.NoError(t, err)
		require.NotNil(t, bl3)
	})

	t.Run("UpdateBlocklist dhash matched returns nil", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag:   "list",
						Attrs: waBinary.Attrs{"matched": "true"},
					},
				},
			}, nil
		}

		bl, err := cli.UpdateBlocklist(ctx, userLID, events.BlocklistChangeActionBlock, "same_dhash")
		require.NoError(t, err)
		assert.Nil(t, bl)
	})
}

// ============================================================
// 10. Usync parameter validation tests
// ============================================================

func TestUsync(t *testing.T) {
	ctx := context.Background()
	cli, _, _ := newTestUserClient()

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		_, err := nilCli.usync(ctx, nil, "query", "interactive", nil)
		assert.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Multiple extra params error", func(t *testing.T) {
		_, err := cli.usync(ctx, nil, "query", "interactive", nil, UsyncQueryExtras{}, UsyncQueryExtras{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only one extra parameter may be provided")
	})

	t.Run("Unknown user server error", func(t *testing.T) {
		badJID := types.NewJID("123", "unknown.server.net")
		_, err := cli.usync(ctx, []types.JID{badJID}, "query", "interactive", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown user server 'unknown.server.net'")
	})

	t.Run("SendIQ error and missing list child", func(t *testing.T) {
		userJID := types.NewJID("6281234", types.DefaultUserServer)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("send error")
		}
		_, err := cli.usync(ctx, []types.JID{userJID}, "query", "interactive", nil, UsyncQueryExtras{IncludePrivacyToken: true})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send usync query: send error")

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err = cli.usync(ctx, []types.JID{userJID}, "query", "interactive", nil)
		assert.Error(t, err)
		var elemErr *ElementMissingError
		assert.True(t, errors.As(err, &elemErr))
	})
}

func TestParseVerifiedName_EdgeCases(t *testing.T) {
	t.Run("Not business tag", func(t *testing.T) {
		vn, err := parseVerifiedName(waBinary.Node{Tag: "not_business"})
		assert.NoError(t, err)
		assert.Nil(t, vn)
	})

	t.Run("Missing verified_name tag", func(t *testing.T) {
		vn, err := parseVerifiedName(waBinary.Node{Tag: "business"})
		assert.NoError(t, err)
		assert.Nil(t, vn)
	})

	t.Run("Content not byte slice", func(t *testing.T) {
		vn, err := parseVerifiedNameContent(waBinary.Node{
			Tag:     "verified_name",
			Content: "not bytes",
		})
		assert.NoError(t, err)
		assert.Nil(t, vn)
	})

	t.Run("Invalid cert protobuf", func(t *testing.T) {
		_, err := parseVerifiedNameContent(waBinary.Node{
			Tag:     "verified_name",
			Content: []byte{0xff, 0xff},
		})
		assert.Error(t, err)
	})

	t.Run("Invalid details protobuf in cert", func(t *testing.T) {
		cert := &waVnameCert.VerifiedNameCertificate{
			Details: []byte{0xff, 0xff},
		}
		certBytes, _ := proto.Marshal(cert)
		_, err := parseVerifiedNameContent(waBinary.Node{
			Tag:     "verified_name",
			Content: certBytes,
		})
		assert.Error(t, err)
	})
}

func TestUpdateBlocklist_ErrorCases(t *testing.T) {
	ctx := context.Background()
	cli, _, lidStore := newTestUserClient()

	userPN := types.NewJID("6281001", types.DefaultUserServer)
	userLID := types.NewJID("lid_1001", types.HiddenUserServer)

	t.Run("GetLIDForPN error", func(t *testing.T) {
		lidStore.getLIDErr = errors.New("lid store read error")
		defer func() { lidStore.getLIDErr = nil }()

		_, err := cli.UpdateBlocklist(ctx, userPN, events.BlocklistChangeActionBlock, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get LID for PN")
	})

	t.Run("GetUserInfo error when LID is empty", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("usync info error")
		}

		_, err := cli.UpdateBlocklist(ctx, userPN, events.BlocklistChangeActionBlock, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get user info")
	})

	t.Run("No LID found from server after GetUserInfo", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			// Return empty user node without lid
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag: "usync",
						Content: []waBinary.Node{
							{
								Tag: "list",
								Content: []waBinary.Node{
									{Tag: "user", Attrs: waBinary.Attrs{"jid": userPN}},
								},
							},
						},
					},
				},
			}, nil
		}

		_, err := cli.UpdateBlocklist(ctx, userPN, events.BlocklistChangeActionBlock, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no LID found for")
	})

	t.Run("GetPNForLID error for HiddenUserServer on block", func(t *testing.T) {
		lidStore.getPNErr = errors.New("pn lookup error")
		defer func() { lidStore.getPNErr = nil }()

		_, err := cli.UpdateBlocklist(ctx, userLID, events.BlocklistChangeActionBlock, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve PN for")
	})

	t.Run("SendIQ error on blocklist update", func(t *testing.T) {
		_ = lidStore.PutLIDMapping(ctx, userLID, userPN)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("iq send blocklist error")
		}

		_, err := cli.UpdateBlocklist(ctx, userLID, events.BlocklistChangeActionBlock, "")
		assert.Error(t, err)
		assert.EqualError(t, err, "iq send blocklist error")
	})

	t.Run("Missing list child on blocklist update", func(t *testing.T) {
		_ = lidStore.PutLIDMapping(ctx, userLID, userPN)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}

		_, err := cli.UpdateBlocklist(ctx, userLID, events.BlocklistChangeActionBlock, "")
		assert.Error(t, err)
		var elemErr *ElementMissingError
		assert.True(t, errors.As(err, &elemErr))
	})
}

func TestParseDeviceList_EdgeCases(t *testing.T) {
	user := types.NewJID("user1", types.DefaultUserServer)
	userLID := types.NewJID("userlid", types.HiddenUserServer)

	t.Run("Invalid tag returns nil", func(t *testing.T) {
		devs := parseDeviceList(user, waBinary.Node{Tag: "not_devices"})
		assert.Nil(t, devs)
	})

	t.Run("Missing device-list child returns nil", func(t *testing.T) {
		devs := parseDeviceList(user, waBinary.Node{Tag: "devices"})
		assert.Nil(t, devs)
	})

	t.Run("Hosted LID device", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "devices",
			Content: []waBinary.Node{
				{
					Tag: "device-list",
					Content: []waBinary.Node{
						{Tag: "device", Attrs: waBinary.Attrs{"id": "2", "is_hosted": "true"}},
						{Tag: "device", Attrs: waBinary.Attrs{"id": "-1"}},    // invalid id
						{Tag: "device", Attrs: waBinary.Attrs{"id": "70000"}}, // out of range
						{Tag: "not_device"},
					},
				},
			},
		}

		devs := parseDeviceList(userLID, node)
		require.Len(t, devs, 1)
		assert.Equal(t, types.HostedLIDServer, devs[0].Server)
		assert.Equal(t, uint16(2), devs[0].Device)
	})
}
