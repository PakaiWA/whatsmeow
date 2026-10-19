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
	"github.com/PakaiWA/whatsmeow/proto/waWa6"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestNewsletter_ConvertQueryID(t *testing.T) {
	jid := types.NewJID("12345", types.DefaultUserServer)
	device := &store.Device{ID: &jid}
	cli := NewClient(device, waLog.Noop)

	// Default platform (Web) -> unchanged
	assert.Equal(t, queryFetchNewsletter, convertQueryID(cli, queryFetchNewsletter))
	assert.Equal(t, "unknown_query", convertQueryID(cli, "unknown_query"))

	// Desktop/MacOS platform (or nil WebInfo)
	origWebInfo := store.BaseClientPayload.WebInfo
	store.BaseClientPayload.WebInfo = nil
	defer func() {
		store.BaseClientPayload.WebInfo = origWebInfo
	}()

	testCases := []struct {
		in       string
		expected string
	}{
		{queryFetchNewsletter, queryFetchNewsletterDesktop},
		{queryRecommendedNewsletters, queryRecommendedNewslettersDesktop},
		{querySubscribedNewsletters, querySubscribedNewslettersDesktop},
		{queryNewsletterSubscribers, queryNewsletterSubscribersDesktop},
		{mutationMuteNewsletter, mutationMuteNewsletterDesktop},
		{mutationUnmuteNewsletter, mutationUnmuteNewsletterDesktop},
		{mutationUpdateNewsletter, mutationUpdateNewsletterDesktop},
		{mutationCreateNewsletter, mutationCreateNewsletterDesktop},
		{mutationUnfollowNewsletter, mutationUnfollowNewsletterDesktop},
		{mutationFollowNewsletter, mutationFollowNewsletterDesktop},
		{"custom_query", "custom_query"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, convertQueryID(cli, tc.in))
	}
}

func TestNewsletter_SubscribeLiveUpdates(t *testing.T) {
	ctx := context.Background()
	device := &store.Device{ID: nil}
	cli := NewClient(device, waLog.Noop)
	channelJID := types.NewJID("1234567890", types.NewsletterServer)

	t.Run("Success", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "newsletter", query.Namespace)
			assert.Equal(t, iqSet, query.Type)
			assert.Equal(t, channelJID, query.To)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:   "live_updates",
					Attrs: waBinary.Attrs{"duration": "300"},
				}},
			}, nil
		}

		dur, err := cli.NewsletterSubscribeLiveUpdates(ctx, channelJID)
		require.NoError(t, err)
		assert.Equal(t, 300*time.Second, dur)
	})

	t.Run("IQ error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("subscription failed")
		}
		_, err := cli.NewsletterSubscribeLiveUpdates(ctx, channelJID)
		require.Error(t, err)
	})
}

func TestNewsletter_MarkViewed(t *testing.T) {
	ctx := context.Background()
	channelJID := types.NewJID("1234567890", types.NewsletterServer)

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		err := nilCli.NewsletterMarkViewed(ctx, channelJID, []types.MessageServerID{100})
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Send success and response fulfillment", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		go func() {
			for i := 0; i < 200; i++ {
				time.Sleep(5 * time.Millisecond)
				cli.responseWaitersLock.Lock()
				for id, waiter := range cli.responseWaiters {
					delete(cli.responseWaiters, id)
					cli.responseWaitersLock.Unlock()
					waiter <- &waBinary.Node{Tag: "ack", Attrs: waBinary.Attrs{"id": id}}
					return
				}
				cli.responseWaitersLock.Unlock()
			}
		}()

		err := cli.NewsletterMarkViewed(ctx, channelJID, []types.MessageServerID{101, 102})
		require.NoError(t, err)
	})

	t.Run("Send error when socket not connected", func(t *testing.T) {
		device := &store.Device{ID: nil}
		cli := NewClient(device, waLog.Noop) // no socket
		err := cli.NewsletterMarkViewed(ctx, channelJID, []types.MessageServerID{101})
		require.ErrorIs(t, err, ErrNotConnected)
	})
}

func TestNewsletter_SendReaction(t *testing.T) {
	ctx := context.Background()
	channelJID := types.NewJID("1234567890", types.NewsletterServer)

	t.Run("Send reaction with code", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		err := cli.NewsletterSendReaction(ctx, channelJID, 12345, "👍", "msg-react-1")
		require.NoError(t, err)
	})

	t.Run("Send revoke reaction (empty code)", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		err := cli.NewsletterSendReaction(ctx, channelJID, 12345, "", "")
		require.NoError(t, err)
	})
}

