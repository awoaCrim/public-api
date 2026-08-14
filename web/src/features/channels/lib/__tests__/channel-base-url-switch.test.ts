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

import {
  getBaseUrlForChannelTypeChange,
  getDefaultBaseUrl,
} from '../channel-type-config'

const VOLC_ENGINE_DEFAULT = 'https://ark.cn-beijing.volces.com'
const OPENCODE_GO_DEFAULT = 'https://opencode.ai'
const OPENAI_DEFAULT = 'https://api.openai.com'

describe('base URL on channel type change', () => {
  test('replaces configured defaults and preserves custom URLs', () => {
    const cases: Array<{
      name: string
      previousType: number
      nextType: number
      currentBaseUrl: string
      expected: string
    }> = [
      {
        name: 'empty VolcEngine URL switches to OpenCode Go default',
        previousType: 45,
        nextType: 61,
        currentBaseUrl: '',
        expected: OPENCODE_GO_DEFAULT,
      },
      {
        name: 'VolcEngine default switches to OpenCode Go default',
        previousType: 45,
        nextType: 61,
        currentBaseUrl: VOLC_ENGINE_DEFAULT,
        expected: OPENCODE_GO_DEFAULT,
      },
      {
        name: 'VolcEngine region choice is preserved',
        previousType: 45,
        nextType: 61,
        currentBaseUrl: 'https://ark.ap-southeast.bytepluses.com',
        expected: 'https://ark.ap-southeast.bytepluses.com',
      },
      {
        name: 'custom VolcEngine URL is preserved',
        previousType: 45,
        nextType: 61,
        currentBaseUrl: 'https://custom.example',
        expected: 'https://custom.example',
      },
      {
        name: 'OpenCode Go default switches to VolcEngine default',
        previousType: 61,
        nextType: 45,
        currentBaseUrl: OPENCODE_GO_DEFAULT,
        expected: VOLC_ENGINE_DEFAULT,
      },
      {
        name: 'custom URL is preserved when switching to VolcEngine',
        previousType: 61,
        nextType: 45,
        currentBaseUrl: 'https://custom.example',
        expected: 'https://custom.example',
      },
      {
        name: 'existing OpenAI default is still replaced',
        previousType: 1,
        nextType: 61,
        currentBaseUrl: OPENAI_DEFAULT,
        expected: OPENCODE_GO_DEFAULT,
      },
      {
        name: 'OpenCode Go default switches back to OpenAI default',
        previousType: 61,
        nextType: 1,
        currentBaseUrl: OPENCODE_GO_DEFAULT,
        expected: OPENAI_DEFAULT,
      },
    ]

    for (const testCase of cases) {
      assert.equal(
        getBaseUrlForChannelTypeChange(
          testCase.previousType,
          testCase.nextType,
          testCase.currentBaseUrl
        ),
        testCase.expected,
        testCase.name
      )
    }
  })

  test('registers the VolcEngine default base URL', () => {
    assert.equal(getDefaultBaseUrl(45), VOLC_ENGINE_DEFAULT)
  })
})
