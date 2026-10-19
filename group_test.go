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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func setupTestGroupClient() (*Client, *mockLIDStore) {
	lidStore := newMockLIDStore()
	contactStore := newMockContactStore()
	device := &store.Device{
		ID:            &types.JID{User: "12345", Server: types.DefaultUserServer},
		IdentityKey:   keys.NewKeyPair(),
		NoiseKey:      keys.NewKeyPair(),
		LIDs:          lidStore,
		Contacts:      contactStore,
		PrivacyTokens: newMockPrivacyTokenStore(),
	}
	cli := NewClient(device, waLog.Noop)
	return cli, lidStore
}

func TestGroup_CreateGroup(t *testing.T) {
	ctx := context.Background()
	groupJID := types.NewJID("12036302@g.us", types.GroupServer)
	user1 := types.NewJID("6281111", types.DefaultUserServer)
	userLID := types.NewJID("100200300", types.HiddenUserServer)
	userPN := types.NewJID("6282222", types.DefaultUserServer)

	t.Run("Create standard group success", func(t *testing.T) {
		cli, lidStore := setupTestGroupClient()
		_ = lidStore.PutLIDMapping(ctx, userLID, userPN)

		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "w:g2", query.Namespace)
			assert.Equal(t, iqSet, query.Type)
			assert.Equal(t, types.GroupServerJID, query.To)

			nodes, _ := query.Content.([]waBinary.Node)
			require.NotEmpty(t, nodes)
			createNode := nodes[0]
			assert.Equal(t, "create", createNode.Tag)
			assert.Equal(t, "My Test Group", createNode.Attrs["subject"])

			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "group",
					Attrs: waBinary.Attrs{
						"id":       "12036302",
						"subject":  "My Test Group",
						"creation": "1700000000",
					},
					Content: []waBinary.Node{
						{Tag: "locked"},
						{Tag: "announcement"},
					},
				}},
			}, nil
		}

		info, err := cli.CreateGroup(ctx, ReqCreateGroup{
			Name:         "My Test Group",
			Participants: []types.JID{user1, userLID},
			GroupEphemeral: types.GroupEphemeral{
				IsEphemeral:       true,
				DisappearingTimer: 86400,
			},
			GroupLocked:   types.GroupLocked{IsLocked: true},
			GroupAnnounce: types.GroupAnnounce{IsAnnounce: true},
			GroupMembershipApprovalMode: types.GroupMembershipApprovalMode{
				IsJoinApprovalRequired: true,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "12036302", info.JID.User)
		assert.Equal(t, "My Test Group", info.Name)
		assert.True(t, info.IsLocked)
		assert.True(t, info.IsAnnounce)
	})

	t.Run("Create parent community and linked parent", func(t *testing.T) {
		cli, _ := setupTestGroupClient()

		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "group",
					Attrs: waBinary.Attrs{
						"id":      "community123",
						"subject": "Community",
					},
					Content: []waBinary.Node{{
						Tag: "parent",
					}},
				}},
			}, nil
		}

		info, err := cli.CreateGroup(ctx, ReqCreateGroup{
			Name: "Community",
			GroupParent: types.GroupParent{
				IsParent: true,
			},
			GroupLinkedParent: types.GroupLinkedParent{
				LinkedParentJID: types.NewJID("parent123", types.GroupServer),
			},
		})
		require.NoError(t, err)
		assert.True(t, info.IsParent)
	})

	t.Run("LID phone number error", func(t *testing.T) {
		cli, lidStore := setupTestGroupClient()
		lidStore.getPNErr = errors.New("db error")

		_, err := cli.CreateGroup(ctx, ReqCreateGroup{
			Name:         "Fail Group",
			Participants: []types.JID{userLID},
		})
		assert.ErrorContains(t, err, "failed to get phone number for participant")
	})

	t.Run("Missing group element in response", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}

		_, err := cli.CreateGroup(ctx, ReqCreateGroup{Name: "Missing Group"})
		assert.Error(t, err)
		var elemErr *ElementMissingError
		assert.True(t, errors.As(err, &elemErr))
	})

	t.Run("sendIQ error", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("network failure")
		}

		_, err := cli.CreateGroup(ctx, ReqCreateGroup{Name: "Fail Group"})
		assert.ErrorContains(t, err, "network failure")
	})
	_ = groupJID
}

