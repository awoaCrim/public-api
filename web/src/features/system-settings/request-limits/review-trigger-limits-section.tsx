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
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

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
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const reviewTriggerLimitsSchema = z.object({
  enabled: z.boolean(),
  maxRPM: z.number().int().min(0).max(2147483647),
  maxInputTokens: z.number().int().min(0).max(2147483647),
  maxOutputTokens: z.number().int().min(0).max(2147483647),
  whitelistModels: z.string(),
})

type ReviewTriggerLimitsFormValues = z.infer<typeof reviewTriggerLimitsSchema>

type ReviewTriggerLimitsSettings = {
  'rate_limit_ban_setting.enabled': boolean
  'rate_limit_ban_setting.max_rpm': number
  'rate_limit_ban_setting.max_input_tokens': number
  'rate_limit_ban_setting.max_output_tokens': number
  'rate_limit_ban_setting.whitelist_models': string[]
}

type ReviewTriggerLimitsSectionProps = {
  defaultValues: ReviewTriggerLimitsSettings
}

const buildFormDefaults = (
  defaults: ReviewTriggerLimitsSettings
): ReviewTriggerLimitsFormValues => ({
  enabled: defaults['rate_limit_ban_setting.enabled'],
  maxRPM: defaults['rate_limit_ban_setting.max_rpm'],
  maxInputTokens: defaults['rate_limit_ban_setting.max_input_tokens'],
  maxOutputTokens: defaults['rate_limit_ban_setting.max_output_tokens'],
  whitelistModels:
    defaults['rate_limit_ban_setting.whitelist_models'].join('\n'),
})

const normalizeWhitelistModels = (value: string) =>
  value
    .split(/\r?\n/)
    .map((model) => model.trim())
    .filter(Boolean)

export function ReviewTriggerLimitsSection({
  defaultValues,
}: ReviewTriggerLimitsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )
  const form = useForm<ReviewTriggerLimitsFormValues>({
    resolver: zodResolver(reviewTriggerLimitsSchema),
    mode: 'onChange',
    defaultValues: formDefaults,
  })

  useEffect(() => {
    form.reset(formDefaults)
  }, [form, formDefaults])

  const onSubmit = async (values: ReviewTriggerLimitsFormValues) => {
    const whitelistModels = normalizeWhitelistModels(values.whitelistModels)
    const defaultWhitelistModels = normalizeWhitelistModels(
      defaultValues['rate_limit_ban_setting.whitelist_models'].join('\n')
    )
    const updates: Array<{
      key: keyof ReviewTriggerLimitsSettings
      value: string | boolean | number
    }> = []

    if (values.enabled !== defaultValues['rate_limit_ban_setting.enabled']) {
      updates.push({
        key: 'rate_limit_ban_setting.enabled',
        value: values.enabled,
      })
    }
    if (values.maxRPM !== defaultValues['rate_limit_ban_setting.max_rpm']) {
      updates.push({
        key: 'rate_limit_ban_setting.max_rpm',
        value: values.maxRPM,
      })
    }
    if (
      values.maxInputTokens !==
      defaultValues['rate_limit_ban_setting.max_input_tokens']
    ) {
      updates.push({
        key: 'rate_limit_ban_setting.max_input_tokens',
        value: values.maxInputTokens,
      })
    }
    if (
      values.maxOutputTokens !==
      defaultValues['rate_limit_ban_setting.max_output_tokens']
    ) {
      updates.push({
        key: 'rate_limit_ban_setting.max_output_tokens',
        value: values.maxOutputTokens,
      })
    }
    if (
      JSON.stringify(whitelistModels) !== JSON.stringify(defaultWhitelistModels)
    ) {
      updates.push({
        key: 'rate_limit_ban_setting.whitelist_models',
        value: JSON.stringify(whitelistModels),
      })
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  return (
    <SettingsSection title={t('LLM Review Trigger Limits')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save review trigger limits'
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Controls the per-user RPM and per-request token thresholds that create LLM review events.'
            )}
          </p>

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable review trigger limits')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When enabled, exceeded limits enqueue an LLM review; they do not directly ban users.'
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

          <div className='grid gap-4 md:grid-cols-3'>
            <FormField
              control={form.control}
              name='maxRPM'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Maximum requests per minute')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={2147483647}
                      step={1}
                      {...field}
                      onChange={(event) =>
                        field.onChange(
                          Number.parseInt(event.target.value, 10) || 0
                        )
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Per-user request rate limit.')}{' '}
                    {t('0 disables this threshold.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='maxInputTokens'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Maximum input tokens per request')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={2147483647}
                      step={1}
                      {...field}
                      onChange={(event) =>
                        field.onChange(
                          Number.parseInt(event.target.value, 10) || 0
                        )
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Pre-request input token limit.')}{' '}
                    {t('0 disables this threshold.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='maxOutputTokens'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Maximum output tokens per request')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={2147483647}
                      step={1}
                      {...field}
                      onChange={(event) =>
                        field.onChange(
                          Number.parseInt(event.target.value, 10) || 0
                        )
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Post-request output token limit.')}{' '}
                    {t('0 disables this threshold.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='whitelistModels'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Model whitelist')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={5}
                    placeholder={t('One model pattern per line')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Use * at the end for a prefix match. Leave blank to apply to all models.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
