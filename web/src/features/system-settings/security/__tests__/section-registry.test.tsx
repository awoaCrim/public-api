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
import { isValidElement, type ComponentProps } from 'react'

import { ReviewTriggerLimitsSection } from '../../request-limits/review-trigger-limits-section'
import type { SecuritySettings } from '../../types'
import {
  SECURITY_SECTION_IDS,
  getSecuritySectionContent,
  getSecuritySectionNavItems,
} from '../section-registry'

const settings = {
  ModelRequestRateLimitEnabled: false,
  ModelRequestRateLimitCount: 0,
  ModelRequestRateLimitSuccessCount: 1000,
  ModelRequestRateLimitDurationMinutes: 1,
  ModelRequestRateLimitGroup: '',
  CheckSensitiveEnabled: false,
  CheckSensitiveOnPromptEnabled: false,
  SensitiveWords: '',
  'fetch_setting.enable_ssrf_protection': true,
  'fetch_setting.allow_private_ip': false,
  'fetch_setting.domain_filter_mode': false,
  'fetch_setting.ip_filter_mode': false,
  'fetch_setting.domain_list': [],
  'fetch_setting.ip_list': [],
  'fetch_setting.allowed_ports': [],
  'fetch_setting.apply_ip_filter_for_domain': false,
  'token_setting.max_user_tokens': 1000,
  'rate_limit_ban_setting.enabled': true,
  'rate_limit_ban_setting.max_rpm': 12,
  'rate_limit_ban_setting.max_input_tokens': 200000,
  'rate_limit_ban_setting.max_output_tokens': 10000,
  'rate_limit_ban_setting.whitelist_models': ['embedding-*'],
} as SecuritySettings

const identityTranslation = ((key: string) => key) as TFunction

describe('security review trigger limits section', () => {
  test('exposes the threshold settings in the Security & Limits navigation', () => {
    assert.equal(
      SECURITY_SECTION_IDS.filter((id) => id === 'review-trigger-limits')
        .length,
      1
    )

    const navItems = getSecuritySectionNavItems(identityTranslation)
    assert.deepEqual(
      navItems.find((item) => item.url.endsWith('/review-trigger-limits')),
      {
        title: 'LLM Review Trigger Limits',
        url: '/system-settings/security/review-trigger-limits',
      }
    )
  })

  test('passes all rate-limit review options to the visible section', () => {
    const content = getSecuritySectionContent('review-trigger-limits', settings)

    assert.ok(isValidElement(content))
    assert.equal(content.type, ReviewTriggerLimitsSection)
    const props = content.props as ComponentProps<
      typeof ReviewTriggerLimitsSection
    >
    assert.deepEqual(props.defaultValues, {
      'rate_limit_ban_setting.enabled': true,
      'rate_limit_ban_setting.max_rpm': 12,
      'rate_limit_ban_setting.max_input_tokens': 200000,
      'rate_limit_ban_setting.max_output_tokens': 10000,
      'rate_limit_ban_setting.whitelist_models': ['embedding-*'],
    })
  })
})
