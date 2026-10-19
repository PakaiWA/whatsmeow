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

func TestCleanPlaceholders(t *testing.T) {
	input := "Hello {{link}}https://whatsapp.com{{/link}} world {{bold}}bolded text{{/bold}}!"
	expected := "Hello https://whatsapp.com world bolded text!"
	assert.Equal(t, expected, cleanPlaceholders(input))

	noPlaceholder := "Just plain text"
	assert.Equal(t, noPlaceholder, cleanPlaceholders(noPlaceholder))
}

func TestPrimitive_Code(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAICodeUXPrimitive",
		"language": "python",
		"code_blocks": [
			{"content": "def hello():\n", "type": "KEYWORD"},
			{"content": "    print('world')\n", "type": "STR"}
		]
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	code, ok := pc.Value.(*GenAICodeUXPrimitive)
	require.True(t, ok)
	assert.Equal(t, "GenAICodeUXPrimitive", code.TypeName)
	assert.Equal(t, "python", code.Language)
	assert.Equal(t, "def hello():\n    print('world')\n", code.String())
	assert.Equal(t, "def hello():\n", code.CodeBlocks[0].GetContent())
	code.isPrimitive()
}

func TestPrimitive_MarkdownText(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAIMarkdownTextUXPrimitive",
		"text": "Answer: {{bold}}42{{/bold}}"
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	md, ok := pc.Value.(*GenAIMarkdownTextUXPrimitive)
	require.True(t, ok)
	assert.Equal(t, "Answer: 42", md.String())
	md.isPrimitive()
}

