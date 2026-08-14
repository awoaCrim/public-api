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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import {
  Activity,
  AlertCircle,
  ArrowDownToLine,
  ArrowUpFromLine,
  BarChart3,
  Database,
  RefreshCw,
  Zap,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  XAxis,
  YAxis,
} from 'recharts'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import dayjs from '@/lib/dayjs'
import { formatPercent, formatQuota } from '@/lib/format'

import {
  getUsageAnalysis,
  getUsageAnalysisOptions,
  type UsageAnalysisParams,
} from './api'
import {
  buildUsageAnalysisTrendData,
  getTodayUsageAnalysisRange,
  resetTokenSelectionForUser,
  type UsageAnalysisSelection,
} from './lib/usage-analysis'

const ALL = 'all'
const DEFAULT_PAGE_SIZE = 20

type UsageAnalysisFilters = UsageAnalysisSelection

function getDefaultFilters(): UsageAnalysisFilters {
  return {
    range: getTodayUsageAnalysisRange(),
    userId: ALL,
    tokenId: ALL,
    modelName: ALL,
    channelId: ALL,
  }
}

function formatInteger(value: number): string {
  return Math.round(value).toLocaleString()
}

export function UsageAnalysis() {
  const { t } = useTranslation()
  const [filters, setFilters] = useState(getDefaultFilters)
  const [appliedFilters, setAppliedFilters] = useState(getDefaultFilters)
  const [page, setPage] = useState(1)

  const optionsQuery = useQuery({
    queryKey: ['usage-analysis', 'options'],
    queryFn: getUsageAnalysisOptions,
    staleTime: 5 * 60 * 1000,
  })
  const options = optionsQuery.data?.data

  const queryParams = useMemo<UsageAnalysisParams>(
    () => ({
      start_timestamp: Math.floor(appliedFilters.range.start.getTime() / 1000),
      end_timestamp: Math.floor(appliedFilters.range.end.getTime() / 1000),
      page,
      page_size: DEFAULT_PAGE_SIZE,
      user_id:
        appliedFilters.userId === ALL
          ? undefined
          : Number(appliedFilters.userId),
      token_id:
        appliedFilters.tokenId === ALL
          ? undefined
          : Number(appliedFilters.tokenId),
      model_name:
        appliedFilters.modelName === ALL ? undefined : appliedFilters.modelName,
      channel_id:
        appliedFilters.channelId === ALL
          ? undefined
          : Number(appliedFilters.channelId),
    }),
    [appliedFilters, page]
  )

  const analysisQuery = useQuery({
    queryKey: ['usage-analysis', queryParams],
    queryFn: () => getUsageAnalysis(queryParams),
    placeholderData: keepPreviousData,
  })
  const analysis = analysisQuery.data?.data
  const summary = analysis?.summary
  const totalPages = Math.max(
    1,
    Math.ceil((analysis?.total ?? 0) / DEFAULT_PAGE_SIZE)
  )
  const visibleTokens = useMemo(() => {
    const tokens = options?.tokens ?? []
    if (filters.userId === ALL) return tokens
    return tokens.filter((token) => token.user_id === Number(filters.userId))
  }, [filters.userId, options?.tokens])

  const trendData = useMemo(
    () =>
      buildUsageAnalysisTrendData(
        analysis?.trend ?? [],
        appliedFilters.range.start,
        appliedFilters.range.end,
        analysis?.bucket_seconds ?? 3600
      ).map((point) => ({
        ...point,
        label: dayjs.unix(point.timestamp).format('MM/DD HH:mm'),
      })),
    [analysis?.bucket_seconds, analysis?.trend, appliedFilters.range]
  )

  const chartConfig = {
    totalTokens: { label: t('Total Tokens'), color: '#8b5cf6' },
    promptTokens: { label: t('Input Tokens'), color: '#3b82f6' },
    completionTokens: { label: t('Output Tokens'), color: '#10b981' },
    cacheReadTokens: { label: t('Cache Read Tokens'), color: '#f59e0b' },
    cacheWriteTokens: { label: t('Cache Write Tokens'), color: '#ec4899' },
  } satisfies ChartConfig

  const applyFilters = () => {
    if (filters.range.end < filters.range.start) {
      toast.error(t('End date must be after start date.'))
      return
    }
    const filtersUnchanged = filters === appliedFilters && page === 1
    setPage(1)
    setAppliedFilters(filters)
    if (filtersUnchanged) void analysisQuery.refetch()
  }
  const selectUser = (value: string | null) => {
    if (value === null) return
    setFilters((current) => resetTokenSelectionForUser(current, value, ALL))
  }
  const isLoading = analysisQuery.isLoading || optionsQuery.isLoading
  const isFetching = analysisQuery.isFetching || optionsQuery.isFetching
  let errorMessage: string | undefined
  if (analysisQuery.data && !analysisQuery.data.success) {
    errorMessage = analysisQuery.data.message
  } else if (optionsQuery.data && !optionsQuery.data.success) {
    errorMessage = optionsQuery.data.message
  } else if (analysisQuery.error instanceof Error) {
    errorMessage = analysisQuery.error.message
  } else if (optionsQuery.error instanceof Error) {
    errorMessage = optionsQuery.error.message
  }

  const metrics = [
    {
      key: 'requests',
      label: t('Requests'),
      value: summary?.request_count ?? 0,
      icon: Activity,
    },
    {
      key: 'input',
      label: t('Input Tokens'),
      value: summary?.prompt_tokens ?? 0,
      icon: ArrowDownToLine,
    },
    {
      key: 'output',
      label: t('Output Tokens'),
      value: summary?.completion_tokens ?? 0,
      icon: ArrowUpFromLine,
    },
    {
      key: 'cache-read',
      label: t('Cache Read Tokens'),
      value: summary?.cache_read_tokens ?? 0,
      icon: Zap,
    },
    {
      key: 'cache-write',
      label: t('Cache Write Tokens'),
      value: summary?.cache_write_tokens ?? 0,
      icon: Zap,
    },
    {
      key: 'legacy',
      label: t('Legacy Usage Rows'),
      value: summary?.legacy_request_count ?? 0,
      icon: Database,
    },
  ]

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Usage Analysis')}</span>
          <Badge variant='outline' className='shrink-0'>
            {t('Root Only')}
          </Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <div className='flex flex-wrap items-center gap-2'>
            <Select value={filters.userId} onValueChange={selectUser}>
              <SelectTrigger className='min-w-32'>
                <SelectValue placeholder={t('All Users')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL}>{t('All Users')}</SelectItem>
                  {(options?.users ?? []).map((user) => (
                    <SelectItem key={user.id} value={String(user.id)}>
                      {user.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>

            <Select
              value={filters.tokenId}
              onValueChange={(value) =>
                value !== null &&
                setFilters((current) => ({ ...current, tokenId: value }))
              }
            >
              <SelectTrigger className='min-w-32'>
                <SelectValue placeholder={t('All API Keys')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL}>{t('All API Keys')}</SelectItem>
                  {visibleTokens.map((token) => (
                    <SelectItem key={token.id} value={String(token.id)}>
                      {token.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>

            <Select
              value={filters.modelName}
              onValueChange={(value) =>
                value !== null &&
                setFilters((current) => ({ ...current, modelName: value }))
              }
            >
              <SelectTrigger className='min-w-36'>
                <SelectValue placeholder={t('All Models')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL}>{t('All Models')}</SelectItem>
                  {(options?.models ?? []).map((modelName) => (
                    <SelectItem key={modelName} value={modelName}>
                      {modelName}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>

            <Select
              value={filters.channelId}
              onValueChange={(value) =>
                value !== null &&
                setFilters((current) => ({ ...current, channelId: value }))
              }
            >
              <SelectTrigger className='min-w-36'>
                <SelectValue placeholder={t('All Channels')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL}>{t('All Channels')}</SelectItem>
                  {(options?.channels ?? []).map((channel) => (
                    <SelectItem key={channel.id} value={String(channel.id)}>
                      {channel.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>

            <CompactDateTimeRangePicker
              start={filters.range.start}
              end={filters.range.end}
              className='w-auto max-w-64'
              onChange={(range) => {
                const { start, end } = range
                if (!start || !end) return
                setFilters((current) => ({
                  ...current,
                  range: { start, end },
                }))
              }}
            />
            <Button
              onClick={applyFilters}
              disabled={isFetching}
              aria-label={t('Refresh usage analysis')}
            >
              <RefreshCw
                className={isFetching ? 'animate-spin' : ''}
                aria-hidden='true'
              />
              {t('Refresh')}
            </Button>
          </div>

          {errorMessage && (
            <div
              className='text-destructive flex items-center gap-2 text-sm'
              role='alert'
            >
              <AlertCircle className='size-4' aria-hidden='true' />
              {errorMessage}
            </div>
          )}

          <Card>
            <CardContent className='p-5 sm:p-6'>
              {isLoading ? (
                <Skeleton className='h-24 w-full' />
              ) : (
                <>
                  <div className='flex flex-wrap items-end justify-between gap-4'>
                    <div>
                      <div className='text-muted-foreground text-sm'>
                        {t('Total Tokens')}
                      </div>
                      <div className='text-4xl font-bold tabular-nums'>
                        {formatInteger(summary?.total_tokens ?? 0)}
                      </div>
                    </div>
                    <div className='text-right'>
                      <div className='text-muted-foreground text-sm'>
                        {t('Total Cost')}
                      </div>
                      <div className='text-xl font-semibold text-emerald-500'>
                        {formatQuota(summary?.quota ?? 0)}
                      </div>
                    </div>
                  </div>
                  <div className='mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-6'>
                    {metrics.map((metric) => (
                      <div key={metric.key} className='rounded-xl border p-3'>
                        <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                          <metric.icon className='size-4' aria-hidden='true' />
                          {metric.label}
                        </div>
                        <div className='mt-2 text-lg font-semibold tabular-nums'>
                          {formatInteger(metric.value)}
                        </div>
                      </div>
                    ))}
                  </div>
                  <div className='text-muted-foreground mt-3 text-xs'>
                    {t('Cache Rate')}: {formatPercent(summary?.cache_rate ?? 0)}{' '}
                    · {t('Legacy rows are excluded from cache metrics.')}
                  </div>
                </>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2 text-base'>
                <BarChart3 className='size-4' aria-hidden='true' />
                {t('Usage Trend')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <Skeleton className='h-[340px] w-full' />
              ) : (
                <ChartContainer
                  config={chartConfig}
                  className='aspect-auto h-[340px] w-full'
                  initialDimension={{ width: 900, height: 340 }}
                >
                  <ComposedChart data={trendData}>
                    <CartesianGrid vertical={false} strokeDasharray='3 3' />
                    <XAxis dataKey='label' tickLine={false} axisLine={false} />
                    <YAxis tickLine={false} axisLine={false} width={72} />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <ChartLegend content={<ChartLegendContent />} />
                    <Area
                      type='monotone'
                      dataKey='totalTokens'
                      stroke='var(--color-totalTokens)'
                      fill='var(--color-totalTokens)'
                      fillOpacity={0.12}
                      dot={false}
                    />
                    <Line
                      type='monotone'
                      dataKey='promptTokens'
                      stroke='var(--color-promptTokens)'
                      dot={false}
                    />
                    <Line
                      type='monotone'
                      dataKey='completionTokens'
                      stroke='var(--color-completionTokens)'
                      dot={false}
                    />
                    <Line
                      type='monotone'
                      dataKey='cacheReadTokens'
                      stroke='var(--color-cacheReadTokens)'
                      dot={false}
                    />
                    <Line
                      type='monotone'
                      dataKey='cacheWriteTokens'
                      stroke='var(--color-cacheWriteTokens)'
                      dot={false}
                    />
                  </ComposedChart>
                </ChartContainer>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className='text-base'>
                {t('Usage Breakdown')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='overflow-x-auto rounded-lg border'>
                <table className='w-full min-w-[1100px] text-sm'>
                  <thead>
                    <tr className='border-b text-left'>
                      <th scope='col' className='px-3 py-2'>
                        {t('User')}
                      </th>
                      <th scope='col' className='px-3 py-2'>
                        {t('API Key')}
                      </th>
                      <th scope='col' className='px-3 py-2'>
                        {t('Model')}
                      </th>
                      <th scope='col' className='px-3 py-2'>
                        {t('Channel')}
                      </th>
                      <th scope='col' className='px-3 py-2 text-right'>
                        {t('Requests')}
                      </th>
                      <th scope='col' className='px-3 py-2 text-right'>
                        {t('Input Tokens')}
                      </th>
                      <th scope='col' className='px-3 py-2 text-right'>
                        {t('Output Tokens')}
                      </th>
                      <th scope='col' className='px-3 py-2 text-right'>
                        {t('Cache Read Tokens')}
                      </th>
                      <th scope='col' className='px-3 py-2 text-right'>
                        {t('Cache Write Tokens')}
                      </th>
                      <th scope='col' className='px-3 py-2 text-right'>
                        {t('Cache Rate')}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {(analysis?.rows ?? []).map((row) => (
                      <tr
                        key={`${row.user_id}-${row.token_id}-${row.model_name}-${row.channel_id}`}
                        className='border-b last:border-0'
                      >
                        <td className='px-3 py-2'>
                          {row.username || `#${row.user_id}`}
                        </td>
                        <td className='px-3 py-2'>
                          {row.token_name || `#${row.token_id}`}
                        </td>
                        <td className='px-3 py-2'>{row.model_name || '-'}</td>
                        <td className='px-3 py-2'>
                          {row.channel_name || `#${row.channel_id}`}
                        </td>
                        <td className='px-3 py-2 text-right'>
                          {formatInteger(row.request_count)}
                        </td>
                        <td className='px-3 py-2 text-right'>
                          {formatInteger(row.prompt_tokens)}
                        </td>
                        <td className='px-3 py-2 text-right'>
                          {formatInteger(row.completion_tokens)}
                        </td>
                        <td className='px-3 py-2 text-right'>
                          {formatInteger(row.cache_read_tokens)}
                        </td>
                        <td className='px-3 py-2 text-right'>
                          {formatInteger(row.cache_write_tokens)}
                        </td>
                        <td className='px-3 py-2 text-right'>
                          {formatPercent(row.cache_rate)}
                        </td>
                      </tr>
                    ))}
                    {!isLoading && (analysis?.rows.length ?? 0) === 0 && (
                      <tr>
                        <td
                          colSpan={10}
                          className='text-muted-foreground h-24 text-center'
                        >
                          {t('No usage data found.')}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
              <div className='mt-3 flex items-center justify-between gap-3'>
                <span className='text-muted-foreground text-sm'>
                  {t('Page {{current}} of {{total}}', {
                    current: page,
                    total: totalPages,
                  })}
                </span>
                <div className='flex gap-2'>
                  <Button
                    variant='outline'
                    disabled={page <= 1 || isFetching}
                    onClick={() =>
                      setPage((current) => Math.max(1, current - 1))
                    }
                  >
                    {t('Previous')}
                  </Button>
                  <Button
                    variant='outline'
                    disabled={page >= totalPages || isFetching}
                    onClick={() =>
                      setPage((current) => Math.min(totalPages, current + 1))
                    }
                  >
                    {t('Next')}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
