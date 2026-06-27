import { keepPreviousData, useQuery } from '@tanstack/react-query'
import {
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  RefreshCw,
} from 'lucide-react'
import { memo, useEffect, useMemo, useState } from 'react'
import Skeleton, { SkeletonTheme } from 'react-loading-skeleton'
import { useLanguage } from '../contexts/LanguageContext'
import { api } from '../lib/api'
import type {
  DirectionStats,
  HistoricalPosition,
  PositionHistoryResponse,
  SymbolStats,
} from '../types'
import { MetricTooltip } from './MetricTooltip'
import { CustomSelect } from './ui/CustomSelect'

// Position History Skeleton Loader
const PositionHistorySkeleton = () => {
  return (
    <SkeletonTheme baseColor="#1E2329" highlightColor="#2B3139">
      <div className="space-y-4 sm:space-y-6">
        {/* Stat Cards Skeleton */}
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-2 sm:gap-3 md:gap-4">
          {[...Array(10)].map((_, i) => (
            <div
              key={i}
              className="rounded-lg p-3 sm:p-4 bg-[#0B0E11] border border-nofx-gold/20 flex flex-col gap-2"
            >
              <div className="flex items-center gap-2">
                <Skeleton width={64} height={12} className="opacity-60" />
              </div>
              <div className="flex items-baseline gap-1">
                <Skeleton width={80} height={24} />
              </div>
              <Skeleton width={96} height={10} className="opacity-40 mt-1" />
            </div>
          ))}
        </div>

        {/* Direction Stats Skeleton */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {[...Array(2)].map((_, i) => (
            <div
              key={i}
              className="rounded-lg p-3 sm:p-4 border border-white/10 bg-[#0B0E11]"
            >
              <Skeleton width={96} height={24} className="mb-4" />
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 sm:gap-4">
                {[...Array(4)].map((_, j) => (
                  <div key={j} className="space-y-2">
                    <Skeleton width={48} height={12} />
                    <Skeleton width={64} height={16} />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>

        {/* Leaderboard Skeleton */}
        <div className="rounded-lg border border-nofx-gold/20 bg-[#0B0E11] overflow-hidden">
          <div className="h-12 flex items-center justify-center border-b border-white/5">
            <Skeleton width={128} height={24} />
          </div>
          <div className="p-1 space-y-1">
            {[...Array(5)].map((_, i) => (
              <div
                key={i}
                className="flex justify-between items-center px-4 py-3 border-b border-white/5 last:border-0"
              >
                <div className="flex gap-3 items-center">
                  <Skeleton width={80} height={20} />
                  <Skeleton width={48} height={12} />
                </div>
                <div className="flex gap-6">
                  <div className="space-y-1">
                    <Skeleton width={48} height={8} />
                    <Skeleton width={64} height={16} />
                  </div>
                  <div className="space-y-1">
                    <Skeleton width={48} height={8} />
                    <Skeleton width={64} height={16} />
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Position List Skeleton */}
        <div className="rounded-lg border border-nofx-gold/20 bg-[#0B0E11] overflow-hidden">
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-4 p-3 sm:p-4 border-b border-white/5">
            <div className="flex items-center gap-4">
              <Skeleton width={128} height={40} />
              <Skeleton width={128} height={40} />
            </div>
            <div className="flex items-center gap-2 sm:ml-auto">
              <Skeleton width={128} height={40} />
              <Skeleton width={40} height={40} />
            </div>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-[#0B0E11]">
                <tr>
                  {[...Array(9)].map((_, i) => (
                    <th key={i} className="py-3 px-4">
                      <Skeleton width={60} height={12} />
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {[...Array(5)].map((_, i) => (
                  <tr key={i} className="border-b border-white/5">
                    <td className="py-3 px-4">
                      <div className="flex gap-2">
                        <Skeleton width={64} height={20} />
                        <Skeleton width={48} height={20} />
                      </div>
                    </td>
                    {[...Array(8)].map((_, j) => (
                      <td key={j} className="py-3 px-4">
                        <div className="flex justify-end">
                          <Skeleton width={64} height={16} />
                        </div>
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </SkeletonTheme>
  )
}

interface PositionHistoryProps {
  traderId: string
  isTraderRunning?: boolean
}

// Format number with proper decimals
function formatNumber(value: number, decimals: number = 2): string {
  if (Math.abs(value) >= 1000000) {
    return (value / 1000000).toFixed(2) + 'M'
  }
  if (Math.abs(value) >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  }
  return value.toFixed(decimals)
}

// Format price with proper decimals
function formatPrice(price: number): string {
  if (!price || price === 0) return '-'
  if (price >= 1000) return price.toFixed(2)
  if (price >= 1) return price.toFixed(4)
  return price.toFixed(6)
}

// Format duration from minutes
function formatDuration(minutes: number): string {
  if (!minutes || minutes <= 0) return '-'
  if (minutes < 60) return `${minutes.toFixed(0)}m`
  if (minutes < 1440) return `${(minutes / 60).toFixed(1)}h`
  return `${(minutes / 1440).toFixed(1)}d`
}

// Format date
function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return '-'
  // sửa thành giờ Việt Nam
  return date.toLocaleDateString('vi-VN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// Stats Card Component with formula tooltip - Memoized
const StatCard = memo(function StatCard({
  title,
  value,
  suffix,
  color,
  subtitle,
  metricKey,
  language = 'en',
}: {
  title: string
  value: string | number
  suffix?: string
  color?: string
  subtitle?: string
  metricKey?: string
  language?: string
}) {
  return (
    <div className="rounded-lg p-3 sm:p-4 transition-colors duration-200 bg-[#0B0E11] border border-nofx-gold/20 shadow-slate-400">
      <div className="flex items-center gap-1 sm:gap-2 mb-1 sm:mb-2">
        {/* <span className="text-base sm:text-lg">{icon}</span> */}
        <span className="text-[10px] sm:text-sm truncate text-[#b1bed0]">
          {title}
        </span>
        {metricKey && (
          <MetricTooltip metricKey={metricKey} language={language} size={12} />
        )}
      </div>
      <div className="flex items-baseline gap-1">
        <span
          className="text-base sm:text-xl font-bold font-mono"
          style={{ color: color || '#EAECEF' }}
        >
          {value}
        </span>
        {suffix && (
          <span className="text-xs sm:text-sm" style={{ color: '#848E9C' }}>
            {suffix}
          </span>
        )}
      </div>
      {subtitle && (
        <div
          className="text-[10px] sm:text-sm mt-1 truncate"
          style={{ color: '#848E9C' }}
        >
          {subtitle}
        </div>
      )}
    </div>
  )
})

// Symbol Stats Row - Memoized
const SymbolStatsRow = memo(function SymbolStatsRow({
  stat,
}: {
  stat: SymbolStats
}) {
  const totalPnl = stat.total_pnl || 0
  const winRate = stat.win_rate || 0
  const pnlColor = totalPnl >= 0 ? '#0ECB81' : '#F6465D'
  const winRateColor =
    winRate >= 60 ? '#0ECB81' : winRate >= 40 ? '#F0B90B' : '#F6465D'

  return (
    <div
      className="flex sm:flex-row items-center justify-between px-3 py-2 sm:px-4 transition-all duration-200 hover:bg-white/5 gap-2 sm:gap-0"
      style={{ borderBottom: '1px solid #2B3139' }}
    >
      <div className="flex gap-2 sm:gap-3 flex-col sm:flex-row">
        <span
          className="font-mono font-semibold text-sm sm:text-base"
          style={{ color: '#EAECEF' }}
        >
          {(stat.symbol || '').replace('USDT', '')}
        </span>
        <span className="text-[10px] sm:text-xs" style={{ color: '#848E9C' }}>
          {stat.total_trades || 0} trades
        </span>
      </div>
      <div className="flex items-center gap-4 sm:gap-6">
        <div className="text-left sm:text-right">
          <div className="text-[10px] sm:text-xs" style={{ color: '#848E9C' }}>
            Win Rate
          </div>
          <div
            className="font-mono font-semibold text-[12px] sm:text-base"
            style={{ color: winRateColor }}
          >
            {winRate.toFixed(1)}%
          </div>
        </div>
        <div className="text-left sm:text-right min-w-[60px] sm:min-w-[80px]">
          <div className="text-[10px] sm:text-xs" style={{ color: '#848E9C' }}>
            P&L
          </div>
          <div
            className="font-mono font-semibold text-[12px] sm:text-base"
            style={{ color: pnlColor }}
          >
            {totalPnl >= 0 ? '+' : ''}
            {formatNumber(totalPnl)}
          </div>
        </div>
      </div>
    </div>
  )
})

// Direction Stats Card - Memoized
const DirectionStatsCard = memo(function DirectionStatsCard({
  stat,
}: {
  stat: DirectionStats
}) {
  const isLong = (stat.side || '').toLowerCase() === 'long'
  const iconColor = isLong ? '#0ECB81' : '#F6465D'
  const totalPnl = stat.total_pnl || 0
  const winRate = stat.win_rate || 0
  const tradeCount = stat.trade_count || 0
  const avgPnl = stat.avg_pnl || 0
  const pnlColor = totalPnl >= 0 ? '#0ECB81' : '#F6465D'

  return (
    <div
      className={`rounded-lg p-3 sm:p-4 border ${isLong ? 'border-green-500 bg-green-500/10 linear-gradient(135deg, #1E2329 0%, #181C21 100%)' : 'border-red-500 bg-red-500/10 linear-gradient(135deg, #1E2329 0%, #181C21 100%)'}`}
    >
      <div className="flex items-center gap-2 mb-2 sm:mb-3">
        <span
          className="font-bold uppercase text-base sm:text-lg"
          style={{ color: iconColor }}
        >
          {stat.side || 'Unknown'}
        </span>
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 sm:gap-4">
        <div>
          <div
            className="text-[10px] sm:text-sm mb-1"
            style={{ color: '#848E9C' }}
          >
            Trades
          </div>
          <div
            className="font-mono font-semibold text-sm sm:text-base"
            style={{ color: '#EAECEF' }}
          >
            {tradeCount}
          </div>
        </div>
        <div>
          <div
            className="text-[10px] sm:text-sm mb-1"
            style={{ color: '#848E9C' }}
          >
            Win Rate
          </div>
          <div
            className="font-mono font-semibold text-sm sm:text-base"
            style={{
              color:
                winRate >= 60
                  ? '#0ECB81'
                  : winRate >= 40
                    ? '#F0B90B'
                    : '#F6465D',
            }}
          >
            {winRate.toFixed(1)}%
          </div>
        </div>
        <div>
          <div
            className="text-[10px] sm:text-xs mb-1"
            style={{ color: '#848E9C' }}
          >
            Total PnL
          </div>
          <div
            className="font-mono font-semibold text-sm sm:text-base"
            style={{ color: pnlColor }}
          >
            {totalPnl >= 0 ? '+' : ''}
            {formatNumber(totalPnl)}
          </div>
        </div>
        <div>
          <div
            className="text-[10px] sm:text-xs mb-1"
            style={{ color: '#848E9C' }}
          >
            Avg PnL
          </div>
          <div
            className="font-mono font-semibold text-sm sm:text-base"
            style={{ color: avgPnl >= 0 ? '#0ECB81' : '#F6465D' }}
          >
            {avgPnl >= 0 ? '+' : ''}
            {formatNumber(avgPnl)}
          </div>
        </div>
      </div>
    </div>
  )
})

// Position Row Component - Memoized
const PositionRow = memo(function PositionRow({
  position,
}: {
  position: HistoricalPosition
}) {
  const side = position.side || ''
  const isLong = side.toUpperCase() === 'LONG'
  const realizedPnl = position.realized_pnl || 0
  const isProfitable = realizedPnl >= 0
  const sideColor = isLong ? '#0ECB81' : '#F6465D'
  const pnlColor = isProfitable ? '#0ECB81' : '#F6465D'

  // Calculate holding time
  const entryTime = position.entry_time
    ? new Date(position.entry_time).getTime()
    : 0
  const exitTime = position.exit_time
    ? new Date(position.exit_time).getTime()
    : 0
  const holdingMinutes =
    entryTime && exitTime && exitTime > entryTime
      ? (exitTime - entryTime) / 60000
      : 0

  // Calculate PnL percentage based on entry price
  const entryPrice = position.entry_price || 0
  const exitPrice = position.exit_price || 0
  let pnlPct = 0
  if (entryPrice > 0) {
    if (isLong) {
      pnlPct = ((exitPrice - entryPrice) / entryPrice) * 100
    } else {
      pnlPct = ((entryPrice - exitPrice) / entryPrice) * 100
    }
  }

  // Use entry_quantity for display (original position size)
  const displayQty = position.entry_quantity || position.quantity || 0

  return (
    <tr
      className="transition-all duration-200 hover:bg-white/5"
      style={{ borderBottom: '1px solid #2B3139' }}
    >
      {/* Symbol */}
      <td className="py-3 px-4">
        <div className="flex items-center gap-2">
          <span
            className="font-mono font-semibold"
            style={{ color: '#EAECEF' }}
          >
            {(position.symbol || '').replace('USDT', '')}
          </span>
          <span
            className="px-2 py-0.5 rounded text-xs font-semibold uppercase"
            style={{
              background: `${sideColor}22`,
              color: sideColor,
              border: `1px solid ${sideColor}44`,
            }}
          >
            {side}
          </span>
        </div>
      </td>

      {/* Entry Price */}
      <td className="py-3 px-4 text-right font-mono">
        {formatPrice(entryPrice)}
      </td>

      {/* Exit Price */}
      <td className="py-3 px-4 text-right font-mono">
        {formatPrice(exitPrice)}
      </td>

      {/* Quantity */}
      <td className="py-3 px-4 text-right font-mono text-gray-400">
        {displayQty.toFixed(2)}
      </td>

      {/* Position Value (Entry Price * Quantity) */}
      <td
        className="py-3 px-4 text-right font-mono"
        style={{ color: '#EAECEF' }}
      >
        {formatNumber(entryPrice * displayQty)}
      </td>

      {/* P&L */}
      <td className="py-3 px-4 text-right">
        <div className="font-mono font-semibold" style={{ color: pnlColor }}>
          {isProfitable ? '+' : ''}
          {formatNumber(realizedPnl)}
        </div>
        <div className="text-xs" style={{ color: pnlColor }}>
          {pnlPct >= 0 ? '+' : ''}
          {pnlPct.toFixed(2)}%
        </div>
      </td>

      {/* Fee - show more precision for small fees */}
      <td className="py-3 px-4 text-right font-mono text-sm text-gray-400">
        -
        {(position.fee || 0) < 0.01 && (position.fee || 0) > 0
          ? (position.fee || 0).toFixed(4)
          : (position.fee || 0).toFixed(3)}
      </td>

      {/* Duration */}
      <td
        className="py-3 px-4 text-center text-sm"
        style={{ color: '#848E9C' }}
      >
        {formatDuration(holdingMinutes)}
      </td>

      {/* Exit Time */}
      <td className="py-3 px-4 text-right text-xs" style={{ color: '#848E9C' }}>
        {formatDate(position.exit_time)}
      </td>
    </tr>
  )
})

export function PositionHistory({
  traderId,
  isTraderRunning = false,
}: PositionHistoryProps) {
  const { language } = useLanguage()

  // Pagination state
  const [pageSize, setPageSize] = useState<number>(20)
  const [currentPage, setCurrentPage] = useState<number>(1)

  // Filter state
  const [filterSymbol, setFilterSymbol] = useState<string>('all')
  const [filterSide, setFilterSide] = useState<string>('all')
  const [sortBy, setSortBy] = useState<'time' | 'pnl' | 'pnl_pct'>('time')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')

  // Use React Query for data fetching with cache key for invalidation
  const { data, error, isLoading, refetch, isRefetching } = useQuery<
    PositionHistoryResponse,
    Error
  >({
    queryKey: ['position-history', traderId],
    queryFn: () =>
      api.getPositionHistory(traderId, Math.max(200, pageSize * 5)),
    enabled: !!traderId && isTraderRunning,
    staleTime: 20000,
    refetchInterval: 10000,
    placeholderData: keepPreviousData,
    notifyOnChangeProps: ['data'],
  })

  // Extract data from React Query response
  const positions = data?.positions || []
  const stats = data?.stats || null
  const symbolStats = data?.symbol_stats || []
  const directionStats = data?.direction_stats || []
  const loading = isLoading

  // Get unique symbols for filter
  const uniqueSymbols = useMemo(() => {
    const symbols = new Set(positions.map((p) => p.symbol))
    return Array.from(symbols).sort()
  }, [positions])

  // Filtered and sorted positions (before pagination)
  const filteredAndSortedPositions = useMemo(() => {
    let result = [...positions]

    // Apply filters
    if (filterSymbol !== 'all') {
      result = result.filter((p) => p.symbol === filterSymbol)
    }
    if (filterSide !== 'all') {
      result = result.filter(
        (p) => (p.side || '').toUpperCase() === filterSide.toUpperCase()
      )
    }

    // Apply sorting
    result.sort((a, b) => {
      let comparison = 0
      switch (sortBy) {
        case 'time':
          comparison =
            new Date(a.exit_time || 0).getTime() -
            new Date(b.exit_time || 0).getTime()
          break
        case 'pnl':
          comparison = (a.realized_pnl || 0) - (b.realized_pnl || 0)
          break
        case 'pnl_pct': {
          const aPrice = a.entry_price || 1
          const bPrice = b.entry_price || 1
          const aPct = (((a.exit_price || 0) - aPrice) / aPrice) * 100
          const bPct = (((b.exit_price || 0) - bPrice) / bPrice) * 100
          comparison = aPct - bPct
          break
        }
      }
      return sortOrder === 'desc' ? -comparison : comparison
    })

    return result
  }, [positions, filterSymbol, filterSide, sortBy, sortOrder])

  // Pagination calculations
  const totalFilteredCount = filteredAndSortedPositions.length
  const totalPages = Math.ceil(totalFilteredCount / pageSize)

  // Reset to page 1 when filters change
  useEffect(() => {
    setCurrentPage(1)
  }, [filterSymbol, filterSide, sortBy, sortOrder, pageSize])

  // Paginated positions (for display)
  const paginatedPositions = useMemo(() => {
    const startIndex = (currentPage - 1) * pageSize
    return filteredAndSortedPositions.slice(startIndex, startIndex + pageSize)
  }, [filteredAndSortedPositions, currentPage, pageSize])

  // For backwards compatibility, keep filteredPositions as the paginated result
  const filteredPositions = paginatedPositions

  // Calculate profit/loss ratio (avg win / avg loss)
  const profitLossRatio = useMemo(() => {
    if (!stats) return 0
    const avgWin = stats.avg_win || 0
    const avgLoss = stats.avg_loss || 0
    if (avgLoss === 0) return avgWin > 0 ? Infinity : 0
    return avgWin / avgLoss
  }, [stats])

  if (loading) {
    return <PositionHistorySkeleton />
  }

  if (error) {
    return (
      <div
        className="rounded-lg p-6 text-center"
        style={{
          background: 'rgba(246, 70, 93, 0.1)',
          border: '1px solid rgba(246, 70, 93, 0.3)',
          color: '#F6465D',
        }}
      >
        {error.message || 'An error occurred'}
      </div>
    )
  }

  if (positions.length === 0) {
    return (
      <div
        className="rounded-lg p-12 text-center"
        style={{
          background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
          border: '1px solid #2B3139',
        }}
      >
        <div className="text-4xl mb-4">📊</div>
        <div
          className="text-lg font-semibold mb-2"
          style={{ color: '#EAECEF' }}
        >
          No Position History
        </div>
        <div style={{ color: '#848E9C' }}>
          Closed positions will appear here
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* Overall Stats - All Metrics Combined */}
      {stats && (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-2 sm:gap-3 md:gap-4">
          {/* Row 1: Core Performance */}
          <StatCard
            title="Trades"
            value={stats.total_trades || 0}
            subtitle={`W${stats.win_trades || 0} / L${stats.loss_trades || 0}`}
            language={language}
          />
          <StatCard
            title="Win Rate"
            value={(stats.win_rate || 0).toFixed(1)}
            suffix="%"
            color={
              (stats.win_rate || 0) >= 60
                ? '#0ECB81'
                : (stats.win_rate || 0) >= 40
                  ? '#F0B90B'
                  : '#F6465D'
            }
            metricKey="win_rate"
            language={language}
          />
          <StatCard
            title="Total PnL"
            value={
              ((stats.total_pnl || 0) >= 0 ? '+' : '') +
              formatNumber(stats.total_pnl || 0)
            }
            color={(stats.total_pnl || 0) >= 0 ? '#0ECB81' : '#F6465D'}
            subtitle={`Fee: -${formatNumber(stats.total_fee || 0)}`}
            metricKey="total_return"
            language={language}
          />
          <StatCard
            title="Net PnL"
            value={
              ((stats.total_pnl || 0) - (stats.total_fee || 0) >= 0
                ? '+'
                : '') +
              formatNumber((stats.total_pnl || 0) - (stats.total_fee || 0))
            }
            color={
              (stats.total_pnl || 0) - (stats.total_fee || 0) >= 0
                ? '#0ECB81'
                : '#F6465D'
            }
            subtitle="After fees"
            language={language}
          />
          {/* Row 2: Risk Metrics */}
          <StatCard
            title="Profit Factor"
            value={(stats.profit_factor || 0).toFixed(2)}
            color={
              (stats.profit_factor || 0) >= 1.5
                ? '#0ECB81'
                : (stats.profit_factor || 0) >= 1
                  ? '#F0B90B'
                  : '#F6465D'
            }
            subtitle="Profit / Loss"
            metricKey="profit_factor"
            language={language}
          />
          <StatCard
            title="P/L Ratio"
            value={
              profitLossRatio === Infinity ? '∞' : profitLossRatio.toFixed(2)
            }
            color={
              profitLossRatio >= 1.5
                ? '#0ECB81'
                : profitLossRatio >= 1
                  ? '#F0B90B'
                  : '#F6465D'
            }
            subtitle="Win / Loss"
            metricKey="expectancy"
            language={language}
          />
          <StatCard
            title="Sharpe"
            value={(stats.sharpe_ratio || 0).toFixed(2)}
            color={
              (stats.sharpe_ratio || 0) >= 1
                ? '#0ECB81'
                : (stats.sharpe_ratio || 0) >= 0
                  ? '#F0B90B'
                  : '#F6465D'
            }
            subtitle="Risk-adjusted"
            metricKey="sharpe_ratio"
            language={language}
          />
          <StatCard
            title="Max DD"
            value={(stats.max_drawdown_pct || 0).toFixed(1)}
            suffix="%"
            color={
              (stats.max_drawdown_pct || 0) <= 10
                ? '#0ECB81'
                : (stats.max_drawdown_pct || 0) <= 20
                  ? '#F0B90B'
                  : '#F6465D'
            }
            metricKey="max_drawdown"
            language={language}
          />
          {/* Row 3: Win/Loss Averages */}
          <StatCard
            title="Avg Win"
            value={'+' + formatNumber(stats.avg_win || 0)}
            color="#0ECB81"
            metricKey="avg_trade_pnl"
            language={language}
          />
          <StatCard
            title="Avg Loss"
            value={'-' + formatNumber(stats.avg_loss || 0)}
            color="#F6465D"
            language={language}
          />
        </div>
      )}

      {/* Direction Stats */}
      {directionStats.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {directionStats.map((stat) => (
            <DirectionStatsCard key={stat.side} stat={stat} />
          ))}
        </div>
      )}

      {/* Symbol Performance */}
      {symbolStats.length > 0 && (
        <div
          className="rounded-lg border border-nofx-gold/20"
          style={{
            background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
          }}
        >
          <div className="flex items-center justify-center p-2 sm:p-4">
            <span className="font-semibold sm:text-2xl text-xl text-nofx-gold shadow-nofx-gold-highlight">
              Leaderboard
            </span>
          </div>
          <div className="space-y-1">
            {symbolStats.slice(0, 10).map((stat) => (
              <SymbolStatsRow key={stat.symbol} stat={stat} />
            ))}
          </div>
        </div>
      )}

      {/* Position List */}
      <div
        className="rounded-lg overflow-hidden border border-nofx-gold/20"
        style={{
          background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
        }}
      >
        {/* Filters */}
        <div className="flex flex-col sm:flex-row flex-wrap items-stretch sm:items-center gap-3 sm:gap-4 p-3 sm:p-4">
          {/* Row 1 on mobile: Symbol and Side filters */}
          <div className="flex flex-wrap items-center gap-2 sm:gap-4">
            <div className="flex items-center gap-2">
              <span className="text-xs sm:text-sm" style={{ color: '#848E9C' }}>
                Symbol:
              </span>
              <CustomSelect
                value={filterSymbol}
                onChange={(val) => setFilterSymbol(String(val))}
                options={[
                  { value: 'all', label: 'All' },
                  ...uniqueSymbols.map((symbol) => ({
                    value: symbol,
                    label: (symbol || '').replace('USDT', ''),
                  })),
                ]}
                className="w-32"
                searchable={true}
                maxHeight="300px"
              />
            </div>

            <div className="flex items-center gap-2">
              <span
                className="text-xs sm:text-sm hidden sm:inline"
                style={{ color: '#848E9C' }}
              >
                Side:
              </span>
              <div
                className="flex rounded overflow-hidden"
                style={{ border: '1px solid #2B3139' }}
              >
                {['all', 'LONG', 'SHORT'].map((side) => (
                  <button
                    key={side}
                    onClick={() => setFilterSide(side)}
                    className="px-2 sm:px-3 py-1 sm:py-1.5 text-xs sm:text-sm capitalize transition-colors"
                    style={{
                      background:
                        filterSide === side ? '#2B3139' : 'transparent',
                      color: filterSide === side ? '#EAECEF' : '#848E9C',
                    }}
                  >
                    {side === 'all' ? 'All' : side}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* Row 2 on mobile: Sort and Refresh */}
          <div className="flex items-center gap-2 sm:ml-auto">
            <span className="text-xs sm:text-sm" style={{ color: '#848E9C' }}>
              Sort:
            </span>
            <CustomSelect
              value={`${sortBy}-${sortOrder}`}
              onChange={(val) => {
                const [by, order] = String(val).split('-') as [
                  'time' | 'pnl' | 'pnl_pct',
                  'asc' | 'desc',
                ]
                setSortBy(by)
                setSortOrder(order)
              }}
              options={[
                { value: 'time-desc', label: 'Latest' },
                { value: 'time-asc', label: 'Oldest' },
                { value: 'pnl-desc', label: 'Best PnL' },
                { value: 'pnl-asc', label: 'Worst PnL' },
              ]}
              className="w-32 flex-1 sm:flex-none"
            />

            <button
              onClick={() => refetch()}
              disabled={loading || isRefetching}
              className={`p-2 rounded-lg transition-all border border-white/10 bg-white/5 ${
                isRefetching ? 'opacity-70' : 'hover:bg-white/10'
              }`}
              title="Refresh"
            >
              <RefreshCw
                className={`${isRefetching ? 'animate-spin' : ''} w-4 h-4`}
              />
            </button>
          </div>
        </div>

        {/* Table - Desktop */}
        <div className="hidden md:block overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr style={{ background: '#0B0E11' }}>
                <th
                  className="py-3 px-4 text-left text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  Symbol
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  Entry
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  Exit
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  Qty
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  Value
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  PnL
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  Fee
                </th>
                <th
                  className="py-3 px-4 text-center text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  Duration
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  Closed At
                </th>
              </tr>
            </thead>
            <tbody>
              {filteredPositions.map((position) => (
                <PositionRow key={position.id} position={position} />
              ))}
            </tbody>
          </table>
        </div>

        {/* Mobile Card View */}
        <div className="md:hidden border-t border-white/10">
          {filteredPositions.map((position) => {
            const side = position.side || ''
            const isLong = side.toUpperCase() === 'LONG'
            const realizedPnl = position.realized_pnl || 0
            const isProfitable = realizedPnl >= 0
            const sideColor = isLong ? '#0ECB81' : '#F6465D'
            const pnlColor = isProfitable ? '#0ECB81' : '#F6465D'
            const entryPrice = position.entry_price || 0
            const exitPrice = position.exit_price || 0
            const displayQty = position.entry_quantity || position.quantity || 0
            let pnlPct = 0
            if (entryPrice > 0) {
              if (isLong) {
                pnlPct = ((exitPrice - entryPrice) / entryPrice) * 100
              } else {
                pnlPct = ((entryPrice - exitPrice) / entryPrice) * 100
              }
            }
            const entryTime = position.entry_time
              ? new Date(position.entry_time).getTime()
              : 0
            const exitTime = position.exit_time
              ? new Date(position.exit_time).getTime()
              : 0
            const holdingMinutes =
              entryTime && exitTime && exitTime > entryTime
                ? (exitTime - entryTime) / 60000
                : 0

            return (
              <div
                key={position.id}
                className="p-3 hover:bg-white/5 transition-colors border-b border-white/10"
              >
                {/* Row 1: Symbol, Side, PnL */}
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <span
                      className="font-mono font-semibold"
                      style={{ color: '#EAECEF' }}
                    >
                      {(position.symbol || '').replace('USDT', '')}
                    </span>
                    <span
                      className="px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase"
                      style={{
                        background: `${sideColor}22`,
                        color: sideColor,
                        border: `1px solid ${sideColor}44`,
                      }}
                    >
                      {side}
                    </span>
                  </div>
                  <div className="text-right">
                    <div
                      className="font-mono font-semibold text-sm"
                      style={{ color: pnlColor }}
                    >
                      {isProfitable ? '+' : ''}
                      {formatNumber(realizedPnl)}
                    </div>
                    <div className="text-[10px]" style={{ color: pnlColor }}>
                      {pnlPct >= 0 ? '+' : ''}
                      {pnlPct.toFixed(2)}%
                    </div>
                  </div>
                </div>

                {/* Row 2: Entry/Exit prices */}
                <div className="grid grid-cols-2 gap-2 text-xs mb-2">
                  <div>
                    <span style={{ color: '#848E9C' }}>Entry: </span>
                    <span className="font-mono" style={{ color: '#EAECEF' }}>
                      {formatPrice(entryPrice)}
                    </span>
                  </div>
                  <div>
                    <span style={{ color: '#848E9C' }}>Exit: </span>
                    <span className="font-mono" style={{ color: '#EAECEF' }}>
                      {formatPrice(exitPrice)}
                    </span>
                  </div>
                </div>

                {/* Row 3: Qty, Value, Fee, Duration */}
                <div
                  className="flex flex-wrap gap-x-4 gap-y-1 text-[10px]"
                  style={{ color: '#848E9C' }}
                >
                  <span>
                    Qty:{' '}
                    <span className="font-mono">{displayQty.toFixed(2)}</span>
                  </span>
                  <span>
                    Value:{' '}
                    <span className="font-mono" style={{ color: '#EAECEF' }}>
                      {formatNumber(entryPrice * displayQty)}
                    </span>
                  </span>
                  <span>
                    Fee:{' '}
                    <span className="font-mono">
                      -{(position.fee || 0).toFixed(3)}
                    </span>
                  </span>
                  <span>
                    Duration:{' '}
                    <span className="font-mono">
                      {formatDuration(holdingMinutes)}
                    </span>
                  </span>
                </div>

                {/* Row 4: Closed At */}
                <div className="text-[10px] mt-1" style={{ color: '#6B7280' }}>
                  Closed: {formatDate(position.exit_time)}
                </div>
              </div>
            )
          })}
        </div>

        {/* Footer with Pagination */}
        <div
          className="flex flex-col sm:flex-row flex-wrap items-start sm:items-center justify-between gap-3 sm:gap-4 p-3 sm:p-4 text-xs sm:text-sm"
          style={{ borderTop: '1px solid #2B3139', color: '#848E9C' }}
        >
          {/* Left: Count info */}
          <div className="flex flex-wrap items-center gap-2 sm:gap-4">
            <span>
              {totalFilteredCount}/{positions.length} positions
            </span>
            {totalFilteredCount > 0 && (
              <span>
                PnL:{' '}
                <span
                  style={{
                    color:
                      filteredAndSortedPositions.reduce(
                        (sum, p) => sum + (p.realized_pnl || 0),
                        0
                      ) >= 0
                        ? '#0ECB81'
                        : '#F6465D',
                  }}
                >
                  {filteredAndSortedPositions.reduce(
                    (sum, p) => sum + (p.realized_pnl || 0),
                    0
                  ) >= 0
                    ? '+'
                    : ''}
                  {formatNumber(
                    filteredAndSortedPositions.reduce(
                      (sum, p) => sum + (p.realized_pnl || 0),
                      0
                    )
                  )}
                </span>
              </span>
            )}
          </div>

          {/* Right: Pagination controls */}
          <div className="flex items-center gap-2 sm:gap-3 w-full sm:w-auto justify-between sm:justify-end">
            {/* Page size selector */}
            <div className="flex items-center gap-1 sm:gap-2">
              <span
                className="text-[10px] sm:text-xs"
                style={{ color: '#848E9C' }}
              >
                Per page:
              </span>
              <CustomSelect
                value={pageSize}
                onChange={(val) => setPageSize(Number(val))}
                options={[
                  { value: 20, label: '20' },
                  { value: 50, label: '50' },
                  { value: 100, label: '100' },
                ]}
                className="w-20"
              />
            </div>

            {/* Page navigation */}
            {totalPages > 1 && (
              <div className="flex items-center gap-0.5 sm:gap-1">
                <button
                  onClick={() => setCurrentPage(1)}
                  disabled={currentPage === 1}
                  className="px-1.5 sm:px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === 1 ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  <ChevronsLeft className="w-4 h-4" />
                </button>
                <button
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                  className="px-1.5 sm:px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === 1 ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>
                <span
                  className="px-2 sm:px-3 text-[11px] sm:text-xs"
                  style={{ color: '#EAECEF' }}
                >
                  {currentPage}/{totalPages}
                </span>
                <button
                  onClick={() =>
                    setCurrentPage((p) => Math.min(totalPages, p + 1))
                  }
                  disabled={currentPage === totalPages}
                  className="px-1.5 sm:px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background:
                      currentPage === totalPages ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  <ChevronRight className="w-4 h-4" />
                </button>
                <button
                  onClick={() => setCurrentPage(totalPages)}
                  disabled={currentPage === totalPages}
                  className="px-1.5 sm:px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background:
                      currentPage === totalPages ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  <ChevronsRight className="w-4 h-4" />
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
