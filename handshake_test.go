// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/libsignal/ecc"
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/proto/waCert"
	"github.com/PakaiWA/whatsmeow/proto/waWa6"
	"github.com/PakaiWA/whatsmeow/socket"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

// Helper to generate a valid test certificate chain signed with a mock root CA
func generateTestCertChain(rootPriv [32]byte, leafKey [32]byte, validPeriod bool) ([]byte, error) {
	intermediateKP := keys.NewKeyPair()

	now := time.Now().Unix()
	var notBefore, notAfter uint64
	if validPeriod {
		notBefore = uint64(now - 100)
		notAfter = uint64(now + 1000)
	} else {
		notBefore = uint64(now - 1000)
		notAfter = uint64(now - 100) // expired
	}

	interDetails := &waCert.CertChain_NoiseCertificate_Details{
		Serial:       proto.Uint32(1),
		IssuerSerial: proto.Uint32(WACertIssuerSerial),
		Key:          intermediateKP.Pub[:],
		NotBefore:    proto.Uint64(notBefore),
		NotAfter:     proto.Uint64(notAfter),
	}
	interDetailsBytes, err := proto.Marshal(interDetails)
	if err != nil {
		return nil, err
	}
	interSig := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(rootPriv), interDetailsBytes)

	leafDetails := &waCert.CertChain_NoiseCertificate_Details{
		Serial:       proto.Uint32(2),
		IssuerSerial: proto.Uint32(1),
		Key:          leafKey[:],
		NotBefore:    proto.Uint64(notBefore),
		NotAfter:     proto.Uint64(notAfter),
	}
	leafDetailsBytes, err := proto.Marshal(leafDetails)
	if err != nil {
		return nil, err
	}
	leafSig := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*intermediateKP.Priv), leafDetailsBytes)

	chain := &waCert.CertChain{
		Intermediate: &waCert.CertChain_NoiseCertificate{
			Details:   interDetailsBytes,
			Signature: interSig[:],
		},
		Leaf: &waCert.CertChain_NoiseCertificate{
			Details:   leafDetailsBytes,
			Signature: leafSig[:],
		},
	}
	return proto.Marshal(chain)
}

func TestHandshake_CheckCertValidity(t *testing.T) {
	now := time.Now().Unix()

	t.Run("Valid cert", func(t *testing.T) {
		cert := &waCert.CertChain_NoiseCertificate_Details{
			NotBefore: proto.Uint64(uint64(now - 50)),
			NotAfter:  proto.Uint64(uint64(now + 500)),
		}
		err := checkCertValidity(cert)
		assert.NoError(t, err)
	})

	t.Run("Not valid yet", func(t *testing.T) {
		cert := &waCert.CertChain_NoiseCertificate_Details{
			NotBefore: proto.Uint64(uint64(now + 100)),
			NotAfter:  proto.Uint64(uint64(now + 500)),
		}
		err := checkCertValidity(cert)
		assert.ErrorContains(t, err, "not valid yet")
	})

	t.Run("Expired cert", func(t *testing.T) {
		cert := &waCert.CertChain_NoiseCertificate_Details{
			NotBefore: proto.Uint64(uint64(now - 500)),
			NotAfter:  proto.Uint64(uint64(now - 100)),
		}
		err := checkCertValidity(cert)
		assert.ErrorContains(t, err, "expired")
	})
}

