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
  pruneChannelModelGroupModes,
  transformFormDataToUpdatePayload,
} from '../channel-form'

describe('channel model group tri-state', () => {
  test('prune drops policies for models removed from the channel', () => {
    const pruned = pruneChannelModelGroupModes(
      [
        { model: 'gpt-4o', mode: 'custom', groups: ['vip'] },
        { model: 'ghost', mode: 'disabled', groups: [] },
      ],
      ['gpt-4o']
    )

    assert.deepEqual(pruned, [
      { model: 'gpt-4o', mode: 'custom', groups: ['vip'] },
    ])
  })

  test('update payload always carries the explicit policy set', () => {
    const payload = transformFormDataToUpdatePayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'channel',
        models: 'gpt-4o,claude',
        group: ['default'],
        model_group_modes: [{ model: 'claude', mode: 'disabled', groups: [] }],
      },
      7
    )

    assert.deepEqual(payload.model_group_modes, [
      { model: 'claude', mode: 'disabled', groups: [] },
    ])
  })

  test('update payload sends an empty array to clear all policies', () => {
    const payload = transformFormDataToUpdatePayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'channel',
        models: 'gpt-4o',
        group: ['default'],
        model_group_modes: [],
      },
      8
    )

    assert.deepEqual(payload.model_group_modes, [])
  })
})
