// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/proto/instamadilloAddMessage"
	"github.com/PakaiWA/whatsmeow/proto/instamadilloDeleteMessage"
	"github.com/PakaiWA/whatsmeow/proto/instamadilloSupplementMessage"
	"github.com/PakaiWA/whatsmeow/proto/instamadilloTransportPayload"
	"github.com/PakaiWA/whatsmeow/proto/waArmadilloApplication"
	"github.com/PakaiWA/whatsmeow/proto/waCommon"
	"github.com/PakaiWA/whatsmeow/proto/waConsumerApplication"
	"github.com/PakaiWA/whatsmeow/proto/waMsgApplication"
	"github.com/PakaiWA/whatsmeow/proto/waMsgTransport"
	"github.com/PakaiWA/whatsmeow/proto/waMultiDevice"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestArmadillo_DecodeArmadillo_Errors(t *testing.T) {
	t.Run("Malformed protobuf", func(t *testing.T) {
		_, err := decodeArmadillo([]byte("malformed-bytes"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal transport")
	})

	t.Run("Empty payload returns empty without error", func(t *testing.T) {
		transport := &waMsgTransport.MessageTransport{}
		bytes, err := proto.Marshal(transport)
		require.NoError(t, err)

		dec, err := decodeArmadillo(bytes)
		require.NoError(t, err)
		assert.NotNil(t, dec.Transport)
		assert.Nil(t, dec.Message)
	})

	t.Run("Unsupported version in transport", func(t *testing.T) {
		transport := &waMsgTransport.MessageTransport{
			Payload: &waMsgTransport.MessageTransport_Payload{
				ApplicationPayload: &waCommon.SubProtocol{
					Version: proto.Int32(9999),
					Payload: []byte("some-payload"),
				},
			},
		}
		bytes, err := proto.Marshal(transport)
		require.NoError(t, err)

		_, err = decodeArmadillo(bytes)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported subprotocol version")
	})
}

func TestArmadillo_DecodeFBArmadillo_And_SubProtocols(t *testing.T) {
	t.Run("DecodeFBApplication with nil payload", func(t *testing.T) {
		app := &waMsgApplication.MessageApplication{}
		dec, err := DecodeFBApplication(app)
		require.NoError(t, err)
		assert.Equal(t, app, dec.FBApplication)
		assert.Nil(t, dec.Message)
	})

	t.Run("DecodeFBApplication unsupported payloads", func(t *testing.T) {
		appCore := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_CoreContent{},
			},
		}
		_, err := DecodeFBApplication(appCore)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported core content payload")

		appSignal := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_Signal{},
			},
		}
		_, err = DecodeFBApplication(appSignal)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported signal payload")

		appData := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_ApplicationData{},
			},
		}
		_, err = DecodeFBApplication(appData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported application data payload")

		// Unknown / nil typed content
		appNilContent := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{},
		}
		_, err = DecodeFBApplication(appNilContent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported application payload content type")
	})

	t.Run("DecodeFBApplication Business, Payment, and Voip subprotocols", func(t *testing.T) {
		// Business
		bizApp := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_SubProtocol{
					SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload{
						SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload_BusinessMessage{
							BusinessMessage: &waCommon.SubProtocol{},
						},
					},
				},
			},
		}
		dec, err := DecodeFBApplication(bizApp)
		require.NoError(t, err)
		assert.NotNil(t, dec.Message)

		// Payment
		payApp := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_SubProtocol{
					SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload{
						SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload_PaymentMessage{
							PaymentMessage: &waCommon.SubProtocol{},
						},
					},
				},
			},
		}
		dec, err = DecodeFBApplication(payApp)
		require.NoError(t, err)
		assert.NotNil(t, dec.Message)

		// Voip
		voipApp := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_SubProtocol{
					SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload{
						SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload_Voip{
							Voip: &waCommon.SubProtocol{},
						},
					},
				},
			},
		}
		dec, err = DecodeFBApplication(voipApp)
		require.NoError(t, err)
		assert.NotNil(t, dec.Message)

		// MultiDevice
		multiDeviceMsg := &waMultiDevice.MultiDevice{}
		multiDeviceBytes, err := proto.Marshal(multiDeviceMsg)
		require.NoError(t, err)
		multiDeviceApp := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_SubProtocol{
					SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload{
						SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload_MultiDevice{
							MultiDevice: &waCommon.SubProtocol{
								Version: proto.Int32(waMsgApplication.MultiDeviceApplicationVersion),
								Payload: multiDeviceBytes,
							},
						},
					},
				},
			},
		}
		dec, err = DecodeFBApplication(multiDeviceApp)
		require.NoError(t, err)
		assert.NotNil(t, dec.Message)

		// Armadillo
		armadilloMsg := &waArmadilloApplication.Armadillo{}
		armadilloBytes, err := proto.Marshal(armadilloMsg)
		require.NoError(t, err)
		armadilloApp := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_SubProtocol{
					SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload{
						SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload_Armadillo{
							Armadillo: &waCommon.SubProtocol{
								Version: proto.Int32(waMsgApplication.ArmadilloApplicationVersion),
								Payload: armadilloBytes,
							},
						},
					},
				},
			},
		}
		dec, err = DecodeFBApplication(armadilloApp)
		require.NoError(t, err)
		assert.NotNil(t, dec.Message)

		// Unsupported subprotocol
		unsupportedSubApp := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_SubProtocol{
					SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload{},
				},
			},
		}
		_, err = DecodeFBApplication(unsupportedSubApp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported subprotocol type")
	})

	t.Run("DecodeFBApplication ConsumerMessage subprotocol", func(t *testing.T) {
		consumerContent := &waConsumerApplication.ConsumerApplication{
			Payload: &waConsumerApplication.ConsumerApplication_Payload{
				Payload: &waConsumerApplication.ConsumerApplication_Payload_Content{
					Content: &waConsumerApplication.ConsumerApplication_Content{
						Content: &waConsumerApplication.ConsumerApplication_Content_MessageText{
							MessageText: &waCommon.MessageText{
								Text: proto.String("hello armadillo consumer"),
							},
						},
					},
				},
			},
		}
		consumerBytes, err := proto.Marshal(consumerContent)
		require.NoError(t, err)

		app := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_SubProtocol{
					SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload{
						SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload_ConsumerMessage{
							ConsumerMessage: &waCommon.SubProtocol{
								Version: proto.Int32(waMsgApplication.ConsumerApplicationVersion),
								Payload: consumerBytes,
							},
						},
					},
				},
			},
		}

		dec, err := DecodeFBApplication(app)
		require.NoError(t, err)
		require.NotNil(t, dec.Message)
	})

	t.Run("Full FB transport decode", func(t *testing.T) {
		app := &waMsgApplication.MessageApplication{}
		appBytes, err := proto.Marshal(app)
		require.NoError(t, err)

		fbTransport := &waMsgTransport.MessageTransport{
			Payload: &waMsgTransport.MessageTransport_Payload{
				ApplicationPayload: &waCommon.SubProtocol{
					Version: proto.Int32(waMsgTransport.FBMessageApplicationVersion),
					Payload: appBytes,
				},
			},
		}
		data, err := proto.Marshal(fbTransport)
		require.NoError(t, err)

		dec, err := decodeArmadillo(data)
		require.NoError(t, err)
		assert.NotNil(t, dec.Transport)
		assert.NotNil(t, dec.FBApplication)

		// FB transport decode error
		badFBTransport := &waMsgTransport.MessageTransport{
			Payload: &waMsgTransport.MessageTransport_Payload{
				ApplicationPayload: &waCommon.SubProtocol{
					Version: proto.Int32(waMsgTransport.FBMessageApplicationVersion),
					Payload: []byte("invalid-fb-app"),
				},
			},
		}
		badData, err := proto.Marshal(badFBTransport)
		require.NoError(t, err)
		_, err = decodeArmadillo(badData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal application")
	})
}

