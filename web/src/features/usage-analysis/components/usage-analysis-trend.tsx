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
import { BarChart3 } from 'lucide-react'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  XAxis,
  YAxis,
} from 'recharts'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Skeleton } from '@/components/ui/skeleton'
import dayjs from '@/lib/dayjs'
import { formatTokens } from '@/lib/format'

import type { UsageAnalysisTrendDatum } from '../lib/usage-analysis'

type LabeledUsageAnalysisTrendDatum = UsageAnalysisTrendDatum & {
  label: string
}

type UsageAnalysisTrendProps = {
  data: LabeledUsageAnalysisTrendDatum[]
  start: Date
  end: Date
  isLoading: boolean
}

export function UsageAnalysisTrend(props: UsageAnalysisTrendProps) {
  const { t } = useTranslation()
  const gradientId = `usage-total-gradient-${useId().replaceAll(':', '')}`
  const chartConfig = {
    totalTokens: { label: t('Total Tokens'), color: '#8b5cf6' },
    promptTokens: { label: t('Input Tokens'), color: '#3b82f6' },
    completionTokens: { label: t('Output Tokens'), color: '#10b981' },
    cacheReadTokens: { label: t('Cache Read Tokens'), color: '#f59e0b' },
    cacheWriteTokens: { label: t('Cache Write Tokens'), color: '#ec4899' },
  } satisfies ChartConfig

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2 text-base'>
          <BarChart3 className='size-4' aria-hidden='true' />
          {t('Usage Trend')}
        </CardTitle>
        <CardDescription>
          {dayjs(props.start).format('MM/DD HH:mm')} ~{' '}
          {dayjs(props.end).format('MM/DD HH:mm')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {props.isLoading ? (
          <Skeleton className='h-[360px] w-full' />
        ) : (
          <ChartContainer
            config={chartConfig}
            className='aspect-auto h-[360px] w-full'
            initialDimension={{ width: 900, height: 360 }}
          >
            <ComposedChart
              data={props.data}
              margin={{ left: 8, right: 16 }}
              accessibilityLayer
            >
              <defs>
                <linearGradient id={gradientId} x1='0' y1='0' x2='0' y2='1'>
                  <stop
                    offset='5%'
                    stopColor='var(--color-totalTokens)'
                    stopOpacity={0.28}
                  />
                  <stop
                    offset='95%'
                    stopColor='var(--color-totalTokens)'
                    stopOpacity={0.02}
                  />
                </linearGradient>
              </defs>
              <CartesianGrid vertical={false} strokeDasharray='3 3' />
              <XAxis
                dataKey='label'
                tickLine={false}
                axisLine={false}
                minTickGap={32}
                tickMargin={10}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                width={70}
                tickFormatter={(value) => formatTokens(Number(value))}
              />
              <ChartTooltip
                cursor={{ strokeDasharray: '3 3' }}
                content={<ChartTooltipContent />}
              />
              <ChartLegend
                content={
                  <ChartLegendContent className='flex-wrap gap-x-3 gap-y-1' />
                }
              />
              <Area
                type='monotone'
                dataKey='totalTokens'
                stroke='var(--color-totalTokens)'
                fill={`url(#${gradientId})`}
                strokeWidth={2}
                dot={false}
              />
              <Line
                type='monotone'
                dataKey='promptTokens'
                stroke='var(--color-promptTokens)'
                strokeWidth={2}
                dot={false}
              />
              <Line
                type='monotone'
                dataKey='completionTokens'
                stroke='var(--color-completionTokens)'
                strokeWidth={2}
                dot={false}
              />
              <Line
                type='monotone'
                dataKey='cacheReadTokens'
                stroke='var(--color-cacheReadTokens)'
                strokeWidth={2}
                dot={false}
              />
              <Line
                type='monotone'
                dataKey='cacheWriteTokens'
                stroke='var(--color-cacheWriteTokens)'
                strokeWidth={2}
                dot={false}
              />
            </ComposedChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}
