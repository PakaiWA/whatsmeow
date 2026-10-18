// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hkdfutil

import (
	"hash"
	"io"
)

// SetNewHKDFForTesting allows tests to inject a mock HKDF reader to test error branches.
func SetNewHKDFForTesting(fn func(h func() hash.Hash, secret, salt, info []byte) io.Reader) func() {
	orig := newHKDF
	newHKDF = fn
	return func() { newHKDF = orig }
}
