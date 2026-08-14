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
import { AlertCircle, RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import dayjs from '@/lib/dayjs'

import {
  getUsageAnalysis,
  getUsageAnalysisOptions,
  type UsageAnalysisParams,
} from './api'
import { UsageAnalysisBreakdown } from './components/usage-analysis-breakdown'
import { UsageAnalysisOverview } from './components/usage-analysis-overview'
import { UsageAnalysisTrend } from './components/usage-analysis-trend'
import {
  areUsageAnalysisSelectionsEqual,
  buildUsageAnalysisTrendData,
  getTodayUsageAnalysisRange,
  hasSameUsageAnalysisDataScope,
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
    placeholderData: (previousData, previousQuery) => {
      const previousParams = previousQuery?.queryKey[1] as
        | UsageAnalysisParams
        | undefined
      return hasSameUsageAnalysisDataScope(queryParams, previousParams)
        ? previousData
        : undefined
    },
  })
  const analysis = analysisQuery.data?.data
  const totalPages = Math.max(
    1,
    Math.ceil((analysis?.total ?? 0) / DEFAULT_PAGE_SIZE)
  )

  const usersById = useMemo(
    () => new Map((options?.users ?? []).map((user) => [user.id, user.name])),
    [options?.users]
  )
  const tokensById = useMemo(
    () =>
      new Map((options?.tokens ?? []).map((token) => [token.id, token.name])),
    [options?.tokens]
  )
  const channelsById = useMemo(
    () =>
      new Map(
        (options?.channels ?? []).map((channel) => [channel.id, channel.name])
      ),
    [options?.channels]
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

  const applyFilters = () => {
    if (filters.range.end < filters.range.start) {
      toast.error(t('End date must be after start date.'))
      return
    }
    const filtersUnchanged =
      areUsageAnalysisSelectionsEqual(filters, appliedFilters) && page === 1
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

  const selectedFilterUserName =
    filters.userId === ALL
      ? t('All Users')
      : (usersById.get(Number(filters.userId)) ?? t('Select User'))
  const selectedFilterTokenName =
    filters.tokenId === ALL
      ? t('All API Keys')
      : (tokensById.get(Number(filters.tokenId)) ?? t('API Key'))
  const selectedFilterModelName =
    filters.modelName === ALL ? t('All Models') : filters.modelName
  const selectedFilterChannelName =
    filters.channelId === ALL
      ? t('All Channels')
      : (channelsById.get(Number(filters.channelId)) ?? t('Channel'))
  const selectedUserName =
    appliedFilters.userId === ALL
      ? t('All Users')
      : (usersById.get(Number(appliedFilters.userId)) ??
        `#${appliedFilters.userId}`)
  const selectedTokenName =
    appliedFilters.tokenId === ALL
      ? t('All API Keys')
      : (tokensById.get(Number(appliedFilters.tokenId)) ??
        `#${appliedFilters.tokenId}`)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Usage Statistics')}</span>
          <Badge variant='outline' className='shrink-0'>
            {t('Root Only')}
          </Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <div className='flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between'>
            <p className='text-muted-foreground text-sm'>
              {t('View AI model usage and cost statistics')}
            </p>
            <div className='flex flex-wrap items-center gap-2 xl:justify-end'>
              <Select value={filters.userId} onValueChange={selectUser}>
                <SelectTrigger
                  className='min-w-32'
                  aria-label={t('Select User')}
                >
                  <SelectValue placeholder={t('Select User')}>
                    {selectedFilterUserName}
                  </SelectValue>
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
                onValueChange={(value) => {
                  if (value === null) return
                  setFilters((current) => ({ ...current, tokenId: value }))
                }}
              >
                <SelectTrigger className='min-w-32' aria-label={t('API Key')}>
                  <SelectValue placeholder={t('API Key')}>
                    {selectedFilterTokenName}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value={ALL}>{t('All API Keys')}</SelectItem>
                    {visibleTokens.map((token) => (
                      <SelectItem key={token.id} value={String(token.id)}>
                        {token.name}
                        {filters.userId === ALL && (
                          <span className='text-muted-foreground'>
                            {usersById.get(token.user_id)}
                          </span>
                        )}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>

              <Select
                value={filters.modelName}
                onValueChange={(value) => {
                  if (value === null) return
                  setFilters((current) => ({ ...current, modelName: value }))
                }}
              >
                <SelectTrigger className='min-w-36' aria-label={t('Model')}>
                  <SelectValue placeholder={t('Model')}>
                    {selectedFilterModelName}
                  </SelectValue>
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
                onValueChange={(value) => {
                  if (value === null) return
                  setFilters((current) => ({ ...current, channelId: value }))
                }}
              >
                <SelectTrigger className='min-w-36' aria-label={t('Channel')}>
                  <SelectValue placeholder={t('Channel')}>
                    {selectedFilterChannelName}
                  </SelectValue>
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
                  data-icon='inline-start'
                  className={isFetching ? 'animate-spin' : ''}
                  aria-hidden='true'
                />
                {t('Refresh')}
              </Button>
            </div>
          </div>

          {errorMessage && (
            <Alert variant='destructive'>
              <AlertCircle aria-hidden='true' />
              <AlertDescription>{errorMessage}</AlertDescription>
            </Alert>
          )}

          <UsageAnalysisOverview
            summary={analysis?.summary}
            selectedUserName={selectedUserName}
            selectedTokenName={selectedTokenName}
            isLoading={isLoading}
          />

          <UsageAnalysisTrend
            data={trendData}
            start={appliedFilters.range.start}
            end={appliedFilters.range.end}
            isLoading={isLoading}
          />

          <UsageAnalysisBreakdown
            rows={analysis?.rows ?? []}
            page={page}
            totalPages={totalPages}
            isLoading={isLoading}
            isFetching={isFetching}
            onPrevious={() => setPage((current) => Math.max(1, current - 1))}
            onNext={() =>
              setPage((current) => Math.min(totalPages, current + 1))
            }
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
