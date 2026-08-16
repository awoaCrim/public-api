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
import type { TFunction } from 'i18next'

import type {
  ReviewCategory,
  ReviewTaskStatus,
  ReviewTriggerType,
  StructuredOutputMode,
} from '../types'

/** Task status -> i18n label key. */
export const REVIEW_STATUS_LABEL_KEYS: Record<ReviewTaskStatus, string> = {
  pending: 'Pending',
  reviewing: 'Reviewing',
  compliant: 'Compliant',
  violation: 'Violation',
  uncertain: 'Uncertain',
  skipped: 'Skipped',
  failed: 'Failed',
  superseded: 'Superseded',
}

/** Trigger type -> i18n label key. */
export const REVIEW_TRIGGER_LABEL_KEYS: Record<ReviewTriggerType, string> = {
  rpm: 'RPM',
  input_token: 'Input Tokens',
  output_token: 'Output Tokens',
}

/** Violation category -> i18n label key. */
export const REVIEW_CATEGORY_LABEL_KEYS: Record<ReviewCategory, string> = {
  commercial_use: 'Commercial Use',
  account_sharing: 'Account Sharing',
  unauthorized_client: 'Unauthorized Client',
  stress_test: 'Stress Test',
  abnormal_automation: 'Abnormal Automation',
  limit_bypass: 'Limit Bypass',
  harmful_resource_use: 'Harmful Resource Use',
  code_generation: 'Code Generation',
  other: 'Other',
  none: 'None',
}

export function getReviewStatusLabel(
  status: ReviewTaskStatus,
  t: TFunction
): string {
  return t(REVIEW_STATUS_LABEL_KEYS[status] ?? status)
}

export function getReviewTriggerLabel(
  trigger: ReviewTriggerType,
  t: TFunction
): string {
  return t(REVIEW_TRIGGER_LABEL_KEYS[trigger] ?? trigger)
}

export function getReviewCategoryLabel(
  category: ReviewCategory | '' | undefined,
  t: TFunction
): string {
  if (!category) return '-'
  return t(REVIEW_CATEGORY_LABEL_KEYS[category] ?? category)
}

/** Structured-output mode -> i18n label key. */
export const REVIEW_OUTPUT_MODE_LABEL_KEYS: Record<
  StructuredOutputMode,
  string
> = {
  strict_schema: 'Strict JSON schema',
  json_object: 'JSON object compatibility',
  prompt_json: 'Prompt-only JSON compatibility',
}

export function getReviewOutputModeLabel(
  mode: StructuredOutputMode | '' | undefined,
  t: TFunction
): string {
  if (!mode) return '-'
  return t(REVIEW_OUTPUT_MODE_LABEL_KEYS[mode] ?? mode)
}

/** Recent (24h) failure rate display percentage. */
export function formatFailureRate(rate: number | undefined): string {
  if (rate == null || !Number.isFinite(rate)) return '-'
  return `${(rate * 100).toFixed(1)}%`
}

/** Oldest waiting seconds in human-readable form. */
export function formatWaitingSeconds(seconds: number | undefined): string {
  if (seconds == null || seconds < 0) return '-'
  if (seconds < 60) return `${Math.floor(seconds)}s`
  if (seconds < 3600) {
    return `${Math.floor(seconds / 60)}m ${Math.floor(seconds % 60)}s`
  }
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}

/** Unix timestamp -> locale string ('-' when absent). */
export function formatReviewTime(ts: number): string {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString()
}
