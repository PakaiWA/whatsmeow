// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package waLog_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type logRecord struct {
	Level     string `json:"level"`
	Message   string `json:"message"`
	Sublogger string `json:"sublogger"`
}

func parseLastLog(t *testing.T, buf *bytes.Buffer) logRecord {
	t.Helper()
	line := bytes.TrimSpace(buf.Bytes())
	require.NotEmpty(t, line, "expected non-empty log output")
	var rec logRecord
	err := json.Unmarshal(line, &rec)
	require.NoError(t, err, "failed to parse JSON log line: %s", string(line))
	buf.Reset()
	return rec
}

func TestZerolog_Levels(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf).Level(zerolog.DebugLevel)
	logger := waLog.Zerolog(zlog)
	require.NotNil(t, logger)

	t.Run("Debugf", func(t *testing.T) {
		logger.Debugf("debug counter: %d", 42)
		rec := parseLastLog(t, &buf)
		assert.Equal(t, "debug", rec.Level)
		assert.Equal(t, "debug counter: 42", rec.Message)
	})

	t.Run("Infof", func(t *testing.T) {
		logger.Infof("connected to %s", "server-1")
		rec := parseLastLog(t, &buf)
		assert.Equal(t, "info", rec.Level)
		assert.Equal(t, "connected to server-1", rec.Message)
	})

	t.Run("Warnf", func(t *testing.T) {
		logger.Warnf("resource usage high: %d%%", 85)
		rec := parseLastLog(t, &buf)
		assert.Equal(t, "warn", rec.Level)
		assert.Equal(t, "resource usage high: 85%", rec.Message)
	})

	t.Run("Errorf", func(t *testing.T) {
		logger.Errorf("connection failed: %s", "timeout")
		rec := parseLastLog(t, &buf)
		assert.Equal(t, "error", rec.Level)
		assert.Equal(t, "connection failed: timeout", rec.Message)
	})
}

func TestZerolog_Sub(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf).Level(zerolog.DebugLevel)
	logger := waLog.Zerolog(zlog)

	t.Run("Single Sublogger", func(t *testing.T) {
		sub := logger.Sub("network")
		require.NotNil(t, sub)

		sub.Infof("handshake success")
		rec := parseLastLog(t, &buf)
		assert.Equal(t, "info", rec.Level)
		assert.Equal(t, "handshake success", rec.Message)
		assert.Equal(t, "network", rec.Sublogger)
	})

	t.Run("Nested Sublogger chaining", func(t *testing.T) {
		parent := logger.Sub("socket")
		child := parent.Sub("noise")
		require.NotNil(t, child)

		child.Debugf("key exchange completed")
		rec := parseLastLog(t, &buf)
		assert.Equal(t, "debug", rec.Level)
		assert.Equal(t, "key exchange completed", rec.Message)
		assert.Equal(t, "socket/noise", rec.Sublogger)

		// Deeper level
		grandchild := child.Sub("crypto")
		grandchild.Warnf("deprecated curve")
		rec2 := parseLastLog(t, &buf)
		assert.Equal(t, "warn", rec2.Level)
		assert.Equal(t, "socket/noise/crypto", rec2.Sublogger)
	})
}

func TestZerolog_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf).Level(zerolog.WarnLevel)
	logger := waLog.Zerolog(zlog)

	logger.Debugf("hidden debug")
	assert.Empty(t, buf.Bytes(), "debug log should be filtered out when Level is Warn")

	logger.Infof("hidden info")
	assert.Empty(t, buf.Bytes(), "info log should be filtered out when Level is Warn")

	logger.Warnf("visible warning")
	assert.NotEmpty(t, buf.Bytes(), "warn log should be printed")
	rec := parseLastLog(t, &buf)
	assert.Equal(t, "warn", rec.Level)
	assert.Equal(t, "visible warning", rec.Message)
}
