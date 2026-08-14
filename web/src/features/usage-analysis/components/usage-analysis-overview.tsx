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
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  Coins,
  Database,
  Sparkles,
  Zap,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatPercent, formatQuota, formatTokens } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { UsageAnalysisMetrics } from '../api'

type UsageAnalysisOverviewProps = {
  summary?: UsageAnalysisMetrics
  selectedUserName: string
  selectedTokenName: string
  isLoading: boolean
}

type MetricKind = 'integer' | 'percent' | 'quota' | 'tokens'

type OverviewMetric = {
  key: string
  label: string
  value: number
  icon: typeof Activity
  kind: MetricKind
  iconClassName: string
  tileClassName: string
}

function formatInteger(value: number): string {
  return Math.round(value).toLocaleString()
}

function formatMetricValue(value: number, kind: MetricKind): string {
  if (kind === 'percent') return formatPercent(value)
  if (kind === 'quota') return formatQuota(value)
  if (kind === 'tokens') return formatTokens(value)
  return formatInteger(value)
}

function metricDetail(value: number, kind: MetricKind): string | null {
  if (kind === 'tokens') return formatInteger(value)
  return null
}

export function UsageAnalysisOverview(props: UsageAnalysisOverviewProps) {
  const { t } = useTranslation()
  const summary = props.summary
  const totalTokens = summary?.total_tokens ?? 0
  const totalRequests = summary?.request_count ?? 0
  const quota = summary?.quota ?? 0
  const averageTokens = totalRequests > 0 ? totalTokens / totalRequests : 0

  const metrics: OverviewMetric[] = [
    {
      key: 'input',
      label: t('Input Tokens'),
      value: summary?.prompt_tokens ?? 0,
      icon: ArrowDownToLine,
      kind: 'tokens',
      iconClassName: 'text-blue-500',
      tileClassName: 'border-blue-500/20 bg-blue-500/5',
    },
    {
      key: 'output',
      label: t('Output Tokens'),
      value: summary?.completion_tokens ?? 0,
      icon: ArrowUpFromLine,
      kind: 'tokens',
      iconClassName: 'text-emerald-500',
      tileClassName: 'border-emerald-500/20 bg-emerald-500/5',
    },
    {
      key: 'cache-read',
      label: t('Cache Read Tokens'),
      value: summary?.cache_read_tokens ?? 0,
      icon: Zap,
      kind: 'tokens',
      iconClassName: 'text-amber-500',
      tileClassName: 'border-amber-500/20 bg-amber-500/5',
    },
    {
      key: 'cache-write',
      label: t('Cache Write Tokens'),
      value: summary?.cache_write_tokens ?? 0,
      icon: Zap,
      kind: 'tokens',
      iconClassName: 'text-pink-500',
      tileClassName: 'border-pink-500/20 bg-pink-500/5',
    },
    {
      key: 'cache-rate',
      label: t('Cache Rate'),
      value: summary?.cache_rate ?? 0,
      icon: Zap,
      kind: 'percent',
      iconClassName: 'text-cyan-500',
      tileClassName: 'border-cyan-500/20 bg-cyan-500/5',
    },
    {
      key: 'average',
      label: t('Average Tokens per Request'),
      value: averageTokens,
      icon: Sparkles,
      kind: 'tokens',
      iconClassName: 'text-violet-500',
      tileClassName: 'border-violet-500/20 bg-violet-500/5',
    },
    {
      key: 'quota',
      label: t('Consumed Quota'),
      value: quota,
      icon: Coins,
      kind: 'quota',
      iconClassName: 'text-orange-500',
      tileClassName: 'border-orange-500/20 bg-orange-500/5',
    },
    {
      key: 'legacy',
      label: t('Legacy Usage Rows'),
      value: summary?.legacy_request_count ?? 0,
      icon: Database,
      kind: 'integer',
      iconClassName: 'text-muted-foreground',
      tileClassName: 'bg-muted/30',
    },
  ]

  return (
    <Card className='overflow-hidden'>
      <CardContent className='p-5 sm:p-6'>
        <div className='flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between'>
          <div className='flex min-w-0 items-center gap-3'>
            <div className='bg-primary/10 text-primary flex size-11 shrink-0 items-center justify-center rounded-xl'>
              <Zap className='size-6' aria-hidden='true' />
            </div>
            <div className='min-w-0'>
              <div className='text-muted-foreground text-xs font-medium'>
                {t('Actual Consumed Tokens')}
              </div>
              {props.isLoading ? (
                <Skeleton className='mt-2 h-9 w-56' />
              ) : (
                <div className='mt-1 flex flex-wrap items-baseline gap-2'>
                  <span className='text-3xl font-bold tracking-tight tabular-nums sm:text-4xl'>
                    {formatInteger(totalTokens)}
                  </span>
                  <span className='text-muted-foreground text-sm'>
                    ≈ {formatTokens(totalTokens)}
                  </span>
                </div>
              )}
              <div className='text-muted-foreground mt-1 truncate text-xs'>
                {props.selectedUserName} · {props.selectedTokenName}
              </div>
            </div>
          </div>

          <div className='grid grid-cols-2 divide-x rounded-xl border px-4 py-3 shadow-sm'>
            <div className='pr-5'>
              <div className='text-muted-foreground text-xs'>
                {t('Total Requests')}
              </div>
              <div className='mt-1 flex items-center gap-1.5 font-semibold tabular-nums'>
                <Activity className='text-primary size-4' aria-hidden='true' />
                {props.isLoading ? (
                  <Skeleton className='h-5 w-14' />
                ) : (
                  formatInteger(totalRequests)
                )}
              </div>
            </div>
            <div className='pl-5'>
              <div className='text-muted-foreground text-xs'>
                {t('Total Cost')}
              </div>
              <div className='mt-1 font-semibold text-emerald-500 tabular-nums'>
                {props.isLoading ? (
                  <Skeleton className='h-5 w-20' />
                ) : (
                  formatQuota(quota)
                )}
              </div>
            </div>
          </div>
        </div>

        <div className='mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
          {metrics.map((metric) => {
            const detail = metricDetail(metric.value, metric.kind)
            return (
              <div
                key={metric.key}
                aria-label={metric.label}
                className={cn(
                  'rounded-xl border p-3 shadow-sm',
                  metric.tileClassName
                )}
              >
                <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                  <metric.icon
                    className={cn('size-4', metric.iconClassName)}
                    aria-hidden='true'
                  />
                  {metric.label}
                </div>
                {props.isLoading ? (
                  <Skeleton className='mt-2 h-7 w-24' />
                ) : (
                  <>
                    <div className='mt-2 text-lg font-semibold tabular-nums'>
                      {formatMetricValue(metric.value, metric.kind)}
                    </div>
                    {detail && (
                      <div className='text-muted-foreground mt-0.5 text-xs tabular-nums'>
                        {detail}
                      </div>
                    )}
                  </>
                )}
              </div>
            )
          })}
        </div>

        {!props.isLoading && (summary?.legacy_request_count ?? 0) > 0 && (
          <Alert className='mt-4'>
            <Database aria-hidden='true' />
            <AlertDescription>
              {t('Legacy Usage Rows')}:{' '}
              {formatInteger(summary?.legacy_request_count ?? 0)} ·{' '}
              {t('Legacy rows are excluded from cache metrics.')}
            </AlertDescription>
          </Alert>
        )}
      </CardContent>
    </Card>
  )
}
