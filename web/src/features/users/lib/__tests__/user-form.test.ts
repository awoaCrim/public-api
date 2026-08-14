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
  transformFormDataToPayload,
  transformUserToFormDefaults,
} from '../user-form'

describe('user extra group grants', () => {
  test('update always sends the explicit grant set (presence semantics)', () => {
    const payload = transformFormDataToPayload(
      {
        username: 'user',
        display_name: 'User',
        password: '',
        role: 1,
        quota_dollars: 0,
        group: 'default',
        remark: '',
        admin_permissions: {},
        extra_group_keys: ['svip'],
      },
      42
    )

    assert.deepEqual(payload.extra_group_keys, ['svip'])
  })

  test('update sends an empty array to clear all manual grants', () => {
    const payload = transformFormDataToPayload(
      {
        username: 'user',
        display_name: 'User',
        password: '',
        role: 1,
        quota_dollars: 0,
        group: 'default',
        remark: '',
        admin_permissions: {},
        extra_group_keys: [],
      },
      42
    )

    assert.deepEqual(payload.extra_group_keys, [])
  })

  test('create omits the grant field entirely', () => {
    const payload = transformFormDataToPayload({
      username: 'user',
      display_name: 'User',
      password: '',
      role: 1,
      quota_dollars: 0,
      group: 'default',
      remark: '',
      admin_permissions: {},
      extra_group_keys: ['svip'],
    })

    assert.equal('extra_group_keys' in payload, false)
  })

  test('form defaults load existing grant keys', () => {
    const defaults = transformUserToFormDefaults({
      id: 42,
      username: 'user',
      display_name: 'User',
      password: '',
      quota: 0,
      used_quota: 0,
      request_count: 0,
      group: 'default',
      status: 1,
      role: 1,
      extra_group_keys: ['svip', 'vip'],
    } as never)

    assert.deepEqual(defaults.extra_group_keys, ['svip', 'vip'])
  })
})
