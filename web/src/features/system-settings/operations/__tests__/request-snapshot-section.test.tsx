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

import type { TFunction } from 'i18next'
import { isValidElement } from 'react'

import { LogSettingsSection } from '../../maintenance/log-settings-section'
import { RequestSnapshotSettingsSection } from '../../maintenance/request-snapshot-settings-section'
import type { OperationsSettings } from '../../types'
import {
  OPERATIONS_SECTION_IDS,
  getOperationsSectionContent,
  getOperationsSectionNavItems,
} from '../section-registry'

const settings = {
  LogConsumeEnabled: true,
  'request_snapshot_setting.enabled': false,
  'request_snapshot_setting.storage_path': './request_snapshots',
  'request_snapshot_setting.max_body_mb': 10,
  'request_snapshot_setting.max_total_mb': 1024,
  'request_snapshot_setting.retention_days': 30,
  'request_snapshot_setting.cleanup_interval_hours': 24,
  'request_snapshot_setting.orphan_grace_minutes': 60,
} as OperationsSettings

const identityTranslation = ((key: string) => key) as TFunction

describe('operations request snapshot section', () => {
  test('exposes a dedicated Request Snapshots navigation route', () => {
    assert.equal(
      OPERATIONS_SECTION_IDS.filter((id) => id === 'request-snapshots').length,
      1
    )

    const navItems = getOperationsSectionNavItems(identityTranslation)
    assert.deepEqual(
      navItems.find((item) => item.url.endsWith('/request-snapshots')),
      {
        title: 'Request Snapshots',
        url: '/system-settings/operations/request-snapshots',
      }
    )
  })

  test('renders the snapshot form only in the dedicated section', () => {
    const logContent = getOperationsSectionContent(
      'logs',
      settings,
      undefined,
      undefined
    )
    const snapshotContent = getOperationsSectionContent(
      'request-snapshots',
      settings,
      undefined,
      undefined
    )

    assert.ok(isValidElement(logContent))
    assert.equal(logContent.type, LogSettingsSection)

    assert.ok(isValidElement(snapshotContent))
    assert.equal(snapshotContent.type, RequestSnapshotSettingsSection)
  })
})