func TestNewsletter_SendMexIQ(t *testing.T) {
	ctx := context.Background()
	jid := types.NewJID("12345", types.DefaultUserServer)
	device := &store.Device{ID: &jid}
	cli := NewClient(device, waLog.Noop)

	t.Run("BaseClientPayload MacOS error", func(t *testing.T) {
		orig := store.BaseClientPayload.UserAgent.Platform
		store.BaseClientPayload.UserAgent.Platform = waWa6.ClientPayload_UserAgent_MACOS.Enum()
		defer func() {
			store.BaseClientPayload.UserAgent.Platform = orig
		}()

		_, err := cli.sendMexIQ(ctx, queryFetchNewsletter, map[string]any{})
		require.ErrorContains(t, err, "argo decoding is currently broken")
	})

	t.Run("IQ Send error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("mex iq failed")
		}
		_, err := cli.sendMexIQ(ctx, queryFetchNewsletter, map[string]any{})
		require.ErrorContains(t, err, "mex iq failed")
	})

	t.Run("Missing result tag", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.sendMexIQ(ctx, queryFetchNewsletter, map[string]any{})
		var elemErr *ElementMissingError
		require.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "result", elemErr.Tag)
	})

	t.Run("Unexpected content type in result tag", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:     "result",
					Content: "invalid string content",
				}},
			}, nil
		}
		_, err := cli.sendMexIQ(ctx, queryFetchNewsletter, map[string]any{})
		require.ErrorContains(t, err, "unexpected content type")
	})

	t.Run("Argo format error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:     "result",
					Attrs:   waBinary.Attrs{"format": "argo"},
					Content: []byte("some-argo-bytes"),
				}},
			}, nil
		}
		_, err := cli.sendMexIQ(ctx, queryFetchNewsletter, map[string]any{})
		require.ErrorContains(t, err, "argo decoding is currently broken")
	})

	t.Run("JSON format unmarshal error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:     "result",
					Content: []byte("invalid-json{"),
				}},
			}, nil
		}
		_, err := cli.sendMexIQ(ctx, queryFetchNewsletter, map[string]any{})
		require.ErrorContains(t, err, "failed to unmarshal graphql response")
	})

	t.Run("GraphQL response with errors", func(t *testing.T) {
		respBody := `{"data":{"key":"val"},"errors":[{"message":"channel not found"}]}`
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:     "result",
					Content: []byte(respBody),
				}},
			}, nil
		}
		data, err := cli.sendMexIQ(ctx, queryFetchNewsletter, map[string]any{})
		require.ErrorContains(t, err, "graphql error")
		assert.NotEmpty(t, data)
	})

	t.Run("GraphQL response success", func(t *testing.T) {
		respBody := `{"data":{"xwa2_newsletter":{"id":"123"}}}`
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:     "result",
					Content: []byte(respBody),
				}},
			}, nil
		}
		data, err := cli.sendMexIQ(ctx, queryFetchNewsletter, map[string]any{})
		require.NoError(t, err)
		assert.JSONEq(t, `{"xwa2_newsletter":{"id":"123"}}`, string(data))
	})
}

func TestNewsletter_HighLevelAPIs(t *testing.T) {
	ctx := context.Background()
	jid := types.NewJID("12345", types.DefaultUserServer)
	device := &store.Device{ID: &jid}
	cli := NewClient(device, waLog.Noop)
	channelJID := types.NewJID("1234567890", types.NewsletterServer)

	mockMexData := func(body string, err error) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			if err != nil {
				return nil, err
			}
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:     "result",
					Content: []byte(`{"data":` + body + `}`),
				}},
			}, nil
		}
	}

	t.Run("GetNewsletterInfo by JID", func(t *testing.T) {
		mockMexData(`{"xwa2_newsletter":{"id":"123@newsletter","thread_metadata":{"name":{"text":"My Channel"}}}}`, nil)
		meta, err := cli.GetNewsletterInfo(ctx, channelJID)
		require.NoError(t, err)
		require.NotNil(t, meta)
		assert.Equal(t, types.NewJID("123", types.NewsletterServer), meta.ID)
		assert.Equal(t, "My Channel", meta.ThreadMeta.Name.Text)
	})

	t.Run("GetNewsletterInfoWithInvite", func(t *testing.T) {
		mockMexData(`{"xwa2_newsletter":{"id":"123@newsletter","thread_metadata":{"name":{"text":"Invite Channel"}}}}`, nil)
		meta, err := cli.GetNewsletterInfoWithInvite(ctx, "https://whatsapp.com/channel/abc123xyz")
		require.NoError(t, err)
		require.NotNil(t, meta)
		assert.Equal(t, "Invite Channel", meta.ThreadMeta.Name.Text)
	})

	t.Run("GetSubscribedNewsletters", func(t *testing.T) {
		mockMexData(`{"xwa2_newsletter_subscribed":[{"id":"101@newsletter"},{"id":"102@newsletter"}]}`, nil)
		list, err := cli.GetSubscribedNewsletters(ctx)
		require.NoError(t, err)
		require.Len(t, list, 2)
		assert.Equal(t, types.NewJID("101", types.NewsletterServer), list[0].ID)
		assert.Equal(t, types.NewJID("102", types.NewsletterServer), list[1].ID)
	})

	t.Run("CreateNewsletter", func(t *testing.T) {
		mockMexData(`{"xwa2_newsletter_create":{"id":"999@newsletter","thread_metadata":{"name":{"text":"New Channel"}}}}`, nil)
		meta, err := cli.CreateNewsletter(ctx, CreateNewsletterParams{
			Name:        "New Channel",
			Description: "Desc",
		})
		require.NoError(t, err)
		require.NotNil(t, meta)
		assert.Equal(t, types.NewJID("999", types.NewsletterServer), meta.ID)
	})

	t.Run("CreateNewsletter unmarshal error", func(t *testing.T) {
		mockMexData(`"not an object"`, nil)
		_, err := cli.CreateNewsletter(ctx, CreateNewsletterParams{Name: "Fail"})
		require.Error(t, err)
	})

	t.Run("AcceptTOSNotice", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "tos", query.Namespace)
			assert.Equal(t, iqSet, query.Type)
			return &waBinary.Node{Tag: "iq"}, nil
		}
		err := cli.AcceptTOSNotice(ctx, "20601218", "5")
		require.NoError(t, err)
	})

	t.Run("NewsletterToggleMute", func(t *testing.T) {
		mockMexData(`{"xwa2_newsletter_update":{"id":"123"}}`, nil)
		err := cli.NewsletterToggleMute(ctx, channelJID, true)
		require.NoError(t, err)
		err = cli.NewsletterToggleMute(ctx, channelJID, false)
		require.NoError(t, err)
	})

	t.Run("FollowNewsletter and UnfollowNewsletter", func(t *testing.T) {
		mockMexData(`{"xwa2_newsletter_follow":{"id":"123"}}`, nil)
		err := cli.FollowNewsletter(ctx, channelJID)
		require.NoError(t, err)

		mockMexData(`{"xwa2_newsletter_unfollow":{"id":"123"}}`, nil)
		err = cli.UnfollowNewsletter(ctx, channelJID)
		require.NoError(t, err)
	})
}

