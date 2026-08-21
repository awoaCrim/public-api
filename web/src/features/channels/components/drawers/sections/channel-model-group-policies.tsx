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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { ChannelModelGroupModeInput } from '@/features/channels/types'

const MODE_OPTIONS = [
  { value: 'inherit', labelKey: 'Inherit channel groups' },
  { value: 'custom', labelKey: 'Custom groups' },
  { value: 'disabled', labelKey: 'Disabled' },
] as const

type ChannelModelGroupPoliciesProps = {
  models: string[]
  groupOptions: { value: string; label: string }[]
  value: ChannelModelGroupModeInput[]
  onChange: (next: ChannelModelGroupModeInput[]) => void
  /** model -> fixed endpoint base URL; empty value removes the pin */
  fixedEndpoints?: Record<string, string>
  onFixedEndpointChange?: (model: string, endpoint: string) => void
  disabled?: boolean
}

/**
 * Per-model group publication tri-state: inherit (channel groups), custom
 * (explicit group set) or disabled (published to no group).
 */
export function ChannelModelGroupPolicies(
  props: ChannelModelGroupPoliciesProps
) {
  const { t } = useTranslation()

  const byModel = useMemo(() => {
    const map = new Map<string, ChannelModelGroupModeInput>()
    for (const mode of props.value) {
      map.set(mode.model, mode)
    }
    return map
  }, [props.value])

  const updateMode = (model: string, mode: string) => {
    const next = props.value.filter((item) => item.model !== model)
    if (mode === 'inherit') {
      props.onChange(next)
      return
    }
    const existing = byModel.get(model)
    props.onChange([
      ...next,
      {
        model,
        mode: mode as ChannelModelGroupModeInput['mode'],
        groups: existing?.groups ?? [],
      },
    ])
  }

  const updateGroups = (model: string, groups: string[]) => {
    const next = props.value.filter((item) => item.model !== model)
    if (groups.length === 0) {
      // An empty custom selection falls back to inherit.
      props.onChange(next)
      return
    }
    props.onChange([...next, { model, mode: 'custom', groups }])
  }

  const onSetFixedEndpoint = props.onFixedEndpointChange
  const showFixedEndpoint =
    props.fixedEndpoints !== undefined && onSetFixedEndpoint !== undefined

  if (props.models.length === 0) {
    return null
  }

  return (
    <div className='space-y-3'>
      {props.models.map((model) => {
        const current = byModel.get(model)
        const mode = current?.mode ?? 'inherit'
        return (
          <div key={model} className='border-border/60 rounded-md border p-3'>
            <div className='mb-2 flex items-center justify-between gap-2'>
              <span className='truncate font-mono text-sm'>{model}</span>
              <Select
                value={mode}
                onValueChange={(nextMode: string | null) =>
                  nextMode && updateMode(model, nextMode)
                }
                disabled={props.disabled}
              >
                <SelectTrigger className='w-44'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {MODE_OPTIONS.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {t(option.labelKey)}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
            {mode === 'custom' && (
              <MultiSelect
                options={props.groupOptions}
                selected={current?.groups ?? []}
                onChange={(groups) => updateGroups(model, groups)}
                placeholder={t('Select groups for this model')}
              />
            )}
            {showFixedEndpoint && (
              <div className='mt-2'>
                <Input
                  value={props.fixedEndpoints?.[model] ?? ''}
                  onChange={(e) =>
                    onSetFixedEndpoint(model, e.target.value)
                  }
                  placeholder={t(
                    'Fixed endpoint (leave empty for any endpoint)'
                  )}
                  disabled={props.disabled}
                  className='font-mono text-sm'
                />
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