func TestHandshake_VerifyServerCert(t *testing.T) {
	// Temporarily override WACertPubKey to our mock root public key
	origWACertPubKey := WACertPubKey
	defer func() { WACertPubKey = origWACertPubKey }()

	rootKP := keys.NewKeyPair()
	WACertPubKey = *rootKP.Pub

	staticKP := keys.NewKeyPair()

	t.Run("Valid cert chain", func(t *testing.T) {
		certChainBytes, err := generateTestCertChain(*rootKP.Priv, *staticKP.Pub, true)
		require.NoError(t, err)

		err = verifyServerCert(certChainBytes, staticKP.Pub[:])
		assert.NoError(t, err)
	})

	t.Run("Invalid protobuf", func(t *testing.T) {
		err := verifyServerCert([]byte("invalid-proto"), staticKP.Pub[:])
		assert.ErrorContains(t, err, "failed to unmarshal noise certificate")
	})

	t.Run("Missing cert parts", func(t *testing.T) {
		chain := &waCert.CertChain{
			Intermediate: &waCert.CertChain_NoiseCertificate{
				Details: []byte("details"),
				// Signature missing
			},
		}
		bytes, err := proto.Marshal(chain)
		require.NoError(t, err)

		err = verifyServerCert(bytes, staticKP.Pub[:])
		assert.ErrorContains(t, err, "missing parts of noise certificate")
	})

	t.Run("Wrong signature length", func(t *testing.T) {
		chain := &waCert.CertChain{
			Intermediate: &waCert.CertChain_NoiseCertificate{
				Details:   []byte("details"),
				Signature: []byte("too-short"),
			},
			Leaf: &waCert.CertChain_NoiseCertificate{
				Details:   []byte("details"),
				Signature: []byte("too-short"),
			},
		}
		bytes, err := proto.Marshal(chain)
		require.NoError(t, err)

		err = verifyServerCert(bytes, staticKP.Pub[:])
		assert.ErrorContains(t, err, "unexpected length of intermediate cert signature")
	})

	t.Run("Invalid intermediate signature", func(t *testing.T) {
		certChainBytes, err := generateTestCertChain(*rootKP.Priv, *staticKP.Pub, true)
		require.NoError(t, err)

		var chain waCert.CertChain
		err = proto.Unmarshal(certChainBytes, &chain)
		require.NoError(t, err)
		chain.Intermediate.Signature[0] ^= 0xff // corrupt signature

		corruptBytes, err := proto.Marshal(&chain)
		require.NoError(t, err)

		err = verifyServerCert(corruptBytes, staticKP.Pub[:])
		assert.ErrorContains(t, err, "failed to verify intermediate cert signature")
	})

	t.Run("Expired cert", func(t *testing.T) {
		certChainBytes, err := generateTestCertChain(*rootKP.Priv, *staticKP.Pub, false)
		require.NoError(t, err)

		err = verifyServerCert(certChainBytes, staticKP.Pub[:])
		assert.ErrorContains(t, err, "expired")
	})

	t.Run("Issuer serial mismatch", func(t *testing.T) {
		interKP := keys.NewKeyPair()
		now := uint64(time.Now().Unix())
		interDetails := &waCert.CertChain_NoiseCertificate_Details{
			Serial:       proto.Uint32(1),
			IssuerSerial: proto.Uint32(999), // wrong issuer serial
			Key:          interKP.Pub[:],
			NotBefore:    proto.Uint64(now - 10),
			NotAfter:     proto.Uint64(now + 100),
		}
		raw, _ := proto.Marshal(interDetails)
		sig := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*rootKP.Priv), raw)
		chain := &waCert.CertChain{
			Intermediate: &waCert.CertChain_NoiseCertificate{
				Details:   raw,
				Signature: sig[:],
			},
			Leaf: &waCert.CertChain_NoiseCertificate{
				Details:   raw,
				Signature: sig[:],
			},
		}
		chainBytes, _ := proto.Marshal(chain)
		err := verifyServerCert(chainBytes, staticKP.Pub[:])
		assert.ErrorContains(t, err, "unexpected intermediate issuer serial")
	})

	t.Run("Invalid intermediate key length", func(t *testing.T) {
		now := uint64(time.Now().Unix())
		interDetails := &waCert.CertChain_NoiseCertificate_Details{
			Serial:       proto.Uint32(1),
			IssuerSerial: proto.Uint32(WACertIssuerSerial),
			Key:          []byte("short-key"),
			NotBefore:    proto.Uint64(now - 10),
			NotAfter:     proto.Uint64(now + 100),
		}
		raw, _ := proto.Marshal(interDetails)
		sig := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*rootKP.Priv), raw)
		chain := &waCert.CertChain{
			Intermediate: &waCert.CertChain_NoiseCertificate{
				Details:   raw,
				Signature: sig[:],
			},
			Leaf: &waCert.CertChain_NoiseCertificate{
				Details:   raw,
				Signature: sig[:],
			},
		}
		chainBytes, _ := proto.Marshal(chain)
		err := verifyServerCert(chainBytes, staticKP.Pub[:])
		assert.ErrorContains(t, err, "unexpected length of intermediate cert key")
	})

	t.Run("Key mismatch with static decrypted", func(t *testing.T) {
		otherKP := keys.NewKeyPair()
		certChainBytes, err := generateTestCertChain(*rootKP.Priv, *otherKP.Pub, true)
		require.NoError(t, err)

		err = verifyServerCert(certChainBytes, staticKP.Pub[:])
		assert.ErrorContains(t, err, "cert key doesn't match decrypted static")
	})
}

