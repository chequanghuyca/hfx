import { keepPreviousData, useQuery } from '@tanstack/react-query'
import {
  AlertTriangle,
  TrendingDown as ArrowDown,
  TrendingUp as ArrowUp,
  BarChart3,
  DollarSign,
  Percent,
} from 'lucide-react'
import { memo, useMemo, useState } from 'react'
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { api } from '../lib/api'

interface EquityPoint {
  timestamp: string
  total_equity: number
  pnl: number
  pnl_pct: number
  cycle_number: number
}

interface EquityChartProps {
  traderId?: string
  embedded?: boolean // 嵌入模式（不显示外层卡片）
  isTraderRunning?: boolean
}

export const EquityChart = memo(function EquityChart({
  traderId,
  embedded = false,
  isTraderRunning = false,
}: EquityChartProps) {
  const { language: _language } = useLanguage()
  const { user, token } = useAuth()
  const [displayMode, setDisplayMode] = useState<'dollar' | 'percent'>('dollar')

  const {
    data: history,
    error,
    isLoading,
  } = useQuery<EquityPoint[]>({
    queryKey: ['equity-history', traderId],
    queryFn: () => api.getEquityHistory(traderId),
    enabled: !!(user && token && traderId && isTraderRunning),
    staleTime: 60000,
    refetchInterval: 120000,
    placeholderData: keepPreviousData,
    notifyOnChangeProps: ['data'],
  })

  const { data: account } = useQuery({
    queryKey: ['account', traderId],
    queryFn: () => api.getAccount(traderId),
    enabled: !!(user && token && traderId && isTraderRunning),
    staleTime: 30000,
    refetchInterval: 60000,
    placeholderData: keepPreviousData,
    notifyOnChangeProps: ['data'],
  })

  // ALL HOOKS MUST BE BEFORE ANY EARLY RETURNS - React Rules of Hooks
  // Memoize filtered history to avoid recalculation on each render
  const validHistory = useMemo(
    () => history?.filter((point) => point.total_equity > 1) || [],
    [history]
  )

  // Memoize display history (limit to MAX_DISPLAY_POINTS for performance)
  const MAX_DISPLAY_POINTS = 2000
  const displayHistory = useMemo(
    () =>
      validHistory.length > MAX_DISPLAY_POINTS
        ? validHistory.slice(-MAX_DISPLAY_POINTS)
        : validHistory,
    [validHistory]
  )

  // Memoize initial balance calculation
  const initialBalance = useMemo(
    () =>
      account?.initial_balance ||
      (validHistory[0]
        ? validHistory[0].total_equity - validHistory[0].pnl
        : undefined) ||
      1000,
    [account?.initial_balance, validHistory]
  )

  // Memoize chart data transformation - this is the main optimization
  const chartData = useMemo(
    () =>
      displayHistory.map((point) => {
        const pnl = point.total_equity - initialBalance
        const pnlPct = ((pnl / initialBalance) * 100).toFixed(2)
        return {
          time: new Date(point.timestamp).toLocaleTimeString('vi-VN', {
            hour: '2-digit',
            minute: '2-digit',
          }),
          value:
            displayMode === 'dollar' ? point.total_equity : parseFloat(pnlPct),
          cycle: point.cycle_number,
          raw_equity: point.total_equity,
          raw_pnl: pnl,
          raw_pnl_pct: parseFloat(pnlPct),
        }
      }),
    [displayHistory, displayMode, initialBalance]
  )

  // Memoize current value and profit status
  const currentValue = useMemo(
    () => chartData[chartData.length - 1],
    [chartData]
  )
  const isProfit = currentValue?.raw_pnl >= 0

  // Memoize Y domain calculation
  const yDomain = useMemo(() => {
    if (!chartData.length) return [0, 100]
    if (displayMode === 'percent') {
      const values = chartData.map((d) => d.value)
      const minVal = Math.min(...values)
      const maxVal = Math.max(...values)
      const range = Math.max(Math.abs(maxVal), Math.abs(minVal))
      const padding = Math.max(range * 0.2, 1)
      return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
    } else {
      const values = chartData.map((d) => d.value)
      const minVal = Math.min(...values, initialBalance)
      const maxVal = Math.max(...values, initialBalance)
      const range = maxVal - minVal
      const padding = Math.max(range * 0.15, initialBalance * 0.01)
      return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
    }
  }, [chartData, displayMode, initialBalance])

  // EARLY RETURNS - these must come AFTER all hooks
  // Loading state - show skeleton
  if (isLoading) {
    return (
      <div className={embedded ? 'p-6' : 'binance-card p-6'}>
        {!embedded && (
          <h3
            className="text-lg font-semibold mb-6"
            style={{ color: '#EAECEF' }}
          >
            Account Equity Curve
          </h3>
        )}
        <div className="animate-pulse">
          <div className="skeleton h-64 w-full rounded"></div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className={embedded ? 'p-6' : 'binance-card p-6'}>
        <div
          className="flex items-center gap-3 p-4 rounded"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.2)',
          }}
        >
          <AlertTriangle className="w-6 h-6" style={{ color: '#F6465D' }} />
          <div>
            <div className="font-semibold" style={{ color: '#F6465D' }}>
              Loading Error
            </div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              {error.message}
            </div>
          </div>
        </div>
      </div>
    )
  }

  // Early return for empty data (after all hooks)
  if (!validHistory || validHistory.length === 0) {
    return (
      <div className={embedded ? 'p-6' : 'binance-card p-6'}>
        {!embedded && (
          <h3
            className="text-lg font-semibold mb-6"
            style={{ color: '#EAECEF' }}
          >
            Account Equity Curve
          </h3>
        )}
        <div className="text-center py-16" style={{ color: '#848E9C' }}>
          <div className="mb-4 flex justify-center opacity-50">
            <BarChart3 className="w-16 h-16" />
          </div>
          <div className="text-lg font-semibold mb-2">No Historical Data</div>
          <div className="text-sm">
            Data will appear after the first trading cycle
          </div>
        </div>
      </div>
    )
  }

  // 自定义Tooltip - Binance Style
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div
          className="rounded p-3 shadow-xl"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            Cycle #{data.cycle}
          </div>
          <div className="font-bold mono" style={{ color: '#EAECEF' }}>
            {data.raw_equity.toFixed(2)} USDT
          </div>
          <div
            className="text-sm mono font-bold"
            style={{ color: data.raw_pnl >= 0 ? '#0ECB81' : '#F6465D' }}
          >
            {data.raw_pnl >= 0 ? '+' : ''}
            {data.raw_pnl.toFixed(2)} USDT ({data.raw_pnl_pct >= 0 ? '+' : ''}
            {data.raw_pnl_pct}%)
          </div>
        </div>
      )
    }
    return null
  }

  return (
    <div
      className={
        embedded ? 'p-3 sm:p-5' : 'binance-card p-3 sm:p-5 animate-fade-in'
      }
    >
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4">
        <div className="flex-1">
          {!embedded && (
            <h3
              className="text-base sm:text-lg font-bold mb-2"
              style={{ color: '#EAECEF' }}
            >
              Account Equity Curve
            </h3>
          )}
          <div className="flex flex-col sm:flex-row sm:items-baseline gap-2 sm:gap-4">
            <span
              className="text-2xl sm:text-3xl font-bold mono"
              style={{ color: '#EAECEF' }}
            >
              {account?.total_equity.toFixed(2) || '0.00'}
              <span
                className="text-base sm:text-lg ml-1"
                style={{ color: '#848E9C' }}
              >
                USDT
              </span>
            </span>
            <div className="flex items-center gap-2 flex-wrap">
              <span
                className="text-sm sm:text-lg font-bold mono px-2 sm:px-3 py-1 rounded flex items-center gap-1"
                style={{
                  color: isProfit ? '#0ECB81' : '#F6465D',
                  background: isProfit
                    ? 'rgba(14, 203, 129, 0.1)'
                    : 'rgba(246, 70, 93, 0.1)',
                  border: `1px solid ${
                    isProfit
                      ? 'rgba(14, 203, 129, 0.2)'
                      : 'rgba(246, 70, 93, 0.2)'
                  }`,
                }}
              >
                {isProfit ? (
                  <ArrowUp className="w-4 h-4" />
                ) : (
                  <ArrowDown className="w-4 h-4" />
                )}
                {isProfit ? '+' : ''}
                {currentValue.raw_pnl_pct}%
              </span>
              <span
                className="text-xs sm:text-sm mono"
                style={{ color: '#848E9C' }}
              >
                ({isProfit ? '+' : ''}
                {currentValue.raw_pnl.toFixed(2)} USDT)
              </span>
            </div>
          </div>
        </div>

        {/* Display Mode Toggle */}
        <div
          className="flex gap-0.5 sm:gap-1 rounded p-0.5 sm:p-1 self-start sm:self-auto"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          <button
            onClick={() => setDisplayMode('dollar')}
            className="px-3 sm:px-4 py-1.5 sm:py-2 rounded text-xs sm:text-sm font-bold transition-all flex items-center gap-1"
            style={
              displayMode === 'dollar'
                ? {
                    background: '#F0B90B',
                    color: '#000',
                    boxShadow: '0 2px 8px rgba(240, 185, 11, 0.4)',
                  }
                : { background: 'transparent', color: '#848E9C' }
            }
          >
            <DollarSign className="w-4 h-4" /> USDT
          </button>
          <button
            onClick={() => setDisplayMode('percent')}
            className="px-3 sm:px-4 py-1.5 sm:py-2 rounded text-xs sm:text-sm font-bold transition-all flex items-center gap-1"
            style={
              displayMode === 'percent'
                ? {
                    background: '#F0B90B',
                    color: '#000',
                    boxShadow: '0 2px 8px rgba(240, 185, 11, 0.4)',
                  }
                : { background: 'transparent', color: '#848E9C' }
            }
          >
            <Percent className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Chart */}
      <div
        className="my-2"
        style={{
          borderRadius: '8px',
          overflow: 'hidden',
          position: 'relative',
        }}
      >
        <ResponsiveContainer width="100%" height={280}>
          <LineChart
            data={chartData}
            margin={{ top: 10, right: 20, left: 5, bottom: 30 }}
          >
            <defs>
              <linearGradient id="colorGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#F0B90B" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#FCD535" stopOpacity={0.2} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="#2B3139" />
            <XAxis
              dataKey="time"
              stroke="#5E6673"
              tick={{ fill: '#848E9C', fontSize: 11 }}
              tickLine={{ stroke: '#2B3139' }}
              interval={Math.floor(chartData.length / 10)}
              angle={-15}
              textAnchor="end"
              height={60}
            />
            <YAxis
              stroke="#5E6673"
              tick={{ fill: '#848E9C', fontSize: 12 }}
              tickLine={{ stroke: '#2B3139' }}
              domain={yDomain}
              tickFormatter={(value) =>
                displayMode === 'dollar' ? `$${value.toFixed(0)}` : `${value}%`
              }
            />
            <Tooltip content={<CustomTooltip />} />
            <ReferenceLine
              y={displayMode === 'dollar' ? initialBalance : 0}
              stroke="#474D57"
              strokeDasharray="3 3"
              label={{
                value: displayMode === 'dollar' ? initialBalance : 0,
                fill: '#848E9C',
                fontSize: 12,
              }}
            />
            <Line
              type="natural"
              dataKey="value"
              stroke="url(#colorGradient)"
              strokeWidth={3}
              dot={chartData.length > 50 ? false : { fill: '#F0B90B', r: 3 }}
              activeDot={{
                r: 6,
                fill: '#FCD535',
                stroke: '#F0B90B',
                strokeWidth: 2,
              }}
              connectNulls={true}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Footer Stats */}
      <div
        className="mt-3 grid grid-cols-2 sm:grid-cols-4 gap-2 sm:gap-3 pt-3"
        style={{ borderTop: '1px solid #2B3139' }}
      >
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(240, 185, 11, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: '#848E9C' }}
          >
            Initial Balance
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: '#EAECEF' }}
          >
            {initialBalance.toFixed(2)} USDT
          </div>
        </div>
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(240, 185, 11, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: '#848E9C' }}
          >
            Current Equity
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: '#EAECEF' }}
          >
            {currentValue.raw_equity.toFixed(2)} USDT
          </div>
        </div>
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(240, 185, 11, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: '#848E9C' }}
          >
            Historical Cycles
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: '#EAECEF' }}
          >
            {validHistory.length} Cycles
          </div>
        </div>
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(240, 185, 11, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: '#848E9C' }}
          >
            Display Range
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: '#EAECEF' }}
          >
            {validHistory.length > MAX_DISPLAY_POINTS
              ? `Recent ${MAX_DISPLAY_POINTS}`
              : 'All Data'}
          </div>
        </div>
      </div>
    </div>
  )
})
