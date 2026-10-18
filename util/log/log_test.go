// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package waLog_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestNoopLogger(t *testing.T) {
	logger := waLog.Noop
	require.NotNil(t, logger)

	// Verify all methods execute safely without panic
	logger.Debugf("noop debug %d", 1)
	logger.Infof("noop info %s", "a")
	logger.Warnf("noop warn")
	logger.Errorf("noop error")

	sub := logger.Sub("submodule")
	require.NotNil(t, sub)
	sub.Debugf("sub debug")
	sub.Infof("sub info")
	sub.Warnf("sub warn")
	sub.Errorf("sub error")
}

func TestStdoutLogger(t *testing.T) {
	t.Run("Basic logging without color", func(t *testing.T) {
		logger := waLog.Stdout("TestMod", "DEBUG", false)
		require.NotNil(t, logger)

		logger.Debugf("debug message %d", 1)
		logger.Infof("info message %s", "ok")
		logger.Warnf("warn message")
		logger.Errorf("error message")
	})

	t.Run("Logging with color enabled", func(t *testing.T) {
		logger := waLog.Stdout("ColorMod", "INFO", true)
		require.NotNil(t, logger)

		// Debugf should be filtered out because minLevel is INFO
		logger.Debugf("filtered debug")
		logger.Infof("colored info")
		logger.Warnf("colored warn")
		logger.Errorf("colored error")
	})

	t.Run("Sublogger chaining", func(t *testing.T) {
		logger := waLog.Stdout("Parent", "WARN", false)
		sub := logger.Sub("Child")
		require.NotNil(t, sub)

		sub.Debugf("should be filtered")
		sub.Infof("should be filtered")
		sub.Warnf("child warning")
		sub.Errorf("child error")
	})

	t.Run("Empty minLevel outputs all", func(t *testing.T) {
		logger := waLog.Stdout("AllMod", "", false)
		require.NotNil(t, logger)

		logger.Debugf("all debug")
		logger.Infof("all info")
	})
}