func TestPrimitive_Table(t *testing.T) {
	data := []byte(`{
		"__typename": "GenATableUXPrimitive",
		"rows": [
			{"cells": ["Col1", "Col2"], "is_header": true},
			{"cells": ["Val1", "Val2"]}
		]
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	tbl, ok := pc.Value.(*GenATableUXPrimitive)
	require.True(t, ok)
	expected := "Col1 | Col2\nVal1 | Val2"
	assert.Equal(t, expected, tbl.String())
	assert.True(t, tbl.Rows[0].IsHeader)
	tbl.isPrimitive()
}

func TestPrimitive_Latex(t *testing.T) {
	t.Run("Latex from Item", func(t *testing.T) {
		data := []byte(`{
			"__typename": "GenAILatexUXPrimitive",
			"latex_expression": "fallback",
			"item": {
				"latex_expression": "x^2 + y^2 = z^2"
			}
		}`)
		var pc PrimitiveContainer
		err := json.Unmarshal(data, &pc)
		require.NoError(t, err)
		latex, ok := pc.Value.(*GenAILatexUXPrimitive)
		require.True(t, ok)
		assert.Equal(t, "x^2 + y^2 = z^2", latex.String())
		assert.Equal(t, "x^2 + y^2 = z^2", latex.GetLatexExpression())
		latex.isPrimitive()
	})

	t.Run("Latex top-level fallback", func(t *testing.T) {
		data := []byte(`{
			"__typename": "GenAILatexUXPrimitive",
			"latex_expression": "a + b = c"
		}`)
		var pc PrimitiveContainer
		err := json.Unmarshal(data, &pc)
		require.NoError(t, err)
		latex, ok := pc.Value.(*GenAILatexUXPrimitive)
		require.True(t, ok)
		assert.Equal(t, "a + b = c", latex.String())
		assert.Equal(t, "a + b = c", latex.GetLatexExpression())
	})
}

func TestPrimitive_FOAText(t *testing.T) {
	data := []byte(`{
		"__typename": "FOATextPrimitive",
		"text": "Hello {{ref}}World{{/ref}}"
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	foa, ok := pc.Value.(*FOATextPrimitive)
	require.True(t, ok)
	assert.Equal(t, "Hello World", foa.String())
	foa.isPrimitive()
}

func TestPrimitive_MetadataText(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAIMetadataTextPrimitive",
		"text": "Some metadata summary"
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	meta, ok := pc.Value.(*GenAIMetadataTextPrimitive)
	require.True(t, ok)
	assert.Equal(t, "Some metadata summary", meta.String())
	meta.isPrimitive()
}

func TestPrimitive_BotThinkingStatus(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAIBotThinkingStatusPrimitive",
		"title": "Thinking...",
		"is_in_progress": true
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	status, ok := pc.Value.(*GenAIBotThinkingStatusPrimitive)
	require.True(t, ok)
	assert.Equal(t, "Thinking...", status.String())
	assert.True(t, status.IsInProgress)
	status.isPrimitive()
}

func TestPrimitive_BotProgressStatus(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAIBotProgressStatusPrimitive",
		"title": "In Progress...",
		"is_in_progress": true
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	status, ok := pc.Value.(*GenAIBotProgressStatusPrimitive)
	require.True(t, ok)
	assert.Equal(t, "In Progress...", status.String())
	status.isPrimitive()
}

func TestPrimitive_ProductItemCard(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAIProductItemCardPrimitive",
		"title": "Wireless Headphones"
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	card, ok := pc.Value.(*GenAIProductItemCardPrimitive)
	require.True(t, ok)
	assert.Equal(t, "Wireless Headphones", card.String())
	card.isPrimitive()
}

func TestPrimitive_Image(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAIImagePrimitive",
		"full_image": {
			"url": "https://img.example.com/full.png",
			"width": 1024,
			"height": 768,
			"mime_type": "image/png"
		}
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	img, ok := pc.Value.(*GenAIImagePrimitive)
	require.True(t, ok)
	assert.Equal(t, "", img.String())
	assert.Equal(t, "https://img.example.com/full.png", img.FullImage.URL)
	img.isPrimitive()
}

func TestPrimitive_Task(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAITaskPrimitive",
		"task_id": "123",
		"title": "Calendar Reminder",
		"subtitle": "Tomorrow at 9 AM",
		"status": "SCHEDULED"
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	task, ok := pc.Value.(*GenAITaskPrimitive)
	require.True(t, ok)
	assert.Equal(t, "Calendar Reminder", task.String())
	task.isPrimitive()
}

func TestPrimitive_Reel(t *testing.T) {
	data := []byte(`{"__typename": "GenAIReelPrimitive"}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	reel, ok := pc.Value.(*GenAIReelPrimitive)
	require.True(t, ok)
	assert.Equal(t, "", reel.String())
	reel.isPrimitive()
}

func TestPrimitive_Post(t *testing.T) {
	data := []byte(`{"__typename": "GenAIPostPrimitive"}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	post, ok := pc.Value.(*GenAIPostPrimitive)
	require.True(t, ok)
	assert.Equal(t, "", post.String())
	post.isPrimitive()
}

func TestPrimitive_Imagine(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAIImaginePrimitive",
		"imagine_type": "IMAGINE",
		"status": {
			"status": "READY",
			"update_text": "Image ready"
		}
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	imagine, ok := pc.Value.(*GenAIImaginePrimitive)
	require.True(t, ok)
	assert.Equal(t, "", imagine.String())
	assert.Equal(t, ImagineTypeImagine, imagine.ImagineType)
	assert.Equal(t, ImagineStatusReady, imagine.Status.Status)
	imagine.isPrimitive()
}

func TestPrimitive_SearchResult(t *testing.T) {
	data := []byte(`{"__typename": "GenAISearchResultPrimitive"}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	search, ok := pc.Value.(*GenAISearchResultPrimitive)
	require.True(t, ok)
	assert.Equal(t, "", search.String())
	search.isPrimitive()
}

func TestPrimitive_FOABloks(t *testing.T) {
	data := []byte(`{
		"__typename": "FOABloksPrimitive",
		"type": "native_bloks",
		"data": "payload",
		"uuid": "uuid-123"
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	bloks, ok := pc.Value.(*FOABloksPrimitive)
	require.True(t, ok)
	assert.Equal(t, "", bloks.String())
	assert.Equal(t, "uuid-123", bloks.UUID)
	bloks.isPrimitive()
}

func TestPrimitive_Divider(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAIDividerPrimitive",
		"type": "HORIZONTAL_LINE"
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	divider, ok := pc.Value.(*GenAIDividerPrimitive)
	require.True(t, ok)
	assert.Equal(t, "", divider.String())
	assert.Equal(t, DividerTypeHorizontalLine, divider.Type)
	divider.isPrimitive()
}

func TestPrimitive_Spacer(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAISpacerPrimitive",
		"spacing": 16
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	spacer, ok := pc.Value.(*GenAISpacerPrimitive)
	require.True(t, ok)
	assert.Equal(t, "", spacer.String())
	assert.Equal(t, 16, spacer.Spacing)
	spacer.isPrimitive()
}

func TestPrimitive_Unknown(t *testing.T) {
	data := []byte(`{
		"__typename": "GenAIUnknownFuturePrimitive",
		"custom": 123
	}`)
	var pc PrimitiveContainer
	err := json.Unmarshal(data, &pc)
	require.NoError(t, err)
	unk, ok := pc.Value.(UnknownPrimitive)
	require.True(t, ok)
	assert.Equal(t, "", unk.String())
	assert.Contains(t, string(unk), "GenAIUnknownFuturePrimitive")
	unk.isPrimitive()
}

func TestPrimitiveContainer_UnmarshalError(t *testing.T) {
	var pc PrimitiveContainer
	// Invalid JSON will cause unmarshalWithTypeName to return an error,
	// PrimitiveContainer.UnmarshalJSON catches err != nil and keeps Value nil, returning nil.
	err := pc.UnmarshalJSON([]byte(`{not valid`))
	assert.NoError(t, err)
	assert.Nil(t, pc.Value)
}
