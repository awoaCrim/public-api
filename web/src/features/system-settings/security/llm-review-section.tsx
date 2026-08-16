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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  ListChecks,
  Loader2,
  PlugZap,
  ShieldCheck,
  Trash2,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  clearLLMReviewApiKey,
  getLLMReviewConfig,
  getLLMReviewQueueSummary,
  getLLMReviewSchemaStatus,
  testLLMReviewConnection,
  testLLMReviewSchema,
  updateLLMReviewConfig,
} from '@/features/llm-review/api'
import {
  formatFailureRate,
  formatWaitingSeconds,
  getReviewOutputModeLabel,
} from '@/features/llm-review/lib/format'

import {
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'

const schema = z.object({
  enabled: z.boolean(),
  baseUrl: z.string(),
  apiKey: z.string(),
  model: z.string(),
  policyText: z.string(),
  timeoutSeconds: z.coerce.number().int().min(1),
  maxAttempts: z.coerce.number().int().min(1).max(10),
  retryIntervalSeconds: z.coerce.number().int().min(1),
  workerConcurrency: z.coerce.number().int().min(1).max(20),
  confidenceThreshold: z.coerce.number().min(0).max(1),
  compliantLimit: z.coerce.number().int().min(0),
  immuneHours: z.coerce.number().int().min(0),
  retentionDays: z.coerce.number().int().min(1),
  maxOutputTokens: z.coerce.number().int().min(1),
  allowPrivateUrl: z.boolean(),
})

type Values = z.infer<typeof schema>

const defaultFormValues: Values = {
  enabled: false,
  baseUrl: '',
  apiKey: '',
  model: '',
  policyText: '',
  timeoutSeconds: 30,
  maxAttempts: 3,
  retryIntervalSeconds: 30,
  workerConcurrency: 2,
  confidenceThreshold: 0.9,
  compliantLimit: 3,
  immuneHours: 5,
  retentionDays: 90,
  maxOutputTokens: 1200,
  allowPrivateUrl: false,
}

export function LLMReviewSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [clearApiKeyOpen, setClearApiKeyOpen] = useState(false)

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: defaultFormValues,
  })

  const { isDirty, isSubmitting } = form.formState
  const baseUrl = form.watch('baseUrl')
  const apiKey = form.watch('apiKey')
  const model = form.watch('model')
  const timeoutSeconds = form.watch('timeoutSeconds')
  const allowPrivateUrl = form.watch('allowPrivateUrl')
  const policyText = form.watch('policyText')
  const candidateReady = baseUrl.trim() !== '' && model.trim() !== ''

  const configQuery = useQuery({
    queryKey: ['llm-review-config'],
    queryFn: getLLMReviewConfig,
    staleTime: 30_000,
  })
  const config = configQuery.data?.data

  useEffect(() => {
    if (!config) return
    form.reset({
      enabled: config.enabled,
      baseUrl: config.base_url,
      apiKey: '',
      model: config.model,
      policyText: config.policy_text,
      timeoutSeconds: config.timeout_seconds,
      maxAttempts: config.max_attempts,
      retryIntervalSeconds: config.retry_interval_seconds,
      workerConcurrency: config.worker_concurrency,
      confidenceThreshold: config.confidence_threshold,
      compliantLimit: config.compliant_limit,
      immuneHours: config.immune_hours,
      retentionDays: config.retention_days,
      maxOutputTokens: config.max_output_tokens,
      allowPrivateUrl: config.allow_private_url,
    })
  }, [config, form])

  const updateConfigMutation = useMutation({
    mutationFn: updateLLMReviewConfig,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to save settings'))
        return
      }
      toast.success(t('Settings saved successfully'))
      queryClient.invalidateQueries({ queryKey: ['llm-review-config'] })
      queryClient.invalidateQueries({
        queryKey: ['llm-review-schema-status'],
      })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to save settings'))
    },
  })

  async function onSubmit(values: Values) {
    const newKey = values.apiKey.trim()
    await updateConfigMutation.mutateAsync({
      enabled: values.enabled,
      base_url: values.baseUrl,
      api_key: newKey || undefined,
      model: values.model,
      policy_text: values.policyText,
      timeout_seconds: values.timeoutSeconds,
      max_attempts: values.maxAttempts,
      retry_interval_seconds: values.retryIntervalSeconds,
      worker_concurrency: values.workerConcurrency,
      confidence_threshold: values.confidenceThreshold,
      compliant_limit: values.compliantLimit,
      immune_hours: values.immuneHours,
      retention_days: values.retentionDays,
      max_output_tokens: values.maxOutputTokens,
      allow_private_url: values.allowPrivateUrl,
    })
  }

  const testConnectionMutation = useMutation({
    mutationFn: () =>
      testLLMReviewConnection({
        base_url: baseUrl,
        api_key: apiKey.trim() || undefined,
        model,
        timeout_seconds: timeoutSeconds,
        allow_private_url: allowPrivateUrl,
      }),
    onSuccess: (data) => {
      if (data.success && data.data) {
        toast.success(
          `${t('Connection successful')} (${data.data.latency_ms}ms, ${data.data.model || '-'})`
        )
        queryClient.invalidateQueries({
          queryKey: ['llm-review-schema-status'],
        })
      } else {
        toast.error(data.message || t('Connection failed'))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Connection failed'))
    },
  })

  const schemaTestMutation = useMutation({
    mutationFn: () =>
      testLLMReviewSchema({
        base_url: baseUrl,
        api_key: apiKey.trim() || undefined,
        model,
        timeout_seconds: timeoutSeconds,
        allow_private_url: allowPrivateUrl,
      }),
    retry: false,
    onSuccess: (data) => {
      // The dashboard API uses success:false with HTTP 200 for capability
      // failures, so refresh cached status even when the probe did not pass.
      queryClient.invalidateQueries({
        queryKey: ['llm-review-schema-status'],
      })
      queryClient.invalidateQueries({ queryKey: ['llm-review-config'] })
      if (data.success) {
        const mode = data.data?.structured_output_mode
        toast.success(
          mode
            ? `${t('Structured output capability test passed')}: ${getReviewOutputModeLabel(mode, t)}`
            : t('Structured output capability test passed')
        )
      } else {
        toast.error(
          data.message || t('Structured output capability test failed')
        )
      }
    },
    onError: (error: Error) => {
      queryClient.invalidateQueries({
        queryKey: ['llm-review-schema-status'],
      })
      queryClient.invalidateQueries({ queryKey: ['llm-review-config'] })
      toast.error(
        error.message || t('Structured output capability test failed')
      )
    },
  })
  const schemaTestInFlightRef = useRef(false)

  async function runSchemaTest() {
    if (schemaTestInFlightRef.current || schemaTestMutation.isPending) return
    schemaTestInFlightRef.current = true
    try {
      await schemaTestMutation.mutateAsync()
    } catch {
      // The mutation's onError handler already shows the user-facing error.
    } finally {
      schemaTestInFlightRef.current = false
    }
  }

  const schemaStatusQuery = useQuery({
    queryKey: ['llm-review-schema-status'],
    queryFn: getLLMReviewSchemaStatus,
    staleTime: 30_000,
  })

  const queueSummaryQuery = useQuery({
    queryKey: ['llm-review-queue-summary'],
    queryFn: getLLMReviewQueueSummary,
    staleTime: 15_000,
  })

  const clearApiKeyMutation = useMutation({
    mutationFn: clearLLMReviewApiKey,
    onSuccess: (data) => {
      if (data.success) {
        toast.success(t('API key cleared'))
        form.setValue('apiKey', '', { shouldDirty: false })
        queryClient.invalidateQueries({ queryKey: ['llm-review-config'] })
        queryClient.invalidateQueries({
          queryKey: ['llm-review-schema-status'],
        })
      } else {
        toast.error(data.message || t('Failed to clear API key'))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to clear API key'))
    },
  })

  const schemaStatus = schemaStatusQuery.data?.data
  const queueSummary = queueSummaryQuery.data?.data
  const maskedKey = config?.api_key || ''

  function renderSchemaStatus() {
    if (schemaStatusQuery.isLoading) {
      return <Skeleton className='h-8 w-56 rounded-md' />
    }
    if (!schemaStatus) {
      return (
        <span className='text-muted-foreground text-xs'>
          {t('No schema test result yet.')}
        </span>
      )
    }
    if (schemaStatus.status === 'passed') {
      const compatibilityMode =
        schemaStatus.structured_output_mode !== 'strict_schema'
      return (
        <div className='flex flex-col gap-1'>
          <div className='flex flex-wrap items-center gap-2'>
            <Badge variant={compatibilityMode ? 'secondary' : 'default'}>
              <ShieldCheck data-icon='inline-start' />
              {compatibilityMode ? t('Compatibility mode') : t('Supported')}
            </Badge>
            <span className='text-muted-foreground text-xs'>
              {t('Output mode')}:{' '}
              {getReviewOutputModeLabel(schemaStatus.structured_output_mode, t)}
            </span>
            <span className='text-muted-foreground text-xs'>
              {t('Tested model')}:{' '}
              {schemaStatus.tested_model ||
                schemaStatus.structured_output_tested_model ||
                '-'}
            </span>
          </div>
          {compatibilityMode && (
            <span className='text-muted-foreground text-xs'>
              {t('Auto-ban is disabled in compatibility mode.')}
            </span>
          )}
          {!schemaStatus.ready && schemaStatus.readiness_reason && (
            <span className='text-destructive text-xs'>
              {schemaStatus.readiness_reason}
            </span>
          )}
        </div>
      )
    }
    if (schemaStatus.status === 'failed') {
      return (
        <div className='flex flex-col gap-1'>
          <Badge variant='destructive'>{t('Capability test failed')}</Badge>
          {schemaStatus.error && (
            <span className='text-muted-foreground text-xs'>
              {schemaStatus.error}
            </span>
          )}
        </div>
      )
    }
    return (
      <div className='flex flex-col gap-1'>
        <Badge variant='secondary'>{t('Untested')}</Badge>
        {schemaStatus.readiness_reason && (
          <span className='text-muted-foreground text-xs'>
            {schemaStatus.readiness_reason}
          </span>
        )}
      </div>
    )
  }

  return (
    <SettingsSection title={t('LLM Compliance Review')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={isSubmitting || updateConfigMutation.isPending}
            isSaveDisabled={!isDirty}
          />

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormLabel>{t('Enable compliance review')}</FormLabel>
              <FormDescription>
                {t(
                  'Disabled by default. Requires a configured reviewer endpoint, policy text, and a passing structured-output capability test before it can be enabled.'
                )}
              </FormDescription>
            </SettingsSwitchContent>
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={(checked) =>
                      form.setValue('enabled', checked, { shouldDirty: true })
                    }
                  />
                </FormControl>
              )}
            />
          </SettingsSwitchItem>

          <SettingsControlGroup title={t('Reviewer Endpoint')}>
            <FormField
              control={form.control}
              name='baseUrl'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Base URL')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder='https://reviewer.example.com'
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='model'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Reviewer Model')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='gpt-4o-mini' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='apiKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('API Key')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type='password'
                      autoComplete='off'
                      placeholder={
                        maskedKey || t('Leave empty to keep the stored key')
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Encrypted at rest; only a tail-derived mask is ever returned.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            {maskedKey && (
              <div className='flex items-center gap-2'>
                <Badge variant='outline'>{maskedKey}</Badge>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={() => setClearApiKeyOpen(true)}
                >
                  <Trash2 data-icon='inline-start' />
                  {t('Clear Key')}
                </Button>
              </div>
            )}
            <SettingsSwitchItem>
              <SettingsSwitchContent>
                <FormLabel>{t('Allow private addresses')}</FormLabel>
                <FormDescription>
                  {t(
                    'Advanced: permits private addresses for the reviewer endpoint. Cloud metadata addresses stay blocked.'
                  )}
                </FormDescription>
              </SettingsSwitchContent>
              <FormField
                control={form.control}
                name='allowPrivateUrl'
                render={({ field }) => (
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={(checked) =>
                        form.setValue('allowPrivateUrl', checked, {
                          shouldDirty: true,
                        })
                      }
                    />
                  </FormControl>
                )}
              />
            </SettingsSwitchItem>
          </SettingsControlGroup>

          <SettingsControlGroup title={t('Compliance Policy')}>
            <FormField
              control={form.control}
              name='policyText'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Policy Text')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      rows={6}
                      placeholder={t(
                        'The exact terms the reviewer is allowed to enforce. Empty policy forces uncertain verdicts.'
                      )}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Submitted with every review payload; HTML/Markdown wrappers are stripped and length is capped.'
                    )}
                  </FormDescription>
                  {!policyText.trim() && config && (
                    <div className='text-destructive flex items-center gap-1 text-xs'>
                      <AlertTriangle aria-hidden='true' />
                      {t(
                        'Policy text is required before compliance review can run.'
                      )}
                    </div>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsControlGroup>

          <SettingsControlGroup title={t('Verification')}>
            <div className='flex flex-wrap items-center gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={
                  !candidateReady ||
                  testConnectionMutation.isPending ||
                  schemaTestMutation.isPending
                }
                onClick={() => testConnectionMutation.mutate()}
              >
                {testConnectionMutation.isPending ? (
                  <Loader2 className='animate-spin' data-icon='inline-start' />
                ) : (
                  <PlugZap data-icon='inline-start' />
                )}
                {t('Test Connection')}
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={
                  !candidateReady ||
                  testConnectionMutation.isPending ||
                  schemaTestMutation.isPending
                }
                onClick={() => void runSchemaTest()}
              >
                {schemaTestMutation.isPending ? (
                  <Loader2 className='animate-spin' data-icon='inline-start' />
                ) : (
                  <ListChecks data-icon='inline-start' />
                )}
                {t('Test Structured Output')}
              </Button>
            </div>
            {renderSchemaStatus()}
          </SettingsControlGroup>

          <SettingsControlGroup title={t('Worker')}>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
              <FormField
                control={form.control}
                name='timeoutSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Timeout (seconds)')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' min={1} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='maxAttempts'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Max Attempts')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' min={1} max={10} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='retryIntervalSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Retry Interval (seconds)')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' min={1} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='workerConcurrency'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Worker Concurrency')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' min={1} max={20} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='maxOutputTokens'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Max Output Tokens')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' min={1} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </SettingsControlGroup>

          <SettingsControlGroup title={t('Automatic Disable')}>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
              <FormField
                control={form.control}
                name='confidenceThreshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Confidence Threshold')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        step={0.01}
                        min={0}
                        max={1}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='compliantLimit'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Compliant Limit')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' min={0} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='immuneHours'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Immune Hours')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' min={0} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='retentionDays'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Retention Days')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' min={1} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormDescription>
              {t(
                'Violations above the confidence threshold in auto-ban categories permanently disable the account. Records tied to permanent disables are never deleted.'
              )}
            </FormDescription>
          </SettingsControlGroup>

          <Separator />

          <SettingsControlGroup title={t('Queue Overview')}>
            <div className='flex flex-wrap items-center gap-2 text-xs'>
              <Badge variant='outline'>
                {t('Pending')}: {queueSummary?.pending ?? '-'}
              </Badge>
              <Badge variant='outline'>
                {t('Reviewing')}: {queueSummary?.reviewing ?? '-'}
              </Badge>
              <span className='text-muted-foreground'>
                {t('Oldest Wait')}:{' '}
                {formatWaitingSeconds(queueSummary?.oldest_waiting_seconds)}
              </span>
              <span className='text-muted-foreground'>
                {t('Recent Failure Rate')}:{' '}
                {formatFailureRate(queueSummary?.recent_failure_rate)}
              </span>
            </div>
          </SettingsControlGroup>
        </SettingsForm>
      </Form>

      <ConfirmDialog
        open={clearApiKeyOpen}
        onOpenChange={setClearApiKeyOpen}
        title={t('Clear API Key')}
        desc={t(
          'This clears the stored reviewer API key and resets the structured-output capability state. The review service cannot run until a capability test passes again.'
        )}
        confirmText={t('Clear')}
        handleConfirm={() => clearApiKeyMutation.mutate()}
        isLoading={clearApiKeyMutation.isPending}
        destructive
      />
    </SettingsSection>
  )
}
