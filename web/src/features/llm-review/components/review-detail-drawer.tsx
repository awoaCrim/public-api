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
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Loader2, RefreshCw, UserCog } from 'lucide-react'
import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { getLLMReviewTaskDetail, retryLLMReviewTask } from '../api'
import {
  formatReviewTime,
  getReviewCategoryLabel,
  getReviewStatusLabel,
  getReviewTriggerLabel,
} from '../lib/format'
import type { ReviewTask } from '../types'

function DetailRow(props: {
  label: ReactNode
  value: ReactNode
  mono?: boolean
}) {
  return (
    <div className='grid min-w-0 grid-cols-[6.5rem_minmax(0,1fr)] gap-2 text-sm sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-3'>
      <span className='text-muted-foreground min-w-0 text-xs'>
        {props.label}
      </span>
      <span
        className={cn(
          'max-w-full min-w-0 text-xs break-all sm:wrap-break-word',
          props.mono && 'font-mono'
        )}
      >
        {props.value}
      </span>
    </div>
  )
}

function DetailSection(props: {
  label: string
  danger?: boolean
  children: ReactNode
}) {
  return (
    <div className='min-w-0 space-y-1.5'>
      <span
        className={cn(
          'block text-xs font-semibold',
          props.danger && 'text-red-500'
        )}
      >
        {props.label}
      </span>
      <div
        className={cn(
          'min-w-0 space-y-1 overflow-hidden rounded-md border p-2.5',
          props.danger
            ? 'border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/20'
            : 'bg-muted/30'
        )}
      >
        {props.children}
      </div>
    </div>
  )
}

function formatNumber(value: number | undefined | null): string {
  if (value == null) return '-'
  return value.toLocaleString()
}

interface ReviewDetailDrawerProps {
  task: ReviewTask | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onRetried?: () => void
}