func TestGroup_LinkAndUnlink(t *testing.T) {
	ctx := context.Background()
	parent := types.NewJID("parent123", types.GroupServer)
	child := types.NewJID("child456", types.GroupServer)

	t.Run("LinkGroup success", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, parent, query.To)
			return &waBinary.Node{Tag: "iq"}, nil
		}
		err := cli.LinkGroup(ctx, parent, child)
		assert.NoError(t, err)
	})

	t.Run("UnlinkGroup success", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, parent, query.To)
			return &waBinary.Node{Tag: "iq"}, nil
		}
		err := cli.UnlinkGroup(ctx, parent, child)
		assert.NoError(t, err)
	})

	t.Run("LeaveGroup success", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, types.GroupServerJID, query.To)
			return &waBinary.Node{Tag: "iq"}, nil
		}
		err := cli.LeaveGroup(ctx, child)
		assert.NoError(t, err)
	})
}

func TestGroup_UpdateGroupParticipants(t *testing.T) {
	ctx := context.Background()
	groupJID := types.NewJID("group123", types.GroupServer)
	user1 := types.NewJID("62811", types.DefaultUserServer)
	userLID := types.NewJID("userlid", types.HiddenUserServer)
	userPN := types.NewJID("62822", types.DefaultUserServer)

	t.Run("Add participants success", func(t *testing.T) {
		cli, lidStore := setupTestGroupClient()
		_ = lidStore.PutLIDMapping(ctx, userLID, userPN)

		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "add",
					Content: []waBinary.Node{
						{Tag: "participant", Attrs: waBinary.Attrs{"jid": user1}},
						{Tag: "participant", Attrs: waBinary.Attrs{"jid": userLID, "phone_number": userPN}},
					},
				}},
			}, nil
		}

		parts, err := cli.UpdateGroupParticipants(ctx, groupJID, []types.JID{user1, userLID}, ParticipantChangeAdd)
		require.NoError(t, err)
		assert.Len(t, parts, 2)
	})

	t.Run("LID GetPN error on Add", func(t *testing.T) {
		cli, lidStore := setupTestGroupClient()
		lidStore.getPNErr = errors.New("db error")

		_, err := cli.UpdateGroupParticipants(ctx, groupJID, []types.JID{userLID}, ParticipantChangeAdd)
		assert.ErrorContains(t, err, "failed to get phone number for LID")
	})

	t.Run("Missing action in response", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.UpdateGroupParticipants(ctx, groupJID, []types.JID{user1}, ParticipantChangeRemove)
		assert.Error(t, err)
	})
}

func TestGroup_GroupRequestParticipants(t *testing.T) {
	ctx := context.Background()
	groupJID := types.NewJID("group123", types.GroupServer)
	user1 := types.NewJID("62811", types.DefaultUserServer)

	t.Run("GetGroupRequestParticipants success", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "membership_approval_requests",
					Content: []waBinary.Node{{
						Tag:   "membership_approval_request",
						Attrs: waBinary.Attrs{"jid": user1, "request_time": "1700000000"},
					}},
				}},
			}, nil
		}

		reqs, err := cli.GetGroupRequestParticipants(ctx, groupJID)
		require.NoError(t, err)
		require.Len(t, reqs, 1)
		assert.Equal(t, user1, reqs[0].JID)
	})

	t.Run("GetGroupRequestParticipants missing tag", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.GetGroupRequestParticipants(ctx, groupJID)
		assert.Error(t, err)
	})

	t.Run("UpdateGroupRequestParticipants success", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "membership_requests_action",
					Content: []waBinary.Node{{
						Tag: string(ParticipantChangeApprove),
						Content: []waBinary.Node{{
							Tag:   "participant",
							Attrs: waBinary.Attrs{"jid": user1},
						}},
					}},
				}},
			}, nil
		}

		parts, err := cli.UpdateGroupRequestParticipants(ctx, groupJID, []types.JID{user1}, ParticipantChangeApprove)
		require.NoError(t, err)
		require.Len(t, parts, 1)
		assert.Equal(t, user1, parts[0].JID)
	})

	t.Run("UpdateGroupRequestParticipants missing tag", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.UpdateGroupRequestParticipants(ctx, groupJID, []types.JID{user1}, ParticipantChangeReject)
		assert.Error(t, err)
	})
}

