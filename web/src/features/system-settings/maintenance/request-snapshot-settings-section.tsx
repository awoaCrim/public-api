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
import { zodResolver } from '@hookform/resolvers/zod'
import { useCallback, useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

/**
 * NOTE: react-hook-form 7 treats dotted `name` strings as nested paths, so the
 * form models the settings as a nested object and flattens back to the
 * server-side `request_snapshot_setting.*` key format only before persisting.
 */
const requestSnapshotSchema = z.object({
  request_snapshot_setting: z.object({
    enabled: z.boolean(),
    storage_path: z.string().min(1),
    max_body_mb: z.number().int().positive(),
    max_total_mb: z.number().int().positive(),
    retention_days: z.number().int().positive(),
    cleanup_interval_hours: z.number().int().positive(),
    orphan_grace_minutes: z.number().int().positive(),
  }),
})

type SnapshotFormInput = z.input<typeof requestSnapshotSchema>
type SnapshotFormValues = z.output<typeof requestSnapshotSchema>

export type FlatSnapshotDefaults = {
  'request_snapshot_setting.enabled': boolean
  'request_snapshot_setting.storage_path': string
  'request_snapshot_setting.max_body_mb': number
  'request_snapshot_setting.max_total_mb': number
  'request_snapshot_setting.retention_days': number
  'request_snapshot_setting.cleanup_interval_hours': number
  'request_snapshot_setting.orphan_grace_minutes': number
}

const buildFormDefaults = (
  defaults: FlatSnapshotDefaults
): SnapshotFormInput => ({
  request_snapshot_setting: {
    enabled: defaults['request_snapshot_setting.enabled'],
    storage_path: defaults['request_snapshot_setting.storage_path'] ?? '',
    max_body_mb: defaults['request_snapshot_setting.max_body_mb'],
    max_total_mb: defaults['request_snapshot_setting.max_total_mb'],
    retention_days: defaults['request_snapshot_setting.retention_days'],
    cleanup_interval_hours:
      defaults['request_snapshot_setting.cleanup_interval_hours'],
    orphan_grace_minutes:
      defaults['request_snapshot_setting.orphan_grace_minutes'],
  },
})

const normalizeFormValues = (
  values: SnapshotFormValues
): FlatSnapshotDefaults => ({
  'request_snapshot_setting.enabled': values.request_snapshot_setting.enabled,
  'request_snapshot_setting.storage_path':
    values.request_snapshot_setting.storage_path,
  'request_snapshot_setting.max_body_mb':
    values.request_snapshot_setting.max_body_mb,
  'request_snapshot_setting.max_total_mb':
    values.request_snapshot_setting.max_total_mb,
  'request_snapshot_setting.retention_days':
    values.request_snapshot_setting.retention_days,
  'request_snapshot_setting.cleanup_interval_hours':
    values.request_snapshot_setting.cleanup_interval_hours,
  'request_snapshot_setting.orphan_grace_minutes':
    values.request_snapshot_setting.orphan_grace_minutes,
})

interface Props {
  defaultValues: FlatSnapshotDefaults
}

export function RequestSnapshotSettingsSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<SnapshotFormInput, unknown, SnapshotFormValues>({
    resolver: zodResolver(requestSnapshotSchema),
    defaultValues: formDefaults,
  })

  const baselineRef = useRef<FlatSnapshotDefaults>(props.defaultValues)
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(props.defaultValues)
  )

  useEffect(() => {
    const serialized = JSON.stringify(props.defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = props.defaultValues
    baselineSerializedRef.current = serialized
    form.reset(buildFormDefaults(props.defaultValues))
  }, [props.defaultValues, form])

  const enabled = form.watch('request_snapshot_setting.enabled')

  const onSubmit = useCallback(
    async (values: SnapshotFormValues) => {
      const normalized = normalizeFormValues(values)
      const changedKeys = (
        Object.keys(normalized) as Array<keyof FlatSnapshotDefaults>
      ).filter((key) => normalized[key] !== baselineRef.current[key])

      if (changedKeys.length === 0) {
        toast.info(t('No changes to save'))
        return
      }

      for (const key of changedKeys) {
        await updateOption.mutateAsync({
          key,
          value: normalized[key],
        })
      }

      baselineRef.current = normalized
      baselineSerializedRef.current = JSON.stringify(normalized)
      form.reset(buildFormDefaults(normalized))
    },
    [form, t, updateOption]
  )

  return (
    <SettingsSection title={t('Request Snapshots')}>
      <Alert>
        <AlertDescription>
          {t(
            'Captures the complete body of authenticated relay requests for troubleshooting. Disabled by default; requires CRYPTO_SECRET or SESSION_SECRET to operate and request_snapshot.read permission to view.'
          )}
        </AlertDescription>
      </Alert>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <FormField
            control={form.control}
            name='request_snapshot_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Request Snapshots')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When enabled, every authenticated well-formed relay request body is encrypted and stored on the node that handled it.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='request_snapshot_setting.storage_path'
            render={({ field }) => (
              <FormItem className='max-w-md'>
                <FormLabel>{t('Snapshot Storage Path')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='./request_snapshots'
                    {...field}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Base directory; each node stores its own encrypted files in a per-node subdirectory.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='request_snapshot_setting.max_body_mb'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Max Body Size (MB)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Requests larger than this are recorded as failed and never partially saved.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='request_snapshot_setting.max_total_mb'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Max Total Size (MB)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Per-node capacity; concurrent captures never exceed it and cleanup evicts the oldest files first.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='request_snapshot_setting.retention_days'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Retention (days)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Snapshots and audit records older than this are removed by the node-local cleanup.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='request_snapshot_setting.cleanup_interval_hours'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Cleanup Interval (hours)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'How often each node runs its maintenance pass (retention, capacity, orphans).'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='request_snapshot_setting.orphan_grace_minutes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Orphan Grace (minutes)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Ownerless files younger than this are kept; older ones are removed by cleanup.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={() => form.reset(buildFormDefaults(props.defaultValues))}
            isSaving={updateOption.isPending}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