func TestHandshake_DoHandshake(t *testing.T) {
	origWACertPubKey := WACertPubKey
	defer func() { WACertPubKey = origWACertPubKey }()

	rootKP := keys.NewKeyPair()
	WACertPubKey = *rootKP.Pub

	serverStaticKP := keys.NewKeyPair()
	serverEphemeralKP := keys.NewKeyPair()

	t.Run("Success handshake roundtrip", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "")

			// 1. Read ClientHello
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			// Skip Header ("WA" = 2 bytes) + 3-byte length prefix
			headerLen := len("WA")
			if len(data) > headerLen+3 {
				data = data[headerLen+3:]
			}
			var clientHello waWa6.HandshakeMessage
			err = proto.Unmarshal(data, &clientHello)
			if err != nil {
				return
			}

			clientEphemeral := clientHello.GetClientHello().GetEphemeral()
			var clientEphemeralArr [32]byte
			copy(clientEphemeralArr[:], clientEphemeral)

			// Construct NoiseHandshake on server side
			serverNH := socket.NewNoiseHandshake()
			serverNH.Start(socket.NoiseStartPattern, []byte("WA"))
			serverNH.Authenticate(clientEphemeral)
			serverNH.Authenticate(serverEphemeralKP.Pub[:])

			err = serverNH.MixSharedSecretIntoKey(*serverEphemeralKP.Priv, clientEphemeralArr)
			if err != nil {
				return
			}

			staticEnc := serverNH.Encrypt(serverStaticKP.Pub[:])
			err = serverNH.MixSharedSecretIntoKey(*serverStaticKP.Priv, clientEphemeralArr)
			if err != nil {
				return
			}

			certChainBytes, err := generateTestCertChain(*rootKP.Priv, *serverStaticKP.Pub, true)
			if err != nil {
				return
			}
			certEnc := serverNH.Encrypt(certChainBytes)

			serverHelloMsg := &waWa6.HandshakeMessage{
				ServerHello: &waWa6.HandshakeMessage_ServerHello{
					Ephemeral: serverEphemeralKP.Pub[:],
					Static:    staticEnc,
					Payload:   certEnc,
				},
			}
			respBytes, err := proto.Marshal(serverHelloMsg)
			if err != nil {
				return
			}

			// Prepend 3-byte length prefix
			lengthPrefix := []byte{byte(len(respBytes) >> 16), byte(len(respBytes) >> 8), byte(len(respBytes))}
			err = conn.Write(context.Background(), websocket.MessageBinary, append(lengthPrefix, respBytes...))
			if err != nil {
				return
			}

			// 2. Read ClientFinish
			_, finishData, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			if len(finishData) > 3 {
				finishData = finishData[3:]
			}
			var clientFinish waWa6.HandshakeMessage
			_ = proto.Unmarshal(finishData, &clientFinish)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		fs := socket.NewFrameSocket(waLog.Noop, nil)
		fs.URL = wsURL
		fs.Header = []byte("WA")
		err := fs.Connect(context.Background())
		require.NoError(t, err)
		defer fs.Close(0)

		dev := &store.Device{
			ID:          &types.JID{User: "12345", Server: types.DefaultUserServer},
			NoiseKey:    keys.NewKeyPair(),
			IdentityKey: keys.NewKeyPair(),
		}
		cli := NewClient(dev, waLog.Noop)
		cli.GetClientPayload = func() *waWa6.ClientPayload {
			return &waWa6.ClientPayload{
				UserAgent: &waWa6.ClientPayload_UserAgent{
					AppVersion: &waWa6.ClientPayload_UserAgent_AppVersion{
						Primary: proto.Uint32(2),
					},
				},
			}
		}

		ephemeralKP := keys.NewKeyPair()
		queue, err := cli.doHandshake(fs, *ephemeralKP)
		require.NoError(t, err)
		assert.NotNil(t, queue)
		assert.NotNil(t, cli.socket)

		cli.expectDisconnect()
		cli.socket.Stop(false, false)
	})

	t.Run("Nil client payload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "")
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			if len(data) > 5 {
				data = data[5:]
			}
			var clientHello waWa6.HandshakeMessage
			_ = proto.Unmarshal(data, &clientHello)
			clientEphemeral := clientHello.GetClientHello().GetEphemeral()
			var clientEphemeralArr [32]byte
			copy(clientEphemeralArr[:], clientEphemeral)

			serverNH := socket.NewNoiseHandshake()
			serverNH.Start(socket.NoiseStartPattern, []byte("WA"))
			serverNH.Authenticate(clientEphemeral)
			serverNH.Authenticate(serverEphemeralKP.Pub[:])
			_ = serverNH.MixSharedSecretIntoKey(*serverEphemeralKP.Priv, clientEphemeralArr)
			staticEnc := serverNH.Encrypt(serverStaticKP.Pub[:])
			_ = serverNH.MixSharedSecretIntoKey(*serverStaticKP.Priv, clientEphemeralArr)
			certChainBytes, _ := generateTestCertChain(*rootKP.Priv, *serverStaticKP.Pub, true)
			certEnc := serverNH.Encrypt(certChainBytes)
			serverHelloMsg := &waWa6.HandshakeMessage{
				ServerHello: &waWa6.HandshakeMessage_ServerHello{
					Ephemeral: serverEphemeralKP.Pub[:],
					Static:    staticEnc,
					Payload:   certEnc,
				},
			}
			respBytes, _ := proto.Marshal(serverHelloMsg)
			lengthPrefix := []byte{byte(len(respBytes) >> 16), byte(len(respBytes) >> 8), byte(len(respBytes))}
			_ = conn.Write(context.Background(), websocket.MessageBinary, append(lengthPrefix, respBytes...))
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		fs := socket.NewFrameSocket(waLog.Noop, nil)
		fs.URL = wsURL
		fs.Header = []byte("WA")
		err := fs.Connect(context.Background())
		require.NoError(t, err)
		defer fs.Close(0)

		dev := &store.Device{
			ID:          &types.JID{User: "12345", Server: types.DefaultUserServer},
			NoiseKey:    keys.NewKeyPair(),
			IdentityKey: keys.NewKeyPair(),
		}
		cli := NewClient(dev, waLog.Noop)
		cli.GetClientPayload = func() *waWa6.ClientPayload {
			return nil // cause error
		}

		ephemeralKP := keys.NewKeyPair()
		_, err = cli.doHandshake(fs, *ephemeralKP)
		assert.ErrorContains(t, err, "got nil client payload")
	})

	t.Run("SendFrame failure", func(t *testing.T) {
		fs := socket.NewFrameSocket(waLog.Noop, nil)
		// Socket not connected -> SendFrame fails immediately
		dev := &store.Device{
			ID:          &types.JID{User: "12345", Server: types.DefaultUserServer},
			NoiseKey:    keys.NewKeyPair(),
			IdentityKey: keys.NewKeyPair(),
		}
		cli := NewClient(dev, waLog.Noop)

		ephemeralKP := keys.NewKeyPair()
		_, err := cli.doHandshake(fs, *ephemeralKP)
		assert.ErrorContains(t, err, "failed to send handshake message")
	})

	t.Run("Invalid handshake response format", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "")
			_, _, _ = conn.Read(context.Background())

			// Send corrupted message
			badBytes := []byte("not-valid-proto")
			lengthPrefix := []byte{0, 0, byte(len(badBytes))}
			_ = conn.Write(context.Background(), websocket.MessageBinary, append(lengthPrefix, badBytes...))
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		fs := socket.NewFrameSocket(waLog.Noop, nil)
		fs.URL = wsURL
		fs.Header = []byte("WA")
		err := fs.Connect(context.Background())
		require.NoError(t, err)
		defer fs.Close(0)

		dev := &store.Device{
			ID:          &types.JID{User: "12345", Server: types.DefaultUserServer},
			NoiseKey:    keys.NewKeyPair(),
			IdentityKey: keys.NewKeyPair(),
		}
		cli := NewClient(dev, waLog.Noop)

		ephemeralKP := keys.NewKeyPair()
		_, err = cli.doHandshake(fs, *ephemeralKP)
		assert.ErrorContains(t, err, "failed to unmarshal handshake response")
	})

	t.Run("Missing parts of handshake response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "")
			_, _, _ = conn.Read(context.Background())

			// Empty ServerHello missing ephemeral key and certs
			emptyMsg := &waWa6.HandshakeMessage{
				ServerHello: &waWa6.HandshakeMessage_ServerHello{},
			}
			respBytes, _ := proto.Marshal(emptyMsg)
			lengthPrefix := []byte{0, 0, byte(len(respBytes))}
			_ = conn.Write(context.Background(), websocket.MessageBinary, append(lengthPrefix, respBytes...))
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		fs := socket.NewFrameSocket(waLog.Noop, nil)
		fs.URL = wsURL
		fs.Header = []byte("WA")
		err := fs.Connect(context.Background())
		require.NoError(t, err)
		defer fs.Close(0)

		dev := &store.Device{
			ID:          &types.JID{User: "12345", Server: types.DefaultUserServer},
			NoiseKey:    keys.NewKeyPair(),
			IdentityKey: keys.NewKeyPair(),
		}
		cli := NewClient(dev, waLog.Noop)

		ephemeralKP := keys.NewKeyPair()
		_, err = cli.doHandshake(fs, *ephemeralKP)
		assert.ErrorContains(t, err, "missing parts of handshake response")
	})

	t.Run("Failed decrypt server static ciphertext", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "")
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			if len(data) > 5 {
				data = data[5:]
			}
			var clientHello waWa6.HandshakeMessage
			_ = proto.Unmarshal(data, &clientHello)

			msg := &waWa6.HandshakeMessage{
				ServerHello: &waWa6.HandshakeMessage_ServerHello{
					Ephemeral: serverEphemeralKP.Pub[:],
					Static:    []byte("bad-corrupt-static-ciphertext"),
					Payload:   []byte("payload"),
				},
			}
			respBytes, _ := proto.Marshal(msg)
			lengthPrefix := []byte{0, 0, byte(len(respBytes))}
			_ = conn.Write(context.Background(), websocket.MessageBinary, append(lengthPrefix, respBytes...))
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		fs := socket.NewFrameSocket(waLog.Noop, nil)
		fs.URL = wsURL
		fs.Header = []byte("WA")
		err := fs.Connect(context.Background())
		require.NoError(t, err)
		defer fs.Close(0)

		dev := &store.Device{
			ID:          &types.JID{User: "12345", Server: types.DefaultUserServer},
			NoiseKey:    keys.NewKeyPair(),
			IdentityKey: keys.NewKeyPair(),
		}
		cli := NewClient(dev, waLog.Noop)

		ephemeralKP := keys.NewKeyPair()
		_, err = cli.doHandshake(fs, *ephemeralKP)
		assert.ErrorContains(t, err, "failed to decrypt server static ciphertext")
	})
}