func TestGroup_GroupMetadataModifications(t *testing.T) {
	ctx := context.Background()
	groupJID := types.NewJID("group123", types.GroupServer)

	t.Run("SetGroupPhoto success and remove", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:   "picture",
					Attrs: waBinary.Attrs{"id": "pic-id-123"},
				}},
			}, nil
		}

		id, err := cli.SetGroupPhoto(ctx, groupJID, []byte("jpeg-data"))
		require.NoError(t, err)
		assert.Equal(t, "pic-id-123", id)

		// Remove photo
		idRem, errRem := cli.SetGroupPhoto(ctx, groupJID, nil)
		require.NoError(t, errRem)
		assert.Equal(t, "remove", idRem)
	})

	t.Run("SetGroupPhoto ErrIQNotAcceptable wrap", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotAcceptable
		}
		_, err := cli.SetGroupPhoto(ctx, groupJID, []byte("invalid-img"))
		assert.ErrorIs(t, err, ErrInvalidImageFormat)
	})

	t.Run("SetGroupName", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		err := cli.SetGroupName(ctx, groupJID, "New Name")
		assert.NoError(t, err)
	})

	t.Run("SetGroupTopic with empty and delete", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			nodes, _ := query.Content.([]waBinary.Node)
			if len(nodes) > 0 && nodes[0].Tag == "query" {
				return &waBinary.Node{
					Tag: "iq",
					Content: []waBinary.Node{{
						Tag:   "group",
						Attrs: waBinary.Attrs{"id": groupJID.User},
						Content: []waBinary.Node{{
							Tag:   "description",
							Attrs: waBinary.Attrs{"id": "topic-prev-id", "t": "1700000000"},
							Content: []waBinary.Node{{
								Tag:     "body",
								Content: []byte("old topic"),
							}},
						}},
					}},
				}, nil
			}
			return &waBinary.Node{Tag: "iq"}, nil
		}

		err := cli.SetGroupTopic(ctx, groupJID, "", "new-id", "new topic")
		assert.NoError(t, err)

		errDel := cli.SetGroupDescription(ctx, groupJID, "")
		assert.NoError(t, errDel)
	})

	t.Run("SetGroupLocked and SetGroupAnnounce", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}

		assert.NoError(t, cli.SetGroupLocked(ctx, groupJID, true))
		assert.NoError(t, cli.SetGroupLocked(ctx, groupJID, false))
		assert.NoError(t, cli.SetGroupAnnounce(ctx, groupJID, true))
		assert.NoError(t, cli.SetGroupAnnounce(ctx, groupJID, false))
	})

	t.Run("SetGroupJoinApprovalMode and MemberAddMode", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}

		assert.NoError(t, cli.SetGroupJoinApprovalMode(ctx, groupJID, true))
		assert.NoError(t, cli.SetGroupJoinApprovalMode(ctx, groupJID, false))

		assert.NoError(t, cli.SetGroupMemberAddMode(ctx, groupJID, types.GroupMemberAddModeAdmin))
		assert.NoError(t, cli.SetGroupMemberAddMode(ctx, groupJID, types.GroupMemberAddModeAllMember))
		assert.Error(t, cli.SetGroupMemberAddMode(ctx, groupJID, types.GroupMemberAddMode("invalid")))
	})
}

