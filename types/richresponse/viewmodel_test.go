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

func TestViewModelContainer_UnmarshalJSON(t *testing.T) {
	t.Run("Invalid outer JSON", func(t *testing.T) {
		var vmc ViewModelContainer
		err := vmc.UnmarshalJSON([]byte(`{invalid`))
		require.Error(t, err)
	})

	t.Run("Invalid inner view_model JSON", func(t *testing.T) {
		var vmc ViewModelContainer
		err := vmc.UnmarshalJSON([]byte(`{"view_model": 123}`))
		require.NoError(t, err)
		assert.Nil(t, vmc.Model)
	})

	t.Run("Unknown ViewModel", func(t *testing.T) {
		data := []byte(`{
			"view_model": {
				"__typename": "UnknownLayoutViewModel",
				"data": "xyz"
			}
		}`)
		var vmc ViewModelContainer
		err := json.Unmarshal(data, &vmc)
		require.NoError(t, err)
		unk, ok := vmc.Model.(UnknownViewModel)
		require.True(t, ok)
		assert.Equal(t, "", unk.String())
		assert.Nil(t, unk.GetPrimitives())
		unk.isViewModel()
	})
}

func TestGenAISingleLayoutViewModel(t *testing.T) {
	data := []byte(`{
		"view_model": {
			"__typename": "GenAISingleLayoutViewModel",
			"primitive": {
				"__typename": "GenAIMetadataTextPrimitive",
				"text": "Single Layout Content"
			}
		}
	}`)
	var vmc ViewModelContainer
	err := json.Unmarshal(data, &vmc)
	require.NoError(t, err)

	single, ok := vmc.Model.(*GenAISingleLayoutViewModel)
	require.True(t, ok)
	assert.Equal(t, "GenAISingleLayoutViewModel", single.TypeName)
	assert.Equal(t, "Single Layout Content", single.String())

	prims := single.GetPrimitives()
	require.Len(t, prims, 1)
	assert.Equal(t, "Single Layout Content", prims[0].String())
	single.isViewModel()
}

func TestMultiLayoutViewModel(t *testing.T) {
	types := []string{
		"GenAIGridLayoutViewModel",
		"GenAIHScrollLayoutViewModel",
		"GenAIVStackLayoutViewModel",
	}

	for _, typeName := range types {
		t.Run(typeName, func(t *testing.T) {
			data := []byte(`{
				"view_model": {
					"__typename": "` + typeName + `",
					"primitives": [
						{
							"__typename": "GenAIMetadataTextPrimitive",
							"text": "Item 1"
						},
						{
							"__typename": "GenAIMetadataTextPrimitive",
							"text": "Item 2"
						}
					]
				}
			}`)
			var vmc ViewModelContainer
			err := json.Unmarshal(data, &vmc)
			require.NoError(t, err)

			multi, ok := vmc.Model.(*MultiLayoutViewModel)
			require.True(t, ok)
			assert.Equal(t, typeName, multi.TypeName)
			assert.Equal(t, "Item 1\nItem 2", multi.String())

			prims := multi.GetPrimitives()
			require.Len(t, prims, 2)
			assert.Equal(t, "Item 1", prims[0].String())
			assert.Equal(t, "Item 2", prims[1].String())
			multi.isViewModel()
		})
	}
}

func TestRichResponse_Unmarshal(t *testing.T) {
	data := []byte(`{
		"response_id": "resp-12345",
		"sections": [
			{
				"view_model": {
					"__typename": "GenAISingleLayoutViewModel",
					"primitive": {
						"__typename": "GenAIMetadataTextPrimitive",
						"text": "Section Header"
					}
				}
			}
		],
		"footer_sections": [
			{
				"view_model": {
					"__typename": "GenAISingleLayoutViewModel",
					"primitive": {
						"__typename": "GenAIMetadataTextPrimitive",
						"text": "Footer Info"
					}
				}
			}
		],
		"embedded_screens": [{"screen": "main"}]
	}`)

	var rr RichResponse
	err := json.Unmarshal(data, &rr)
	require.NoError(t, err)
	assert.Equal(t, "resp-12345", rr.ResponseID)
	require.Len(t, rr.Sections, 1)
	assert.Equal(t, "Section Header", rr.Sections[0].Model.String())
	require.Len(t, rr.FooterSections, 1)
	assert.Equal(t, "Footer Info", rr.FooterSections[0].Model.String())
	require.Len(t, rr.EmbeddedScreens, 1)
}