func TestArmadillo_DecodeIGArmadillo(t *testing.T) {
	t.Run("Decode IG Add, Supplement, Delete payloads", func(t *testing.T) {
		addPayload := &instamadilloTransportPayload.TransportPayload{
			TransportPayload: &instamadilloTransportPayload.TransportPayload_Add{
				Add: &instamadilloAddMessage.AddMessagePayload{},
			},
		}
		addBytes, err := proto.Marshal(addPayload)
		require.NoError(t, err)

		igTransport := &waMsgTransport.MessageTransport{
			Payload: &waMsgTransport.MessageTransport_Payload{
				ApplicationPayload: &waCommon.SubProtocol{
					Version: proto.Int32(waMsgTransport.IGMessageApplicationVersion),
					Payload: addBytes,
				},
			},
		}
		data, err := proto.Marshal(igTransport)
		require.NoError(t, err)

		dec, err := decodeArmadillo(data)
		require.NoError(t, err)
		assert.NotNil(t, dec.IGTransport)
		assert.NotNil(t, dec.Message)

		// Supplement
		suppPayload := &instamadilloTransportPayload.TransportPayload{
			TransportPayload: &instamadilloTransportPayload.TransportPayload_Supplement{
				Supplement: &instamadilloSupplementMessage.SupplementMessagePayload{},
			},
		}
		suppBytes, err := proto.Marshal(suppPayload)
		require.NoError(t, err)
		igTransport.Payload.ApplicationPayload.Payload = suppBytes
		data, err = proto.Marshal(igTransport)
		require.NoError(t, err)

		dec, err = decodeArmadillo(data)
		require.NoError(t, err)
		assert.NotNil(t, dec.Message)

		// Delete
		delPayload := &instamadilloTransportPayload.TransportPayload{
			TransportPayload: &instamadilloTransportPayload.TransportPayload_Delete{
				Delete: &instamadilloDeleteMessage.DeleteMessagePayload{},
			},
		}
		delBytes, err := proto.Marshal(delPayload)
		require.NoError(t, err)
		igTransport.Payload.ApplicationPayload.Payload = delBytes
		data, err = proto.Marshal(igTransport)
		require.NoError(t, err)

		dec, err = decodeArmadillo(data)
		require.NoError(t, err)
		assert.NotNil(t, dec.Message)

		// IG decode error
		badIGTransport := &waMsgTransport.MessageTransport{
			Payload: &waMsgTransport.MessageTransport_Payload{
				ApplicationPayload: &waCommon.SubProtocol{
					Version: proto.Int32(waMsgTransport.IGMessageApplicationVersion),
					Payload: []byte("invalid-ig-payload"),
				},
			},
		}
		badData, err := proto.Marshal(badIGTransport)
		require.NoError(t, err)
		_, err = decodeArmadillo(badData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal IG transport")
	})
}

func TestArmadillo_HandleDecryptedArmadillo(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("Decode error sets protobufFailed", func(t *testing.T) {
		info := &types.MessageInfo{}
		handlerFailed, protoFailed := cli.handleDecryptedArmadillo(ctx, info, []byte("invalid-bytes"), 0)
		assert.False(t, handlerFailed)
		assert.True(t, protoFailed)
	})

	t.Run("Sender key distribution message handling in group and direct", func(t *testing.T) {
		transport := &waMsgTransport.MessageTransport{
			Protocol: &waMsgTransport.MessageTransport_Protocol{
				Ancillary: &waMsgTransport.MessageTransport_Protocol_Ancillary{
					Skdm: &waMsgTransport.MessageTransport_Protocol_Ancillary_SenderKeyDistributionMessage{
						AxolotlSenderKeyDistributionMessage: []byte("skdm-data"),
					},
				},
			},
		}
		data, err := proto.Marshal(transport)
		require.NoError(t, err)

		// Direct chat warning log
		directInfo := &types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:    types.NewJID("user1", types.DefaultUserServer),
				Sender:  types.NewJID("user1", types.DefaultUserServer),
				IsGroup: false,
			},
		}
		handlerFailed, protoFailed := cli.handleDecryptedArmadillo(ctx, directInfo, data, 0)
		assert.False(t, handlerFailed)
		assert.False(t, protoFailed)

		// Group chat calls handleSenderKeyDistributionMessage
		groupInfo := &types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:    types.NewJID("group1", types.GroupServer),
				Sender:  types.NewJID("user1", types.DefaultUserServer),
				IsGroup: true,
			},
		}
		handlerFailed, protoFailed = cli.handleDecryptedArmadillo(ctx, groupInfo, data, 0)
		assert.False(t, handlerFailed)
		assert.False(t, protoFailed)
	})

	t.Run("Dispatches FBMessage event when Message or FBApplication is present", func(t *testing.T) {
		app := &waMsgApplication.MessageApplication{}
		appBytes, err := proto.Marshal(app)
		require.NoError(t, err)

		transport := &waMsgTransport.MessageTransport{
			Payload: &waMsgTransport.MessageTransport_Payload{
				ApplicationPayload: &waCommon.SubProtocol{
					Version: proto.Int32(waMsgTransport.FBMessageApplicationVersion),
					Payload: appBytes,
				},
			},
		}
		data, err := proto.Marshal(transport)
		require.NoError(t, err)

		info := &types.MessageInfo{ID: "armadillo-msg-1"}
		var dispatched *events.FBMessage
		cli.AddEventHandler(func(evt any) {
			if fb, ok := evt.(*events.FBMessage); ok {
				dispatched = fb
			}
		})

		handlerFailed, protoFailed := cli.handleDecryptedArmadillo(ctx, info, data, 1)
		assert.False(t, handlerFailed)
		assert.False(t, protoFailed)
		require.NotNil(t, dispatched)
		assert.Equal(t, types.MessageID("armadillo-msg-1"), dispatched.Info.ID)
		assert.Equal(t, 1, dispatched.RetryCount)
	})
}
