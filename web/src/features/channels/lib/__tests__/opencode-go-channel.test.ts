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
  CHANNEL_TYPE_NEW_API,
  CHANNEL_TYPE_OPENCODE_GO,
  CHANNEL_TYPE_OPTIONS,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'
import {
  getBaseUrlForChannelTypeChange,
  getChannelTypeConfig,
  getDefaultBaseUrl,
} from '../channel-type-config'
import { getChannelTypeIcon, getKeyPromptForType } from '../channel-utils'

describe('OpenCode Go channel', () => {
  test('registers type, ordering, model discovery, icon, and defaults', () => {
    assert.equal(CHANNEL_TYPE_NEW_API, 60)
    assert.equal(CHANNEL_TYPE_OPENCODE_GO, 61)
    assert.deepEqual(
      CHANNEL_TYPE_OPTIONS.find(
        (option) => option.value === CHANNEL_TYPE_OPENCODE_GO
      ),
      { value: CHANNEL_TYPE_OPENCODE_GO, label: 'OpenCode Go' }
    )
    assert.equal(
      CHANNEL_TYPE_OPTIONS.findIndex(
        (option) => option.value === CHANNEL_TYPE_OPENCODE_GO
      ),
      CHANNEL_TYPE_OPTIONS.findIndex(
        (option) => option.value === CHANNEL_TYPE_NEW_API
      ) + 1
    )
    assert.equal(MODEL_FETCHABLE_TYPES.has(CHANNEL_TYPE_OPENCODE_GO), true)
    assert.equal(getChannelTypeIcon(CHANNEL_TYPE_OPENCODE_GO), 'OpenAI')
    assert.equal(
      getKeyPromptForType(CHANNEL_TYPE_OPENCODE_GO),
      'Enter API key for this channel'
    )
    assert.deepEqual(getChannelTypeConfig(CHANNEL_TYPE_OPENCODE_GO), {
      id: CHANNEL_TYPE_OPENCODE_GO,
      name: 'OpenCode Go',
      icon: 'openai',
      defaultBaseUrl: 'https://opencode.ai',
      hints: {
        baseUrl: 'Default: https://opencode.ai',
        key: 'Enter API key for this channel',
        models: 'Models fetched from upstream /v1/models',
      },
    })
    assert.equal(
      getDefaultBaseUrl(CHANNEL_TYPE_OPENCODE_GO),
      'https://opencode.ai'
    )
  })

  test('applies and replaces configured defaults without overwriting custom URLs', () => {
    assert.equal(
      getBaseUrlForChannelTypeChange(1, CHANNEL_TYPE_OPENCODE_GO, ''),
      'https://opencode.ai'
    )
    assert.equal(
      getBaseUrlForChannelTypeChange(
        CHANNEL_TYPE_OPENCODE_GO,
        1,
        'https://opencode.ai'
      ),
      'https://api.openai.com'
    )
    assert.equal(
      getBaseUrlForChannelTypeChange(
        CHANNEL_TYPE_OPENCODE_GO,
        1,
        'https://custom.example'
      ),
      'https://custom.example'
    )
  })
})
