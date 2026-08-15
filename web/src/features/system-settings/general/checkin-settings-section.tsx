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
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

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
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const createSchema = (translate: (key: string) => string) =>
  z.object({
    enabled: z.boolean(),
    minQuota: z.coerce.number().int().min(0),
    maxQuota: z.coerce.number().int().min(0),
    balanceThresholdEnabled: z.boolean(),
    balanceThreshold: z.coerce
      .number()
      .finite()
      .refine((value) => value > 0, {
        message: translate('Check-in balance threshold must be greater than 0'),
      }),
  })

type Values = z.infer<ReturnType<typeof createSchema>>

export function CheckinSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    enabled: boolean
    minQuota: number
    maxQuota: number
    balanceThresholdEnabled: boolean
    balanceThreshold: number
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const schema = createSchema(t)

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      enabled: defaultValues.enabled,
      minQuota: defaultValues.minQuota,
      maxQuota: defaultValues.maxQuota,
      balanceThresholdEnabled: defaultValues.balanceThresholdEnabled,
      balanceThreshold: defaultValues.balanceThreshold,
    },
  })

  const { isDirty, isSubmitting } = form.formState
  const enabled = form.watch('enabled')
  const balanceThresholdEnabled = form.watch('balanceThresholdEnabled')
  const currencyDisplay = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const thresholdUnitLabel =
    currencyDisplay.config.quotaDisplayType === 'TOKENS' ? 'USD' : currencyLabel

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []

    if (values.enabled !== defaultValues.enabled) {
      updates.push({
        key: 'checkin_setting.enabled',
        value: String(values.enabled),
      })
    }

    if (values.minQuota !== defaultValues.minQuota) {
      updates.push({
        key: 'checkin_setting.min_quota',
        value: String(values.minQuota),
      })
    }

    if (values.maxQuota !== defaultValues.maxQuota) {
      updates.push({
        key: 'checkin_setting.max_quota',
        value: String(values.maxQuota),
      })
    }

    if (
      values.balanceThresholdEnabled !== defaultValues.balanceThresholdEnabled
    ) {
      updates.push({
        key: 'checkin_setting.balance_threshold_enabled',
        value: String(values.balanceThresholdEnabled),
      })
    }

    if (values.balanceThreshold !== defaultValues.balanceThreshold) {
      updates.push({
        key: 'checkin_setting.balance_threshold',
        value: String(values.balanceThreshold),
      })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    form.reset(values)
  }

  return (
    <SettingsSection title={t('Check-in Settings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save check-in settings'
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable check-in feature')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow users to check in daily for random quota rewards'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='balanceThresholdEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Enable check-in balance threshold')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Only users with a current balance below the threshold can check in'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled && (
            <div className='grid gap-6 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='minQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder={t('1000')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Minimum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='maxQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Maximum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder={t('10000')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Maximum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          )}

          {balanceThresholdEnabled && (
            <FormField
              control={form.control}
              name='balanceThreshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Check-in balance threshold')} ({thresholdUnitLabel})
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step='any'
                      placeholder='1'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {currencyDisplay.config.quotaDisplayType === 'TOKENS'
                      ? t('Token display mode interprets this threshold as USD')
                      : t(
                          'The saved value follows the current display currency and exchange rate'
                        )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
