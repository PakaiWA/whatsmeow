// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package richresponse

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextEntity_UnmarshalJSON(t *testing.T) {
	t.Run("Invalid JSON wrapper", func(t *testing.T) {
		var te TextEntity
		err := json.Unmarshal([]byte(`{invalid`), &te)
		require.Error(t, err)
	})

	t.Run("Empty or null metadata", func(t *testing.T) {
		// null metadata
		var te1 TextEntity
		err := json.Unmarshal([]byte(`{"key":"k1","metadata":null}`), &te1)
		require.NoError(t, err)
		assert.Equal(t, "k1", te1.Key)
		assert.Nil(t, te1.Metadata)

		// omitted/empty metadata
		var te2 TextEntity
		err = json.Unmarshal([]byte(`{"key":"k2"}`), &te2)
		require.NoError(t, err)
		assert.Equal(t, "k2", te2.Key)
		assert.Nil(t, te2.Metadata)
	})

	t.Run("Invalid metadata JSON", func(t *testing.T) {
		var te TextEntity
		// metadata is not a JSON object or string
		err := te.UnmarshalJSON([]byte(`{"key":"k","metadata":123}`))
		require.Error(t, err)

		// metadata is known type but fields have invalid types
		err = te.UnmarshalJSON([]byte(`{"key":"k","metadata":{"__typename":"GenAILatexItem","latex_expression":123}}`))
		require.Error(t, err)
	})

	t.Run("GenAISearchCitationItem", func(t *testing.T) {
		data := []byte(`{"key":"cite","metadata":{"__typename":"GenAISearchCitationItem"}}`)
		var te TextEntity
		err := json.Unmarshal(data, &te)
		require.NoError(t, err)
		assert.Equal(t, "cite", te.Key)
		citation, ok := te.Metadata.(*GenAISearchCitationItem)
		require.True(t, ok)
		assert.Equal(t, "GenAISearchCitationItem", citation.TypeName)
		citation.isTextEntityMetadata()
	})

	t.Run("GenAIInlineLinkItem", func(t *testing.T) {
		data := []byte(`{"key":"link","metadata":{"__typename":"GenAIInlineLinkItem","url":"https://example.com","display_name":"Example"}}`)
		var te TextEntity
		err := json.Unmarshal(data, &te)
		require.NoError(t, err)
		assert.Equal(t, "link", te.Key)
		link, ok := te.Metadata.(*GenAIInlineLinkItem)
		require.True(t, ok)
		assert.Equal(t, "GenAIInlineLinkItem", link.TypeName)
		assert.Equal(t, "https://example.com", link.URL)
		assert.Equal(t, "Example", link.DisplayName)
		link.isTextEntityMetadata()
	})

	t.Run("GenAIDeepLinkItem", func(t *testing.T) {
		data := []byte(`{"key":"deep","metadata":{"__typename":"GenAIDeepLinkItem","deeplink_url":"app://test","text":"Open App"}}`)
		var te TextEntity
		err := json.Unmarshal(data, &te)
		require.NoError(t, err)
		assert.Equal(t, "deep", te.Key)
		deep, ok := te.Metadata.(*GenAIDeepLinkItem)
		require.True(t, ok)
		assert.Equal(t, "GenAIDeepLinkItem", deep.TypeName)
		assert.Equal(t, "app://test", deep.DeepLinkURL)
		assert.Equal(t, "Open App", deep.Text)
		deep.isTextEntityMetadata()
	})

	t.Run("GenAILatexItem", func(t *testing.T) {
		data := []byte(`{"key":"math","metadata":{"__typename":"GenAILatexItem","latex_expression":"E=mc^2"}}`)
		var te TextEntity
		err := json.Unmarshal(data, &te)
		require.NoError(t, err)
		assert.Equal(t, "math", te.Key)
		latex, ok := te.Metadata.(*GenAILatexItem)
		require.True(t, ok)
		assert.Equal(t, "GenAILatexItem", latex.TypeName)
		assert.Equal(t, "E=mc^2", latex.LatexExpression)
		latex.isTextEntityMetadata()
	})

	t.Run("UnknownTextEntityMetadata", func(t *testing.T) {
		data := []byte(`{"key":"unknown","metadata":{"__typename":"SomeFutureMetadata","foo":"bar"}}`)
		var te TextEntity
		err := json.Unmarshal(data, &te)
		require.NoError(t, err)
		assert.Equal(t, "unknown", te.Key)
		unk, ok := te.Metadata.(UnknownTextEntityMetadata)
		require.True(t, ok)
		assert.Contains(t, string(unk), "SomeFutureMetadata")
		unk.isTextEntityMetadata()
	})
}