func TestGroup_InviteLinks(t *testing.T) {
	ctx := context.Background()
	groupJID := types.NewJID("group123", types.GroupServer)
	adminJID := types.NewJID("admin1", types.DefaultUserServer)

	t.Run("GetGroupInviteLink success and reset", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:   "invite",
					Attrs: waBinary.Attrs{"code": "INVITECODE123"},
				}},
			}, nil
		}

		link, err := cli.GetGroupInviteLink(ctx, groupJID, false)
		require.NoError(t, err)
		assert.Equal(t, InviteLinkPrefix+"INVITECODE123", link)

		linkReset, errReset := cli.GetGroupInviteLink(ctx, groupJID, true)
		require.NoError(t, errReset)
		assert.Equal(t, InviteLinkPrefix+"INVITECODE123", linkReset)
	})

	t.Run("GetGroupInviteLink errors", func(t *testing.T) {
		cli, _ := setupTestGroupClient()

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotAuthorized
		}
		_, err := cli.GetGroupInviteLink(ctx, groupJID, false)
		assert.ErrorIs(t, err, ErrGroupInviteLinkUnauthorized)

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotFound
		}
		_, err = cli.GetGroupInviteLink(ctx, groupJID, false)
		assert.ErrorIs(t, err, ErrGroupNotFound)

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQForbidden
		}
		_, err = cli.GetGroupInviteLink(ctx, groupJID, false)
		assert.ErrorIs(t, err, ErrNotInGroup)
	})

	t.Run("GetGroupInfoFromInvite & JoinGroupWithInvite", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			if query.Type == iqGet {
				return &waBinary.Node{
					Tag: "iq",
					Content: []waBinary.Node{{
						Tag:   "group",
						Attrs: waBinary.Attrs{"id": groupJID.User, "subject": "Invited Group"},
					}},
				}, nil
			}
			return &waBinary.Node{Tag: "iq"}, nil
		}

		info, err := cli.GetGroupInfoFromInvite(ctx, groupJID, adminJID, "CODE", 1700000000)
		require.NoError(t, err)
		assert.Equal(t, "Invited Group", info.Name)

		errJoin := cli.JoinGroupWithInvite(ctx, groupJID, adminJID, "CODE", 1700000000)
		assert.NoError(t, errJoin)

		// Missing group tag
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, errMissing := cli.GetGroupInfoFromInvite(ctx, groupJID, adminJID, "CODE", 1700000000)
		assert.Error(t, errMissing)
	})

	t.Run("GetGroupInfoFromLink & JoinGroupWithLink", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			if query.Type == iqGet {
				return &waBinary.Node{
					Tag: "iq",
					Content: []waBinary.Node{{
						Tag:   "group",
						Attrs: waBinary.Attrs{"id": groupJID.User, "subject": "Link Group"},
					}},
				}, nil
			}
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:   "group",
					Attrs: waBinary.Attrs{"jid": groupJID},
				}},
			}, nil
		}

		info, err := cli.GetGroupInfoFromLink(ctx, InviteLinkPrefix+"ABC")
		require.NoError(t, err)
		assert.Equal(t, "Link Group", info.Name)

		joinedJID, errJoin := cli.JoinGroupWithLink(ctx, InviteLinkPrefix+"ABC")
		require.NoError(t, errJoin)
		assert.Equal(t, groupJID, joinedJID)
	})

	t.Run("JoinGroupWithLink approval request", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:   "membership_approval_request",
					Attrs: waBinary.Attrs{"jid": groupJID},
				}},
			}, nil
		}

		reqJID, err := cli.JoinGroupWithLink(ctx, "ABC")
		require.NoError(t, err)
		assert.Equal(t, groupJID, reqJID)
	})

	t.Run("Link errors", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQGone
		}
		_, err := cli.GetGroupInfoFromLink(ctx, "ABC")
		assert.ErrorIs(t, err, ErrInviteLinkRevoked)

		_, err = cli.JoinGroupWithLink(ctx, "ABC")
		assert.ErrorIs(t, err, ErrInviteLinkRevoked)

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, ErrIQNotAcceptable
		}
		_, err = cli.GetGroupInfoFromLink(ctx, "ABC")
		assert.ErrorIs(t, err, ErrInviteLinkInvalid)

		_, err = cli.JoinGroupWithLink(ctx, "ABC")
		assert.ErrorIs(t, err, ErrInviteLinkInvalid)
	})
}

