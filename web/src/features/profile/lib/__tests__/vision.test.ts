/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { DEFAULT_VISION_PROMPT, normalizeVisionPrompt } from '../vision'

const expectedDefaultPrompt = `Describe the provided image as accurately and comprehensively as possible, based strictly on visible evidence.

Preserve all information that may be useful to another AI, including subjects, objects, appearance, actions, colors, materials, spatial relationships, layout, foreground/background, lighting, perspective, composition, UI elements, symbols, diagrams, charts, and visible text.

Transcribe readable text exactly. Mark unreadable text as [illegible].

Do not guess, invent, or infer unsupported identities, locations, relationships, intentions, events, brands, or hidden details. Explicitly mark uncertain observations as uncertain.

Prioritize factual accuracy, spatial relationships, and information preservation over elegant prose or brevity.`

describe('Vision prompt defaults', () => {
  test('uses the complete evidence-preserving default prompt', () => {
    assert.equal(DEFAULT_VISION_PROMPT, expectedDefaultPrompt)
    assert.equal(normalizeVisionPrompt(''), expectedDefaultPrompt)
  })

  test('preserves a non-blank custom prompt verbatim', () => {
    const customPrompt = '  Describe only the visible objects.  '
    assert.equal(normalizeVisionPrompt(customPrompt), customPrompt)
  })
})