export function ReviewDetailDrawer(props: ReviewDetailDrawerProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['llm-review-task-detail', props.task?.id],
    queryFn: () => {
      if (!props.task) return Promise.reject(new Error('No task selected'))
      return getLLMReviewTaskDetail(props.task.id)
    },
    enabled: props.open && !!props.task,
    staleTime: 0,
  })
  const detail = data?.success ? data.data : undefined

  const channelDisplay = useMemo(() => {
    if (!detail) return '-'
    if (detail.channel_name) {
      return `${detail.channel_name} (ID: ${detail.channel_id})`
    }
    if (detail.channel_assignment === 'unassigned_preflight') {
      return t('Unassigned (RPM preflight block)')
    }
    return t('Unassigned')
  }, [detail, t])

  const canRetry = useMemo(() => {
    if (!detail) return false
    return detail.status === 'failed' || detail.status === 'uncertain'
  }, [detail])

  const evidenceItems = useMemo(() => {
    const occurrences = new Map<string, number>()
    return (detail?.evidence ?? []).map((value) => {
      const occurrence = (occurrences.get(value) ?? 0) + 1
      occurrences.set(value, occurrence)
      return { key: `${value}:${occurrence}`, value }
    })
  }, [detail?.evidence])

  const handleRetry = async () => {
    if (!detail) return
    try {
      const res = await retryLLMReviewTask(detail.id)
      if (res.success) {
        toast.success(t('Review task resubmitted'))
        props.onRetried?.()
      } else {
        toast.error(res.message || t('Failed to resubmit review'))
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to resubmit review')
      )
    }
  }

  const handleUserJump = () => {
    const username = detail?.username ?? props.task?.username
    void navigate({
      to: '/users',
      search: { filter: username ? username : undefined },
    })
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent side='right' className='w-full sm:max-w-xl'>
        <SheetHeader>
          <SheetTitle className='flex items-center gap-2 pr-6'>
            {t('Review Details')}
            {detail && (
              <Badge variant='secondary'>
                {getReviewStatusLabel(detail.status, t)}
              </Badge>
            )}
          </SheetTitle>
        </SheetHeader>

        {isLoading || !detail ? (
          <div className='flex flex-1 flex-col gap-3 p-4'>
            <Skeleton className='h-4 w-2/3 rounded-md' />
            <Skeleton className='h-4 w-1/2 rounded-md' />
            <Skeleton className='h-24 w-full rounded-md' />
            <Skeleton className='h-24 w-full rounded-md' />
          </div>
        ) : (
          <ScrollArea className='min-h-0 flex-1'>
            <div className='space-y-3 px-4 pb-6'>
              <div className='min-w-0 space-y-1'>
                <DetailRow
                  label={t('Review No')}
                  value={detail.review_no || `#${detail.id}`}
                  mono
                />
                <DetailRow
                  label={t('User')}
                  value={`${detail.display_name || detail.username || '-'} (ID: ${detail.user_id})`}
                />
                <DetailRow
                  label={t('Status')}
                  value={getReviewStatusLabel(detail.status, t)}
                />
                <DetailRow
                  label={t('Trigger Type')}
                  value={getReviewTriggerLabel(detail.trigger_type, t)}
                />
                <DetailRow
                  label={t('Trigger Stage')}
                  value={
                    detail.trigger_stage === 'preflight'
                      ? t('Preflight')
                      : t('Postflight')
                  }
                />
                <DetailRow
                  label={t('Model')}
                  value={detail.model_name || '-'}
                />
                <DetailRow label={t('Channel')} value={channelDisplay} />
                {detail.channel_assignment === 'unassigned_preflight' &&
                  detail.recent_channel_name && (
                    <DetailRow
                      label={t('Recently used channel')}
                      value={`${detail.recent_channel_name} (ID: ${detail.recent_channel_id}, ${formatReviewTime(detail.recent_channel_at)})`}
                    />
                  )}
                <DetailRow
                  label={t('API Endpoint')}
                  value={detail.api_endpoint || '-'}
                  mono
                />
                <DetailRow label='IP' value={detail.ip_masked || '-'} mono />
              </div>

              <Separator />

              <div className='min-w-0 space-y-1'>
                <DetailRow
                  label={t('Current / Limit')}
                  value={`${formatNumber(detail.current_value)} / ${formatNumber(detail.limit_value)}`}
                  mono
                />
                <DetailRow
                  label={t('Estimated Input')}
                  value={formatNumber(detail.estimated_input_tokens)}
                  mono
                />
                <DetailRow
                  label={t('Actual Input')}
                  value={formatNumber(detail.actual_input_tokens)}
                  mono
                />
                <DetailRow
                  label={t('Actual Output')}
                  value={formatNumber(detail.actual_output_tokens)}
                  mono
                />
                <DetailRow
                  label={t('Merged Events')}
                  value={formatNumber(detail.merged_event_count)}
                />
                <DetailRow
                  label={t('Created At')}
                  value={formatReviewTime(detail.created_at)}
                />
                <DetailRow
                  label={t('Finished At')}
                  value={formatReviewTime(detail.finished_at)}
                />
                <DetailRow
                  label={t('Next Retry At')}
                  value={formatReviewTime(detail.next_retry_at)}
                />
              </div>

              {detail.verdict && (
                <>
                  <Separator />
                  <DetailSection label={t('LLM Verdict')}>
                    <DetailRow
                      label={t('Verdict')}
                      value={detail.verdict}
                      mono
                    />
                    <DetailRow
                      label={t('Category')}
                      value={getReviewCategoryLabel(detail.category, t)}
                    />
                    <DetailRow
                      label={t('Confidence')}
                      value={
                        detail.confidence == null
                          ? '-'
                          : `${(detail.confidence * 100).toFixed(0)}%`
                      }
                    />
                    <DetailRow
                      label={t('Reason')}
                      value={detail.short_reason || '-'}
                    />
                    {detail.human_override && (
                      <DetailRow
                        label={t('Human Override')}
                        value={detail.human_override}
                      />
                    )}
                  </DetailSection>
                </>
              )}

              {evidenceItems.length > 0 && (
                <DetailSection label={t('Evidence')}>
                  {evidenceItems.map((item) => (
                    <div key={item.key} className='text-xs'>
                      {item.value}
                    </div>
                  ))}
                </DetailSection>
              )}

              {detail.request_summary && (
                <DetailSection label={t('Request Summary')}>
                  <pre className='max-h-48 overflow-auto text-xs break-all whitespace-pre-wrap'>
                    {detail.request_summary}
                  </pre>
                </DetailSection>
              )}

              {detail.review_payload && (
                <DetailSection label={t('Review Payload')}>
                  <pre className='max-h-48 overflow-auto text-xs break-all whitespace-pre-wrap'>
                    {detail.review_payload}
                  </pre>
                </DetailSection>
              )}

              {detail.schema_error && (
                <DetailSection label={t('Schema Error')} danger>
                  <span className='text-xs'>{detail.schema_error}</span>
                </DetailSection>
              )}

              {detail.banned && (
                <DetailSection label={t('Account Permanently Disabled')} danger>
                  <span className='text-xs'>{t('Yes')}</span>
                </DetailSection>
              )}
              {detail.ban_error && (
                <DetailSection label={t('Ban Error')} danger>
                  <span className='text-xs'>{detail.ban_error}</span>
                </DetailSection>
              )}

              {detail.attempts && detail.attempts.length > 0 && (
                <DetailSection label={t('Attempts')}>
                  {detail.attempts.map((attempt) => (
                    <div
                      key={attempt.id}
                      className='text-muted-foreground text-xs'
                    >
                      #{attempt.attempt_no}{' '}
                      {formatReviewTime(attempt.requested_at)} ·{' '}
                      {attempt.duration_ms}ms · HTTP {attempt.http_status}
                      {attempt.error ? ` · ${attempt.error}` : ''}
                    </div>
                  ))}
                </DetailSection>
              )}

              <Separator />

              <div className='flex flex-wrap items-center gap-2'>
                <Button variant='outline' size='sm' onClick={handleUserJump}>
                  <UserCog />
                  {t('View User')}
                </Button>
                {canRetry && (
                  <Button
                    variant='secondary'
                    size='sm'
                    disabled={isFetching}
                    onClick={handleRetry}
                  >
                    {isFetching ? (
                      <Loader2 className='animate-spin' />
                    ) : (
                      <RefreshCw />
                    )}
                    {t('Resubmit Review')}
                  </Button>
                )}
              </div>
            </div>
          </ScrollArea>
        )}
      </SheetContent>
    </Sheet>
  )
}