func TestGroup_GetJoinedAndSubGroups(t *testing.T) {
	ctx := context.Background()
	community := types.NewJID("comm123", types.GroupServer)
	group1 := types.NewJID("subgroup1", types.GroupServer)

	t.Run("GetJoinedGroups success", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "groups",
					Content: []waBinary.Node{{
						Tag:   "group",
						Attrs: waBinary.Attrs{"id": "grp1", "subject": "Group One"},
					}},
				}},
			}, nil
		}

		groups, err := cli.GetJoinedGroups(ctx)
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.Equal(t, "Group One", groups[0].Name)

		// Missing groups tag
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, errMissing := cli.GetJoinedGroups(ctx)
		assert.Error(t, errMissing)
	})

	t.Run("GetSubGroups success", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "sub_groups",
					Content: []waBinary.Node{{
						Tag:   "group",
						Attrs: waBinary.Attrs{"id": group1.User, "subject": "Sub 1"},
						Content: []waBinary.Node{{
							Tag: "default_sub_group",
						}},
					}},
				}},
			}, nil
		}

		subGroups, err := cli.GetSubGroups(ctx, community)
		require.NoError(t, err)
		require.Len(t, subGroups, 1)
		assert.True(t, subGroups[0].IsDefaultSubGroup)

		// Missing sub_groups tag
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, errMissingSub := cli.GetSubGroups(ctx, community)
		assert.Error(t, errMissingSub)
	})

	t.Run("GetLinkedGroupsParticipants success", func(t *testing.T) {
		cli, _ := setupTestGroupClient()
		user1 := types.NewJID("62811", types.DefaultUserServer)
		userLID := types.NewJID("userlid", types.HiddenUserServer)
		userPN := types.NewJID("62822", types.DefaultUserServer)

		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "linked_groups_participants",
					Content: []waBinary.Node{
						{Tag: "participant", Attrs: waBinary.Attrs{"jid": user1}},
						{Tag: "participant", Attrs: waBinary.Attrs{"jid": userLID, "phone_number": userPN}},
					},
				}},
			}, nil
		}

		parts, err := cli.GetLinkedGroupsParticipants(ctx, community)
		require.NoError(t, err)
		assert.Len(t, parts, 2)

		// Missing linked_groups_participants tag
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, errMissingLink := cli.GetLinkedGroupsParticipants(ctx, community)
		assert.Error(t, errMissingLink)
	})
}

func TestGroup_GetGroupInfoAndCache(t *testing.T) {
	ctx := context.Background()
	groupJID := types.NewJID("grp123", types.GroupServer)
	userAdmin := types.NewJID("62811", types.DefaultUserServer)
	userMember := types.NewJID("62822", types.DefaultUserServer)

	cli, _ := setupTestGroupClient()
	cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
		return &waBinary.Node{
			Tag: "iq",
			Content: []waBinary.Node{{
				Tag: "group",
				Attrs: waBinary.Attrs{
					"id":      query.To.User,
					"subject": "Cached Group",
					"creator": userAdmin,
				},
				Content: []waBinary.Node{
					{
						Tag:   "participant",
						Attrs: waBinary.Attrs{"jid": userAdmin, "type": "superadmin"},
					},
					{
						Tag:   "participant",
						Attrs: waBinary.Attrs{"jid": userMember},
					},
					{
						Tag: "announcement",
					},
					{
						Tag:   "description",
						Attrs: waBinary.Attrs{"id": "topic-1", "t": "1700000000"},
						Content: []waBinary.Node{{
							Tag:     "body",
							Content: []byte("Group Topic"),
						}},
					},
					{
						Tag:   "ephemeral",
						Attrs: waBinary.Attrs{"expiration": "86400"},
					},
					{
						Tag:     "member_add_mode",
						Content: []byte("all_member_add"),
					},
					{
						Tag:   "parent",
						Attrs: waBinary.Attrs{"default_membership_approval_mode": "request_required"},
					},
					{
						Tag: "default_sub_group",
					},
					{
						Tag: "incognito",
					},
					{
						Tag: "membership_approval_mode",
					},
					{
						Tag: "suspended",
					},
				},
			}},
		}, nil
	}

	info, err := cli.GetGroupInfo(ctx, groupJID)
	require.NoError(t, err)
	assert.Equal(t, "Cached Group", info.Name)
	assert.True(t, info.IsAnnounce)

	// Fetch via cached data (hits cache)
	cached, errCached := cli.getCachedGroupData(ctx, groupJID)
	require.NoError(t, errCached)
	assert.True(t, cached.ReadOnly)
	assert.Len(t, cached.Members, 2)
	assert.Len(t, cached.Admins, 1)

	// Fetch uncached group data (triggers getGroupInfo inside getCachedGroupData)
	uncachedJID := types.NewJID("uncached123", types.GroupServer)
	cached2, errCached2 := cli.getCachedGroupData(ctx, uncachedJID)
	require.NoError(t, errCached2)
	assert.NotNil(t, cached2)

	// Error cases for getGroupInfo
	cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
		return nil, ErrIQNotFound
	}
	_, errNotFound := cli.GetGroupInfo(ctx, groupJID)
	assert.ErrorIs(t, errNotFound, ErrGroupNotFound)

	cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
		return nil, ErrIQForbidden
	}
	_, errForbidden := cli.GetGroupInfo(ctx, groupJID)
	assert.ErrorIs(t, errForbidden, ErrNotInGroup)

	// Missing group node
	cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
		return &waBinary.Node{Tag: "iq"}, nil
	}
	_, errMissing := cli.GetGroupInfo(ctx, groupJID)
	assert.Error(t, errMissing)
}

