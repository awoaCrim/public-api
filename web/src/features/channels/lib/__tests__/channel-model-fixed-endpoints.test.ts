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
  CHANNEL_FORM_DEFAULT_VALUES,
  pruneChannelModelFixedEndpoints,
  transformFormDataToUpdatePayload,
  transformFormDataToCreatePayload,
} from '../channel-form'

describe('channel model fixed endpoints', () => {
  test('prune drops entries for removed models and blank endpoints', () => {
    const pruned = pruneChannelModelFixedEndpoints(
      {
        'gpt-4o': 'https://api.allowed.example.com',
        ghost: 'https://ghost.example.com',
        'gpt-4o-mini': '   ',
      },
      ['gpt-4o', 'gpt-4o-mini']
    )

    assert.deepEqual(pruned, {
      'gpt-4o': 'https://api.allowed.example.com',
    })
  })

  test('update payload carries only pruned fixed endpoints', () => {
    const payload = transformFormDataToUpdatePayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'channel',
        models: 'gpt-4o,claude',
        group: ['default'],
        model_fixed_endpoints: {
          'gpt-4o': 'https://api.allowed.example.com/',
          ghost: 'https://ghost.example.com',
        },
      },
      7
    )

    assert.deepEqual(payload.model_fixed_endpoints, {
      'gpt-4o': 'https://api.allowed.example.com/',
    })
  })

  test('update payload sends an empty object to clear all fixed endpoints', () => {
    const payload = transformFormDataToUpdatePayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'channel',
        models: 'gpt-4o',
        group: ['default'],
        model_fixed_endpoints: {},
      },
      8
    )

    assert.deepEqual(payload.model_fixed_endpoints, {})
  })

  test('create payload carries pruned fixed endpoints', () => {
    const result = transformFormDataToCreatePayload({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'channel',
      type: 1,
      models: 'gpt-4o',
      group: ['default'],
      model_fixed_endpoints: {
        'gpt-4o': 'https://api.allowed.example.com',
      },
    })

    assert.deepEqual(result.channel.model_fixed_endpoints, {
      'gpt-4o': 'https://api.allowed.example.com',
    })
  })
})
