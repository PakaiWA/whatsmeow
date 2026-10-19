// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestBusiness_GetOrderDetails(t *testing.T) {
	ctx := context.Background()

	t.Run("SendIQ error", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("network failure")
		}

		_, err := cli.GetOrderDetails(ctx, "ORDER-1", "token123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send order IQ")
	})

	t.Run("Missing order node in response", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.sendIQHook = func(_ context.Context, q infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "fb:thrift_iq", q.Namespace)
			assert.Equal(t, iqGet, q.Type)
			assert.Equal(t, "5", q.SMaxID)
			assert.Equal(t, types.ServerJID, q.To)
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{Tag: "not_order"},
				},
			}, nil
		}

		_, err := cli.GetOrderDetails(ctx, "ORDER-1", "token123")
		require.Error(t, err)
		var elemErr *ElementMissingError
		require.ErrorAs(t, err, &elemErr)
		assert.Equal(t, "order", elemErr.Tag)
	})

	t.Run("Success with full order details", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		now := time.Now().Truncate(time.Second)

		cli.sendIQHook = func(_ context.Context, q infoQuery) (*waBinary.Node, error) {
			assert.Equal(t, "fb:thrift_iq", q.Namespace)
			assert.Equal(t, iqGet, q.Type)
			assert.Equal(t, "5", q.SMaxID)
			assert.Equal(t, types.ServerJID, q.To)

			// verify request structure
			contentNodes, ok := q.Content.([]waBinary.Node)
			require.True(t, ok)
			require.Len(t, contentNodes, 1)
			orderReqNode := contentNodes[0]
			assert.Equal(t, "order", orderReqNode.Tag)
			assert.Equal(t, "get", orderReqNode.Attrs["op"])
			assert.Equal(t, "ORDER-12345", orderReqNode.Attrs["id"])

			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{
					{
						Tag: "order",
						Attrs: waBinary.Attrs{
							"id":          "ORDER-12345",
							"creation_ts": strconv.FormatInt(now.Unix(), 10),
						},
						Content: []waBinary.Node{
							{
								Tag: "price",
								Content: []waBinary.Node{
									{Tag: "subtotal", Content: []byte("150000")},
									{Tag: "total", Content: []byte("165000")},
									{Tag: "currency", Content: []byte("IDR")},
									{Tag: "price_status", Content: []byte("paid")},
								},
							},
							{
								Tag: "catalog",
								Content: []waBinary.Node{
									{Tag: "id", Content: []byte("CATALOG-XYZ")},
								},
							},
							{
								Tag: "product",
								Content: []waBinary.Node{
									{Tag: "id", Content: []byte("PROD-1")},
									{Tag: "name", Content: []byte("Awesome Item")},
									{Tag: "price", Content: []byte("75000")},
									{Tag: "quantity", Content: []byte("2")},
									{Tag: "currency", Content: []byte("IDR")},
									{
										Tag: "image",
										Content: []waBinary.Node{
											{Tag: "id", Content: []byte("IMG-123")},
											{Tag: "url", Content: []byte("https://example.com/img123.jpg")},
										},
									},
									{
										Tag: "variant_info",
										Content: []waBinary.Node{
											{Tag: "properties", Content: []byte("Color: Red, Size: L")},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		}

		details, err := cli.GetOrderDetails(ctx, "ORDER-12345", "token123")
		require.NoError(t, err)
		require.NotNil(t, details)
		assert.Equal(t, "ORDER-12345", details.ID)
		assert.Equal(t, now.Unix(), details.CreatedAt.Unix())
		assert.Equal(t, int64(150000), details.Price.Subtotal)
		assert.Equal(t, int64(165000), details.Price.Total)
		assert.Equal(t, "IDR", details.Price.Currency)
		assert.Equal(t, "paid", details.Price.PriceStatus)
		assert.Equal(t, "CATALOG-XYZ", details.CatalogID)
		require.Len(t, details.Products, 1)

		prod := details.Products[0]
		assert.Equal(t, "PROD-1", prod.ID)
		assert.Equal(t, "Awesome Item", prod.Name)
		assert.Equal(t, int64(75000), prod.Price)
		assert.Equal(t, 2, prod.Quantity)
		assert.Equal(t, "IDR", prod.Currency)
		assert.Equal(t, "IMG-123", prod.ImageID)
		assert.Equal(t, "https://example.com/img123.jpg", prod.ImageURL)
		assert.Equal(t, "Color: Red, Size: L", prod.VariantInfo.Properties)
	})
}

func TestBusiness_ParseOrderDetailsNode_EdgeCases(t *testing.T) {
	t.Run("Missing required attributes triggers error", func(t *testing.T) {
		// Missing 'id' attribute
		node := waBinary.Node{
			Tag: "order",
			Attrs: waBinary.Attrs{
				"creation_ts": strconv.FormatInt(time.Now().Unix(), 10),
			},
		}
		details, err := parseOrderDetailsNode(node)
		require.Error(t, err)
		assert.Nil(t, details)
	})

	t.Run("Minimal order without optional nodes", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		node := waBinary.Node{
			Tag: "order",
			Attrs: waBinary.Attrs{
				"id":          "ORDER-MINIMAL",
				"creation_ts": strconv.FormatInt(now.Unix(), 10),
			},
		}
		details, err := parseOrderDetailsNode(node)
		require.NoError(t, err)
		require.NotNil(t, details)
		assert.Equal(t, "ORDER-MINIMAL", details.ID)
		assert.Equal(t, now.Unix(), details.CreatedAt.Unix())
		assert.Empty(t, details.CatalogID)
		assert.Empty(t, details.Products)
		assert.Equal(t, int64(0), details.Price.Subtotal)
	})
}
