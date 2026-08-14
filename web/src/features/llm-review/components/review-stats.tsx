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
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { getLLMReviewQueueSummary } from '../api'
import { formatFailureRate, formatWaitingSeconds } from '../lib/format'

function StatBadge(props: {
  label: string
  value: string | number
  accent: string
}) {
  return (
    <span className='border-border/60 bg-muted/25 inline-flex h-7 items-center gap-2 rounded-md border px-2.5 text-xs shadow-xs'>
      <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='text-foreground/85 font-mono font-semibold tabular-nums'>
        {props.value}
      </span>
    </span>
  )
}

export function ReviewQueueStats() {
  const { t } = useTranslation()

  const { data, isLoading } = useQuery({
    queryKey: ['llm-review-queue-summary'],
    queryFn: getLLMReviewQueueSummary,
    staleTime: 15_000,
  })

  const stats = data?.data

  if (isLoading) {
    return (
      <div className='flex items-center gap-2'>
        <Skeleton className='h-7 w-[100px] rounded-md' />
        <Skeleton className='h-7 w-[110px] rounded-md' />
        <Skeleton className='h-7 w-[130px] rounded-md' />
      </div>
    )
  }

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <StatBadge
        label={t('Pending')}
        value={stats?.pending ?? 0}
        accent='bg-sky-500/70'
      />
      <StatBadge
        label={t('Reviewing')}
        value={stats?.reviewing ?? 0}
        accent='bg-amber-500/70'
      />
      <StatBadge
        label={t('Oldest Wait')}
        value={formatWaitingSeconds(stats?.oldest_waiting_seconds)}
        accent='bg-slate-400/70'
      />
      <StatBadge
        label={t('Recent Failure Rate')}
        value={formatFailureRate(stats?.recent_failure_rate)}
        accent='bg-rose-500/65'
      />
      <StatBadge
        label={t('Merged Events')}
        value={stats?.merged_events ?? 0}
        accent='bg-emerald-500/65'
      />
    </div>
  )
}