func TestNewsletter_GetMessagesAndUpdates(t *testing.T) {
	ctx := context.Background()
	device := &store.Device{ID: nil}
	cli := NewClient(device, waLog.Noop)
	channelJID := types.NewJID("1234567890", types.NewsletterServer)

	t.Run("GetNewsletterMessages success with params", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "newsletter", query.Namespace)
			assert.Equal(t, iqGet, query.Type)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "messages",
					Content: []waBinary.Node{{
						Tag: "message",
						Attrs: waBinary.Attrs{
							"server_id": "501",
							"id":        "msg-501",
							"type":      "text",
							"t":         "1700000000",
						},
						Content: []waBinary.Node{
							{Tag: "views_count", Attrs: waBinary.Attrs{"count": "42"}},
							{Tag: "forwards_count", Attrs: waBinary.Attrs{"count": "3"}},
							{
								Tag: "reactions",
								Content: []waBinary.Node{
									{Tag: "reaction", Attrs: waBinary.Attrs{"code": "❤️", "count": "10"}},
								},
							},
						},
					}},
				}},
			}, nil
		}

		msgs, err := cli.GetNewsletterMessages(ctx, channelJID, &GetNewsletterMessagesParams{
			Count:  10,
			Before: 600,
		})
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, types.MessageServerID(501), msgs[0].MessageServerID)
		assert.Equal(t, types.MessageID("msg-501"), msgs[0].MessageID)
		assert.Equal(t, 42, msgs[0].ViewsCount)
		assert.Equal(t, 3, msgs[0].ForwardsCount)
		assert.Equal(t, 10, msgs[0].ReactionCounts["❤️"])
	})

	t.Run("GetNewsletterMessages missing messages child", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.GetNewsletterMessages(ctx, channelJID, nil)
		var elemErr *ElementMissingError
		require.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "messages", elemErr.Tag)
	})

	t.Run("GetNewsletterMessages IQ error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("iq error")
		}
		_, err := cli.GetNewsletterMessages(ctx, channelJID, nil)
		require.Error(t, err)
	})

	t.Run("GetNewsletterMessageUpdates success with params", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "newsletter", query.Namespace)
			assert.Equal(t, iqGet, query.Type)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag: "message_updates",
					Content: []waBinary.Node{{
						Tag: "messages",
						Content: []waBinary.Node{{
							Tag: "message",
							Attrs: waBinary.Attrs{
								"server_id": "701",
								"id":        "msg-701",
								"type":      "text",
								"t":         "1700000000",
							},
						}},
					}},
				}},
			}, nil
		}

		updates, err := cli.GetNewsletterMessageUpdates(ctx, channelJID, &GetNewsletterUpdatesParams{
			Count: 5,
			Since: time.Unix(1690000000, 0),
			After: 700,
		})
		require.NoError(t, err)
		require.Len(t, updates, 1)
		assert.Equal(t, types.MessageServerID(701), updates[0].MessageServerID)
	})

	t.Run("GetNewsletterMessageUpdates missing messages child", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		_, err := cli.GetNewsletterMessageUpdates(ctx, channelJID, nil)
		var elemErr *ElementMissingError
		require.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "messages", elemErr.Tag)
	})

	t.Run("GetNewsletterMessageUpdates IQ error", func(t *testing.T) {
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("iq fail")
		}
		_, err := cli.GetNewsletterMessageUpdates(ctx, channelJID, nil)
		require.Error(t, err)
	})
}