func TestGroup_ParseParticipant(t *testing.T) {
	node := waBinary.Node{
		Tag: "participant",
		Attrs: waBinary.Attrs{
			"jid":          types.NewJID("userlid", types.HiddenUserServer),
			"phone_number": types.NewJID("62899", types.DefaultUserServer),
			"type":         "admin",
			"display_name": "John Doe",
			"error":        "403",
		},
		Content: []waBinary.Node{{
			Tag: "add_request",
			Attrs: waBinary.Attrs{
				"code":       "REQCODE",
				"expiration": "1700000000",
			},
		}},
	}

	part := parseParticipant(node.AttrGetter(), &node)
	assert.True(t, part.IsAdmin)
	assert.False(t, part.IsSuperAdmin)
	assert.Equal(t, "John Doe", part.DisplayName)
	assert.Equal(t, 403, part.Error)
	require.NotNil(t, part.AddRequest)
	assert.Equal(t, "REQCODE", part.AddRequest.Code)
}

func TestGroup_ParseGroupNotification(t *testing.T) {
	cli, _ := setupTestGroupClient()
	groupJID := types.NewJID("group123", types.GroupServer)
	userSender := types.NewJID("62811", types.DefaultUserServer)

	t.Run("Create notification", func(t *testing.T) {
		parentNode := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"from":        groupJID,
				"participant": userSender,
			},
		}
		createNode := waBinary.Node{
			Tag: "create",
			Attrs: waBinary.Attrs{
				"reason": "created",
			},
			Content: []waBinary.Node{{
				Tag:   "group",
				Attrs: waBinary.Attrs{"id": groupJID.User, "subject": "Newly Created"},
			}},
		}
		parentNode.Content = []waBinary.Node{createNode}

		res, _, _, err := cli.parseGroupNotification(parentNode)
		require.NoError(t, err)
		evt, ok := res.(*events.JoinedGroup)
		require.True(t, ok)
		assert.Equal(t, "Newly Created", evt.GroupInfo.Name)
	})

	t.Run("Group changes notification", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"from":        groupJID,
				"participant": userSender,
				"t":           "1700000000",
			},
			Content: []waBinary.Node{
				{
					Tag: "subject",
					Attrs: waBinary.Attrs{
						"subject": "Changed Subject",
						"s_t":     "1700000000",
					},
				},
				{
					Tag: "locked",
				},
				{
					Tag: "unlocked",
				},
				{
					Tag:   "not_announcement",
					Attrs: waBinary.Attrs{"v_id": "2"},
				},
				{
					Tag: "not_ephemeral",
				},
				{
					Tag:   "delete",
					Attrs: waBinary.Attrs{"reason": "deleted"},
				},
				{
					Tag:   "invite",
					Attrs: waBinary.Attrs{"code": "INVITE123"},
				},
				{
					Tag:   "description",
					Attrs: waBinary.Attrs{"id": "desc1"},
					Content: []waBinary.Node{{
						Tag:     "body",
						Content: []byte("New Topic Body"),
					}},
				},
				{
					Tag: "add",
					Content: []waBinary.Node{{
						Tag:   "participant",
						Attrs: waBinary.Attrs{"jid": userSender},
					}},
				},
				{
					Tag: "promote",
					Content: []waBinary.Node{{
						Tag:   "participant",
						Attrs: waBinary.Attrs{"jid": userSender},
					}},
				},
				{
					Tag: "demote",
					Content: []waBinary.Node{{
						Tag:   "participant",
						Attrs: waBinary.Attrs{"jid": userSender},
					}},
				},
				{
					Tag: "remove",
					Content: []waBinary.Node{{
						Tag:   "participant",
						Attrs: waBinary.Attrs{"jid": userSender},
					}},
				},
				{
					Tag:   "link",
					Attrs: waBinary.Attrs{"link_type": "sub"},
					Content: []waBinary.Node{{
						Tag:   "group",
						Attrs: waBinary.Attrs{"id": "subgroup99", "subject": "Sub Group 99"},
					}},
				},
				{
					Tag:   "unlink",
					Attrs: waBinary.Attrs{"unlink_type": "sub", "unlink_reason": "removed"},
					Content: []waBinary.Node{{
						Tag:   "group",
						Attrs: waBinary.Attrs{"id": "subgroup99", "subject": "Sub Group 99"},
					}},
				},
				{
					Tag: "unknown_custom_tag",
				},
				{
					Tag: "announcement",
					Attrs: waBinary.Attrs{
						"v_id": "1",
					},
				},
				{
					Tag: "ephemeral",
					Attrs: waBinary.Attrs{
						"expiration": "86400",
					},
				},
				{
					Tag: "membership_approval_mode",
				},
				{
					Tag: "suspended",
				},
				{
					Tag: "unsuspended",
				},
			},
		}

		res, _, _, err := cli.parseGroupNotification(node)
		require.NoError(t, err)
		evt, ok := res.(*events.GroupInfo)
		require.True(t, ok)
		assert.Equal(t, "Changed Subject", evt.Name.Name)
		assert.False(t, evt.Locked.IsLocked)
		assert.True(t, evt.Announce.IsAnnounce)
		assert.True(t, evt.Ephemeral.IsEphemeral)
		assert.Equal(t, uint32(86400), evt.Ephemeral.DisappearingTimer)
		assert.True(t, evt.Suspended)
		assert.True(t, evt.Unsuspended)
		assert.NotNil(t, evt.NewInviteLink)
		assert.NotNil(t, evt.Topic)
		assert.Equal(t, "New Topic Body", evt.Topic.Topic)
		assert.NotNil(t, evt.Link)
		assert.NotNil(t, evt.Unlink)
		assert.NotEmpty(t, evt.UnknownChanges)
	})

	t.Run("Update participant cache", func(t *testing.T) {
		cli.groupCacheLock.Lock()
		cli.groupCache[groupJID] = &groupMetaCache{
			Members: []types.JID{userSender},
			Admins:  []types.JID{},
		}
		cli.groupCacheLock.Unlock()

		newUser := types.NewJID("62833", types.DefaultUserServer)
		changeEvt := &events.GroupInfo{
			JID:     groupJID,
			Join:    []types.JID{newUser},
			Promote: []types.JID{newUser},
		}
		cli.updateGroupParticipantCache(changeEvt)

		cli.groupCacheLock.Lock()
		cached := cli.groupCache[groupJID]
		cli.groupCacheLock.Unlock()

		assert.Contains(t, cached.Members, newUser)
		assert.Contains(t, cached.Admins, newUser)

		// Demote and Leave
		changeEvt2 := &events.GroupInfo{
			JID:    groupJID,
			Demote: []types.JID{newUser},
			Leave:  []types.JID{newUser},
		}
		cli.updateGroupParticipantCache(changeEvt2)

		cli.groupCacheLock.Lock()
		cached2 := cli.groupCache[groupJID]
		cli.groupCacheLock.Unlock()

		assert.NotContains(t, cached2.Members, newUser)
		assert.NotContains(t, cached2.Admins, newUser)
	})
}
