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
  canViewRequestSnapshot,
  decodeSnapshotContent,
  snapshotBytesToText,
  snapshotErrorKey,
  snapshotFileName,
  type RequestSnapshotPayload,
  type SnapshotViewerUser,
} from '../request-snapshot'

function adminWith(
  admin_permissions: Record<string, Record<string, boolean>>
): SnapshotViewerUser {
  return {
    id: 1,
    username: 'admin',
    role: 10,
    permissions: {
      admin_permissions,
    },
  }
}

function snapshotPayload(
  overrides: Partial<RequestSnapshotPayload> = {}
): RequestSnapshotPayload {
  return {
    request_id: 'req-1',
    content_type: 'application/json',
    size: 27,
    content_base64: 'eyJtZXNzYWdlcyI6W119', // {"messages":[]}
    ...overrides,
  }
}

describe('request snapshot gating', () => {
  test('requires admin, a request id, and the request_snapshot.read permission', () => {
    const granted = adminWith({ request_snapshot: { read: true } })
    const ungranted = adminWith({ request_snapshot: { read: false } })

    assert.equal(canViewRequestSnapshot(granted, true, 'req-1'), true)
    assert.equal(
      canViewRequestSnapshot(granted, false, 'req-1'),
      false,
      'non-admin is never allowed'
    )
    assert.equal(
      canViewRequestSnapshot(granted, true, ''),
      false,
      'missing request id is never allowed'
    )
    assert.equal(canViewRequestSnapshot(granted, true, null), false)
    assert.equal(
      canViewRequestSnapshot(ungranted, true, 'req-1'),
      false,
      'missing permission is never allowed'
    )
    assert.equal(
      canViewRequestSnapshot(null, true, 'req-1'),
      false,
      'anonymous user is never allowed'
    )
  })

  test('root superuser is implicitly allowed without an explicit grant', () => {
    const root = adminWith({}) as SnapshotViewerUser
    root.role = 100
    assert.equal(canViewRequestSnapshot(root, true, 'req-1'), true)
  })

  test('unknown users or users without permissions are denied', () => {
    assert.equal(
      canViewRequestSnapshot({ id: 2, username: 'u', role: 10 }, true, 'req-1'),
      false
    )
  })
})

describe('request snapshot payload decoding', () => {
  test('decodes exact bytes and renders text losslessly', () => {
    const payload = snapshotPayload({
      content_base64: 'eyJtb2RlbCI6ImdwdC00byJ9', // {"model":"gpt-4o"}
    })
    const bytes = decodeSnapshotContent(payload)
    assert.equal(snapshotBytesToText(bytes), '{"model":"gpt-4o"}')
  })

  test('preserves arbitrary binary bytes exactly', () => {
    const raw = new Uint8Array([0x00, 0x01, 0xff, 0xfe, 0x80])
    let binary = ''
    for (const byte of raw) binary += String.fromCharCode(byte)
    const payload = snapshotPayload({ content_base64: btoa(binary) })
    const decoded = decodeSnapshotContent(payload)
    assert.deepEqual([...decoded], [...raw])
  })

  test('maps stable backend error codes to i18n keys', () => {
    assert.equal(snapshotErrorKey('SNAPSHOT_NOT_FOUND'), 'Snapshot not found')
    assert.equal(snapshotErrorKey('SNAPSHOT_DELETED'), 'Snapshot deleted')
    assert.equal(snapshotErrorKey('SNAPSHOT_MISSING'), 'Snapshot file missing')
    assert.equal(
      snapshotErrorKey('SNAPSHOT_UNAVAILABLE'),
      'Snapshot unavailable'
    )
    assert.equal(snapshotErrorKey('SNAPSHOT_CORRUPT'), 'Snapshot corrupt')
    assert.equal(
      snapshotErrorKey('SNAPSHOT_WRONG_NODE'),
      'Snapshot stored on another node'
    )
    assert.equal(
      snapshotErrorKey('SNAPSHOT_AUDIT_FAILED'),
      'Snapshot access could not be audited'
    )
    assert.equal(
      snapshotErrorKey('SNAPSHOT_READ_FAILED'),
      'Snapshot could not be read'
    )
    assert.equal(snapshotErrorKey('UNKNOWN'), null)
    assert.equal(snapshotErrorKey(undefined), null)
  })

  test('builds safe download file names', () => {
    assert.equal(snapshotFileName('req-123'), 'req-123-body.txt')
    assert.equal(snapshotFileName('../etc/passwd'), '.._etc_passwd-body.txt')
    assert.equal(snapshotFileName(''), 'request-body.txt')
  })
})
