// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"net/http"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"

	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestMakeDialOptions(t *testing.T) {
	client := &http.Client{}
	fs := NewFrameSocket(waLog.Noop, client)
	fs.HTTPHeaders.Set("X-Custom-Header", "Value123")

	opts := fs.makeDialOptions()
	assert.Equal(t, client, opts.HTTPClient)
	assert.Equal(t, fs.HTTPHeaders, opts.HTTPHeader)
	assert.Equal(t, "Value123", opts.HTTPHeader.Get("X-Custom-Header"))
	assert.Equal(t, websocket.CompressionContextTakeover, opts.CompressionMode)
}
