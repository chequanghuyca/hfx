import { Loader2, LogOut } from 'lucide-react'
import { memo, useCallback, useMemo, useState } from 'react'
import type { Position } from '../types'
import { CustomSelect } from './ui/CustomSelect'

interface PositionsTableProps {
  positions: Position[]
  onSymbolClick: (symbol: string) => void
  onClosePosition: (symbol: string, side: string) => Promise<void>
}

/**
 * PositionsTable - Memoized component to display current positions
 * Prevents unnecessary re-renders of parent components (like ChartTabs)
 * when positions data changes.
 */
export const PositionsTable = memo(function PositionsTable({
  positions,
  onSymbolClick,
  onClosePosition,
}: PositionsTableProps) {
  const [closingPosition, setClosingPosition] = useState<string | null>(null)
  const [pageSize, setPageSize] = useState<number>(20)
  const [currentPage, setCurrentPage] = useState<number>(1)

  // Paginated positions with useMemo
  const totalPositions = Array.isArray(positions) ? positions.length : 0
  const totalPages = Math.ceil(totalPositions / pageSize)
  const paginatedPositions = useMemo(
    () =>
      Array.isArray(positions)
        ? positions.slice((currentPage - 1) * pageSize, currentPage * pageSize)
        : [],
    [positions, currentPage, pageSize]
  )

  // Total PnL calculation
  const totalPnL = useMemo(
    () =>
      Array.isArray(positions)
        ? positions.reduce((acc, pos) => acc + pos.unrealized_pnl, 0)
        : 0,
    [positions]
  )

  // Memoized close position handler
  const handleClosePosition = useCallback(
    async (symbol: string, side: string) => {
      setClosingPosition(symbol)
      try {
        await onClosePosition(symbol, side)
      } finally {
        setClosingPosition(null)
      }
    },
    [onClosePosition]
  )

  if (!positions || positions.length === 0) {
    return (
      <div className="text-center py-16 text-nofx-text-muted opacity-60">
        <div className="text-lg font-semibold mb-2">No Positions</div>
        <div className="text-sm">
          Please check your strategy settings and try again.
        </div>
      </div>
    )
  }

  return (
    <div>
      {/* Header Stats */}
      <div className="flex sm:flex-row flex-col sm:items-center items-start sm:justify-between justify-start gap-2 mb-5 relative z-10">
        <h2 className="text-lg font-bold flex items-center gap-2 text-nofx-text-main uppercase tracking-wide">
          Current Positions
        </h2>
        <div className="flex gap-2">
          <div className="text-xs px-2 py-1 rounded bg-nofx-gold/10 text-nofx-gold border border-nofx-gold/20 font-mono shadow-[0_0_10px_rgba(240,185,11,0.1)]">
            {positions.length} ACTIVE
          </div>
          <div
            className={
              totalPnL >= 0
                ? 'text-xs px-2 py-1 rounded bg-green-500/30 text-green-400 border border-green-700 font-mono shadow-[0_0_10px_rgba(240,185,11,0.1)]'
                : 'text-xs px-2 py-1 rounded bg-red-500/30 text-red-400 border border-red-700 font-mono shadow-[0_0_10px_rgba(240,185,11,0.1)]'
            }
          >
            Total PnL:{' '}
            <span
              className={
                totalPnL >= 0
                  ? 'text-green-400 font-extrabold'
                  : 'text-red-500 font-extrabold'
              }
            >
              {totalPnL.toFixed(2)}
            </span>
          </div>
        </div>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead className="text-left border-b border-white/5">
            <tr>
              <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-left">
                Symbol
              </th>
              <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-center">
                Side
              </th>
              <th
                className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell"
                title={'Entry'}
              >
                {'Entry'}
              </th>
              <th
                className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell"
                title={'Mark'}
              >
                {'Mark'}
              </th>
              <th
                className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right"
                title={'Qty'}
              >
                {'Qty'}
              </th>
              <th
                className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell"
                title={'Value'}
              >
                {'Value'}
              </th>
              <th
                className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-center hidden md:table-cell"
                title={'Lev.'}
              >
                {'Lev.'}
              </th>
              <th
                className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right"
                title={'uPnL'}
              >
                {'uPnL'}
              </th>
              <th
                className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell"
                title={'Liq.'}
              >
                {'Liq.'}
              </th>
              <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-center">
                {'Action'}
              </th>
            </tr>
          </thead>
          <tbody>
            {paginatedPositions.map((pos, i) => (
              <tr
                key={pos.symbol + i}
                className="border-b border-white/30 last:border-0 transition-all hover:bg-white/5 cursor-pointer group/row"
                onClick={() => onSymbolClick(pos.symbol)}
              >
                <td className="px-1 py-3 font-mono font-semibold whitespace-nowrap text-left text-white group-hover/row:text-white transition-colors">
                  {pos.symbol}
                </td>
                <td className="px-1 py-3 whitespace-nowrap text-center">
                  <span
                    className={`px-1.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${pos.side === 'long' ? 'bg-green-500/80 text-nofx-green shadow-[0_0_8px_rgba(14,203,129,0.2)]' : 'bg-red-500/80 text-nofx-red shadow-[0_0_8px_rgba(246,70,93,0.2)]'}`}
                  >
                    {pos.side === 'long' ? 'long' : 'short'}
                  </span>
                </td>
                <td className="px-1 py-3 font-mono whitespace-nowrap text-right text-nofx-text-main hidden md:table-cell">
                  {pos.entry_price.toFixed(4)}
                </td>
                <td className="px-1 py-3 font-mono whitespace-nowrap text-right text-nofx-text-main hidden md:table-cell">
                  {pos.mark_price.toFixed(4)}
                </td>
                <td className="px-1 py-3 font-mono whitespace-nowrap text-right text-nofx-text-main">
                  {pos.quantity.toFixed(4)}
                </td>
                <td className="px-1 py-3 font-mono font-bold whitespace-nowrap text-right text-nofx-text-main hidden md:table-cell">
                  {(pos.quantity * pos.mark_price).toFixed(2)}
                </td>
                <td className="px-1 py-3 font-mono whitespace-nowrap text-center text-nofx-gold hidden md:table-cell">
                  {pos.leverage}x
                </td>
                <td className="px-1 py-3 font-mono whitespace-nowrap text-right">
                  <span
                    className={`font-bold ${pos.unrealized_pnl >= 0 ? 'text-green-500/80 shadow-green-500/80' : 'text-red-500/80 shadow-red-500/80'}`}
                  >
                    {pos.unrealized_pnl >= 0 ? '+' : ''}
                    {pos.unrealized_pnl.toFixed(2)}
                  </span>
                </td>
                <td className="px-1 py-3 font-mono whitespace-nowrap text-right text-nofx-text-muted hidden md:table-cell">
                  {pos.liquidation_price.toFixed(4)}
                </td>
                <td className="px-1 py-3 whitespace-nowrap text-center">
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation()
                      handleClosePosition(pos.symbol, pos.side.toUpperCase())
                    }}
                    disabled={closingPosition === pos.symbol}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-semibold transition-colors hover:bg-red-500/20 disabled:opacity-50 disabled:cursor-not-allowed mx-auto bg-red-500/10 text-red-500 border border-red-500/30"
                    title={'Close Position'}
                  >
                    {closingPosition === pos.symbol ? (
                      <Loader2 className="w-3 h-3 animate-spin" />
                    ) : (
                      <LogOut className="w-3 h-3" />
                    )}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination footer */}
      {totalPositions > 10 && (
        <div className="flex flex-wrap items-center justify-between gap-3 pt-4 mt-4 text-xs border-t border-white/5 text-nofx-text-muted">
          <span>
            Showing {paginatedPositions.length} of {totalPositions} positions
          </span>
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              <span>Per page:</span>
              <CustomSelect
                value={pageSize}
                onChange={(val) => setPageSize(Number(val))}
                options={[
                  { value: 20, label: '20' },
                  { value: 50, label: '50' },
                  { value: 100, label: '100' },
                ]}
                className="w-16"
              />
            </div>
            {totalPages > 1 && (
              <div className="flex items-center gap-1">
                {['«', '‹', `${currentPage} / ${totalPages}`, '›', '»'].map(
                  (label, idx) => {
                    const isText = idx === 2
                    const isFirst = idx === 0
                    const isPrev = idx === 1
                    const isNext = idx === 3
                    const isLast = idx === 4
                    if (isText)
                      return (
                        <span key={idx} className="px-3 text-nofx-text-main">
                          {label}
                        </span>
                      )

                    let onClick = () => {}
                    let disabled = false

                    if (isFirst) {
                      onClick = () => setCurrentPage(1)
                      disabled = currentPage === 1
                    }
                    if (isPrev) {
                      onClick = () => setCurrentPage((p) => Math.max(1, p - 1))
                      disabled = currentPage === 1
                    }
                    if (isNext) {
                      onClick = () =>
                        setCurrentPage((p) => Math.min(totalPages, p + 1))
                      disabled = currentPage === totalPages
                    }
                    if (isLast) {
                      onClick = () => setCurrentPage(totalPages)
                      disabled = currentPage === totalPages
                    }

                    return (
                      <button
                        key={idx}
                        onClick={onClick}
                        disabled={disabled}
                        className={`px-2 py-1 rounded transition-colors ${disabled ? 'opacity-30 cursor-not-allowed' : 'hover:bg-white/10 text-nofx-text-main bg-white/5'}`}
                      >
                        {label}
                      </button>
                    )
                  }
                )}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
})
