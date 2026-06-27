import { useQueryClient } from '@tanstack/react-query'
import { Check, Copy, Eye, EyeOff, RefreshCw } from 'lucide-react'
import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ChartTabs } from '../components/ChartTabs'
import { DecisionCard } from '../components/DecisionCard'
import { DeepVoidBackground } from '../components/DeepVoidBackground'
import { PositionHistory } from '../components/PositionHistory'
import { PositionsTable } from '../components/PositionsTable'
import { PunkAvatar, getTraderAvatar } from '../components/PunkAvatar'
import { TraderSelector } from '../components/TraderSelector'
import { CustomSelect } from '../components/ui/CustomSelect'
import { type Language } from '../i18n/translations'
import { api } from '../lib/api'
import { confirmToast, notify } from '../lib/notify'
import type {
  AccountInfo,
  DecisionRecord,
  Exchange,
  Position,
  Statistics,
  SystemStatus,
  TraderInfo,
} from '../types'

// --- Helper Functions ---

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

// Helper function to get exchange display name from exchange ID (UUID)
function getExchangeDisplayNameFromList(
  exchangeId: string | undefined,
  exchanges: Exchange[] | undefined
): string {
  if (!exchangeId || !Array.isArray(exchanges)) return 'Unknown'
  const exchange = exchanges.find((e) => e.id === exchangeId)
  if (!exchange) return exchangeId.substring(0, 8).toUpperCase() + '...'
  const typeName = exchange.exchange_type?.toUpperCase() || exchange.name
  return exchange.account_name
    ? `${typeName} - ${exchange.account_name}`
    : typeName
}

// Helper function to get exchange type from exchange ID (UUID) - for kline charts
function getExchangeTypeFromList(
  exchangeId: string | undefined,
  exchanges: Exchange[] | undefined
): string {
  if (!exchangeId || !Array.isArray(exchanges)) return 'binance'
  const exchange = exchanges.find((e) => e.id === exchangeId)
  if (!exchange) return 'binance' // Default to binance for charts
  return exchange.exchange_type?.toLowerCase() || 'binance'
}

// Helper function to check if exchange is a perp-dex type (wallet-based)
function isPerpDexExchange(exchangeType: string | undefined): boolean {
  if (!exchangeType) return false
  const perpDexTypes = ['hyperliquid', 'lighter', 'aster']
  return perpDexTypes.includes(exchangeType.toLowerCase())
}

// Helper function to get wallet address for perp-dex exchanges
function getWalletAddress(exchange: Exchange | undefined): string | undefined {
  if (!exchange) return undefined
  const type = exchange.exchange_type?.toLowerCase()
  switch (type) {
    case 'hyperliquid':
      return exchange.hyperliquidWalletAddr
    case 'lighter':
      return exchange.lighterWalletAddr
    case 'aster':
      return exchange.asterSigner
    default:
      return undefined
  }
}

// Helper function to truncate wallet address for display
function truncateAddress(address: string, startLen = 6, endLen = 4): string {
  if (address.length <= startLen + endLen + 3) return address
  return `${address.slice(0, startLen)}...${address.slice(-endLen)}`
}

// --- Components ---

interface TraderDashboardPageProps {
  selectedTrader?: TraderInfo
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
  onNavigateToTraders: () => void
  status?: SystemStatus
  account?: AccountInfo
  positions?: Position[]
  decisions?: DecisionRecord[]
  refetchDecisions?: () => void
  decisionsLimit: number
  onDecisionsLimitChange: (limit: number) => void
  stats?: Statistics
  lastUpdate: string
  language: Language
  exchanges?: Exchange[]
}

export function TraderDashboardPage({
  selectedTrader,
  status,
  account,
  positions,
  decisions,
  refetchDecisions,
  decisionsLimit,
  onDecisionsLimitChange,
  language,
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
  onNavigateToTraders,
  exchanges,
}: TraderDashboardPageProps) {
  const queryClient = useQueryClient()
  // Track previous trader ID to detect actual trader switches
  const prevTraderIdRef = useRef<string | undefined>(undefined)

  // Initialize symbol from localStorage or default to BTC
  const [selectedChartSymbol, setSelectedChartSymbol] = useState<string>(() => {
    if (selectedTrader?.trader_id) {
      return (
        localStorage.getItem(`nofx-chart-symbol-${selectedTrader.trader_id}`) ||
        'BTC'
      )
    }
    return 'BTC'
  })

  // Sync symbol changes to localStorage
  useEffect(() => {
    if (selectedTrader?.trader_id && selectedChartSymbol) {
      localStorage.setItem(
        `nofx-chart-symbol-${selectedTrader.trader_id}`,
        selectedChartSymbol
      )
    }
  }, [selectedChartSymbol, selectedTrader?.trader_id])

  // Load saved symbol ONLY when switching to a different trader
  useEffect(() => {
    const currentTraderId = selectedTrader?.trader_id
    // Only reset symbol when actually switching to a DIFFERENT trader
    if (currentTraderId && currentTraderId !== prevTraderIdRef.current) {
      const saved = localStorage.getItem(`nofx-chart-symbol-${currentTraderId}`)
      setSelectedChartSymbol(saved || 'BTC')
      prevTraderIdRef.current = currentTraderId
    }
  }, [selectedTrader?.trader_id])

  const chartSectionRef = useRef<HTMLDivElement>(null)

  const [showWalletAddress, setShowWalletAddress] = useState<boolean>(false)
  const [copiedAddress, setCopiedAddress] = useState<boolean>(false)

  // Get current exchange info for perp-dex wallet display
  const currentExchange = useMemo(
    () =>
      Array.isArray(exchanges)
        ? exchanges.find((e) => e.id === selectedTrader?.exchange_id)
        : undefined,
    [exchanges, selectedTrader?.exchange_id]
  )
  const walletAddress = getWalletAddress(currentExchange)
  const isPerpDex = isPerpDexExchange(currentExchange?.exchange_type)

  // Copy wallet address to clipboard
  const handleCopyAddress = useCallback(async () => {
    if (!walletAddress) return
    try {
      await navigator.clipboard.writeText(walletAddress)
      setCopiedAddress(true)
      setTimeout(() => setCopiedAddress(false), 2000)
    } catch (err) {
      console.error('Failed to copy address:', err)
    }
  }, [walletAddress])

  // Handle symbol click from Decision Card
  const handleSymbolClick = useCallback((symbol: string) => {
    // Set the selected symbol
    setSelectedChartSymbol(symbol)
    // Scroll to chart section
    setTimeout(() => {
      chartSectionRef.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      })
    }, 100)
  }, [])

  // Close position handler - memoized to prevent unnecessary re-renders
  const handleClosePosition = useCallback(
    async (symbol: string, side: string) => {
      if (!selectedTraderId) return

      const confirmMsg = `Are you sure you want to close ${symbol} ${side === 'LONG' ? 'LONG' : 'SHORT'} position?`

      const confirmed = await confirmToast(confirmMsg, {
        title: 'Confirm Close',
        okText: 'Confirm',
        cancelText: 'Cancel',
      })

      if (!confirmed) return

      try {
        await api.closePosition(selectedTraderId, symbol, side)
        notify.success('Position closed successfully')
        // Use React Query to refresh data instead of reloading the page
        queryClient.invalidateQueries({
          queryKey: ['positions', selectedTraderId],
        })
        queryClient.invalidateQueries({
          queryKey: ['account', selectedTraderId],
        })
        queryClient.invalidateQueries({
          queryKey: ['position-history', selectedTraderId],
        })
      } catch (err: unknown) {
        const errorMsg =
          err instanceof Error ? err.message : 'Failed to close position'
        notify.error(errorMsg)
      }
    },
    [selectedTraderId, language, queryClient]
  )

  // If API failed with error, show empty state (likely backend not running)
  if (tradersError) {
    return (
      <div className="flex items-center justify-center min-h-[60vh] relative z-10">
        <div className="text-center max-w-md mx-auto px-6">
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center nofx-glass"
            style={{
              background: 'rgba(240, 185, 11, 0.1)',
              borderColor: 'rgba(240, 185, 11, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12 text-nofx-gold"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>
          <h2 className="text-2xl font-bold mb-3 text-nofx-text-main">
            Connection Failed
          </h2>
          <p className="text-base mb-6 text-nofx-text-muted">
            Please check if the backend service is running.
          </p>
          <button
            onClick={() => window.location.reload()}
            className="px-6 py-3 rounded-lg font-semibold transition-colors nofx-glass border border-nofx-gold/30 text-nofx-gold hover:bg-nofx-gold/10"
          >
            Retry
          </button>
        </div>
      </div>
    )
  }

  // If traders is loaded and empty, show empty state
  if (traders && traders.length === 0) {
    return (
      <div className="flex items-center justify-center min-h-[60vh] relative z-10">
        <div className="text-center max-w-md mx-auto px-6">
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center nofx-glass"
            style={{
              background: 'rgba(240, 185, 11, 0.1)',
              borderColor: 'rgba(240, 185, 11, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12 text-nofx-gold"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>
          <h2 className="text-2xl font-bold mb-3 text-nofx-text-main">
            Let's Get Started!
          </h2>
          <p className="text-base mb-6 text-nofx-text-muted">
            Create your first AI trader to automate your trading strategy.
            Connect an exchange, choose an AI model, and start trading in
            minutes!
          </p>
          <button
            onClick={onNavigateToTraders}
            className="px-6 py-3 rounded-lg font-semibold transition-colors nofx-glass border border-nofx-gold/30 text-nofx-gold hover:bg-nofx-gold/10"
          >
            Create Your First Trader
          </button>
        </div>
      </div>
    )
  }

  // If traders is still loading or selectedTrader is not ready, show skeleton
  if (!selectedTrader) {
    return (
      <div className="space-y-6 relative z-10">
        <div className="nofx-glass">
          <div className="h-8 w-48 mb-3 bg-nofx-bg/50 rounded"></div>
          <div className="flex gap-4">
            <div className="h-4 w-32 bg-nofx-bg/50 rounded"></div>
            <div className="h-4 w-24 bg-nofx-bg/50 rounded"></div>
            <div className="h-4 w-28 bg-nofx-bg/50 rounded"></div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="nofx-glass p-5">
              <div className="h-4 w-24 mb-3 bg-nofx-bg/50 rounded"></div>
              <div className="h-8 w-32 bg-nofx-bg/50 rounded"></div>
            </div>
          ))}
        </div>
        <div className="nofx-glass">
          <div className="h-6 w-40 mb-4 bg-nofx-bg/50 rounded"></div>
          <div className="h-64 w-full bg-nofx-bg/50 rounded"></div>
        </div>
      </div>
    )
  }

  return (
    <DeepVoidBackground className="min-h-screen pb-12">
      <div className="w-full px-4 md:px-8 relative z-10 pt-6">
        {/* Trader Header */}
        <div
          className="mb-3 md:mb-6 rounded-lg p-3 md:p-6 nofx-glass group border border-white/20 mx-0 my-2 md:mx-16 md:my-6"
          style={{
            background:
              'linear-gradient(135deg, rgba(15, 23, 42, 0.6) 0%, rgba(15, 23, 42, 0.4) 100%)',
          }}
        >
          <div className="flex items-start justify-between mb-4">
            <h2 className="text-2xl font-bold flex items-center gap-4 text-nofx-text-main">
              <div className="relative">
                <PunkAvatar
                  seed={getTraderAvatar(
                    selectedTrader.trader_id,
                    selectedTrader.trader_name
                  )}
                  size={56}
                  className="rounded-xl border-2 border-yellow-500/30 shadow-[0_0_15px_rgba(240,185,11,0.2)]"
                />
                <div className="absolute -bottom-1 -right-1 w-4 h-4 bg-green-500 rounded-full border-2 border-[#0B0E11] shadow-[0_0_8px_rgba(14,203,129,0.8)]" />
              </div>
              <div className="flex flex-col gap-2">
                <span className="text-lg sm:text-3xl tracking-tight text-yellow-500 font-semibold">
                  {selectedTrader.trader_name}
                </span>
                <span className="text-xs font-mono text-nofx-text-muted opacity-60 flex items-center gap-2">
                  ID: {selectedTrader.trader_id.slice(0, 8)}...
                </span>
              </div>
            </h2>

            <div className="flex items-center gap-4 sm:flex-row flex-col">
              {/* Trader Selector */}
              {traders && traders.length > 0 && (
                <TraderSelector
                  traders={traders}
                  selectedTraderId={selectedTraderId}
                  onSelect={onTraderSelect}
                />
              )}

              {/* Wallet Address Display for Perp-DEX */}
              {exchanges && isPerpDex && (
                <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg nofx-glass border border-yellow-500/20">
                  {walletAddress ? (
                    <>
                      <span className="text-xs font-mono text-yellow-500">
                        {showWalletAddress
                          ? walletAddress
                          : truncateAddress(walletAddress)}
                      </span>
                      <button
                        type="button"
                        onClick={() => setShowWalletAddress(!showWalletAddress)}
                        className="p-1 rounded hover:bg-white/10 transition-colors"
                        title={
                          showWalletAddress
                            ? 'Hide address'
                            : 'Show full address'
                        }
                      >
                        {showWalletAddress ? (
                          <EyeOff className="w-3.5 h-3.5 text-nofx-text-muted" />
                        ) : (
                          <Eye className="w-3.5 h-3.5 text-nofx-text-muted" />
                        )}
                      </button>
                      <button
                        type="button"
                        onClick={handleCopyAddress}
                        className="p-1 rounded hover:bg-white/10 transition-colors"
                        title={'Copy address'}
                      >
                        {copiedAddress ? (
                          <Check className="w-3.5 h-3.5 text-nofx-green" />
                        ) : (
                          <Copy className="w-3.5 h-3.5 text-nofx-text-muted" />
                        )}
                      </button>
                    </>
                  ) : (
                    <span className="text-xs text-nofx-text-muted">
                      No address configured
                    </span>
                  )}
                </div>
              )}
            </div>
          </div>
          <div className="flex items-center gap-6 text-sm flex-wrap text-nofx-text-muted font-mono pl-2">
            <span className="flex items-center gap-2">
              <span className="opacity-60">AI Model:</span>
              <span
                className="font-bold px-2 py-0.5 rounded text-xs tracking-wide"
                style={{
                  background: selectedTrader.ai_model.includes('qwen')
                    ? 'rgba(192, 132, 252, 0.15)'
                    : 'rgba(96, 165, 250, 0.15)',
                  color: selectedTrader.ai_model.includes('qwen')
                    ? '#c084fc'
                    : '#60a5fa',
                  border: `1px solid ${selectedTrader.ai_model.includes('qwen') ? '#c084fc' : '#60a5fa'}40`,
                }}
              >
                {getModelDisplayName(
                  selectedTrader.ai_model.split('_').pop() ||
                    selectedTrader.ai_model
                )}
              </span>
            </span>
            <span className="w-px h-3 bg-white/10 hidden md:block" />
            <span className="flex items-center gap-2">
              <span className="opacity-60">Exchange:</span>
              <span className="text-nofx-text-main font-semibold">
                {getExchangeDisplayNameFromList(
                  selectedTrader.exchange_id,
                  exchanges
                )}
              </span>
            </span>
            <span className="w-px h-3 bg-white/10 hidden md:block" />
            <span className="flex items-center gap-2">
              <span className="opacity-60">Strategy:</span>
              <span className="text-nofx-gold font-semibold tracking-wide">
                {selectedTrader.strategy_name || 'No Strategy'}
              </span>
            </span>
            {status && (
              <div className="hidden md:contents">
                <span className="w-px h-3 bg-white/10" />
                <span>
                  Cycles:{' '}
                  <span className="text-nofx-text-main">
                    {status.call_count}
                  </span>
                </span>
                <span className="w-px h-3 bg-white/10" />
                <span>
                  Runtime:{' '}
                  <span className="text-nofx-text-main">
                    {status.runtime_minutes} min
                  </span>
                </span>
              </div>
            )}
          </div>
        </div>

        {/* Account Overview */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 px-0 py-2 md:px-16 md:py-2">
          <StatCard
            title="Total Equity"
            value={`${account?.total_equity?.toFixed(2) || '0.00'}`}
            unit="USDT"
            change={account?.total_pnl_pct || 0}
            positive={(account?.total_pnl ?? 0) > 0}
            icon=""
          />
          <StatCard
            title="Available Balance"
            value={`${account?.available_balance?.toFixed(2) || '0.00'}`}
            unit="USDT"
            subtitle={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% free`}
            icon=""
          />
          <StatCard
            title="Total PnL"
            value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'}`}
            unit="USDT"
            change={account?.total_pnl_pct || 0}
            positive={(account?.total_pnl ?? 0) >= 0}
            icon=""
          />
          <StatCard
            title="Positions"
            value={`${account?.position_count || 0}`}
            unit="ACTIVE"
            subtitle={`Margin Used: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`}
            icon=""
          />
        </div>

        {/* Main Content Area */}
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-6 px-0 py-2 md:px-16 md:py-6">
          {/* Left Column: Charts + Positions */}
          <div className="lg:col-span-3 space-y-6">
            {/* Chart Tabs (Equity / K-line) */}
            <div ref={chartSectionRef}>
              <ChartTabs
                key={`chart-${selectedTrader.trader_id}`}
                traderId={selectedTrader.trader_id}
                symbol={selectedChartSymbol}
                onSymbolChange={setSelectedChartSymbol}
                exchangeId={getExchangeTypeFromList(
                  selectedTrader.exchange_id,
                  exchanges
                )}
                isTraderRunning={selectedTrader.is_running}
              />
            </div>

            {/* Current Positions - Memoized to prevent chart re-renders */}
            <div className="nofx-glass relative overflow-hidden group">
              <PositionsTable
                positions={positions || []}
                onSymbolClick={(symbol) => {
                  setSelectedChartSymbol(symbol)

                  if (chartSectionRef.current) {
                    chartSectionRef.current.scrollIntoView({
                      behavior: 'smooth',
                      block: 'start',
                    })
                  }
                }}
                onClosePosition={handleClosePosition}
              />
            </div>
          </div>

          {/* Right Column: Recent Decisions */}
          <div className="lg:col-span-2 nofx-glass h-fit lg:sticky lg:top-24 lg:max-h-[calc(100vh-120px)] flex flex-col">
            {/* Header */}
            <div className="flex items-center gap-3 mb-5 shrink-0">
              <div className="flex-1">
                <h2 className="text-xl font-bold text-nofx-text-main">
                  Recent Decisions
                </h2>
                {decisions && decisions.length > 0 && (
                  <div className="text-xs text-nofx-text-muted">
                    Last {decisions.length} trading cycles
                  </div>
                )}
              </div>
              {/* Limit Selector */}
              <div className="flex items-center gap-2">
                <button
                  className={
                    'w-8 h-8 flex items-center justify-center rounded-lg transition-all duration-200 bg-white/5 border border-white/10 hover:bg-white/10 hover:border-nofx-gold/30 text-sm font-medium text-nofx-text-main focus:outline-none'
                  }
                  onClick={() => refetchDecisions?.()}
                >
                  <RefreshCw className="w-4 h-4" />
                </button>
                <CustomSelect
                  value={decisionsLimit}
                  onChange={(val) => onDecisionsLimitChange(Number(val))}
                  options={[
                    { value: 5, label: '5' },
                    { value: 10, label: '10' },
                    { value: 20, label: '20' },
                    { value: 50, label: '50' },
                    { value: 100, label: '100' },
                  ]}
                  className="w-20"
                />
              </div>
            </div>

            {/* Decisions List - Scrollable */}
            <div
              className="space-y-4 overflow-y-auto p-2 rounded-xl custom-scrollbar border border-white/20 bg-white/5 custom-scrollbar"
              style={{
                maxHeight: 'calc(100vh - 200px)',
                boxShadow: 'inset -1px 0px 20px 10px #41300059',
              }}
            >
              {Array.isArray(decisions) && decisions.length > 0 ? (
                decisions.map((decision, i) => (
                  <DecisionCard
                    key={i}
                    decision={decision}
                    language={language}
                    onSymbolClick={handleSymbolClick}
                  />
                ))
              ) : (
                <div className="py-16 text-center text-nofx-text-muted opacity-60">
                  <div className="text-lg font-semibold mb-2 text-nofx-text-main">
                    No Decisions Yet
                  </div>
                  <div className="text-sm">
                    AI trading decisions will appear here
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Position History Section */}
        {selectedTraderId && (
          <div className="nofx-glass px-0 py-2 md:px-16 md:py-6">
            <div className="flex items-center justify-between mb-5">
              <h2 className="text-xl font-bold flex items-center gap-2 text-nofx-text-main">
                Position History
              </h2>
            </div>
            <PositionHistory 
              traderId={selectedTraderId} 
              isTraderRunning={selectedTrader.is_running}
            />
          </div>
        )}
      </div>
    </DeepVoidBackground>
  )
}

// Stat Card Component - Deep Void Style - Memoized to prevent unnecessary re-renders
const StatCard = memo(function StatCard({
  title,
  value,
  unit,
  change,
  positive,
  subtitle,
  icon,
}: {
  title: string
  value: string
  unit?: string
  change?: number
  positive?: boolean
  subtitle?: string
  icon?: string
}) {
  return (
    <div className="group nofx-glass p-5 rounded-lg transition-colors duration-300 hover:bg-white/5 border border-white/20 hover:border-nofx-gold/20 relative overflow-hidden">
      <div className="absolute top-0 right-0 p-4 opacity-5 group-hover:opacity-10 transition-opacity text-4xl grayscale group-hover:grayscale-0">
        {icon}
      </div>
      <div className="text-xs mb-2 font-mono uppercase tracking-wider text-nofx-text-muted flex items-center gap-2">
        {title}
      </div>
      <div className="flex items-baseline gap-1 mb-1">
        <div className="text-2xl font-bold font-mono text-nofx-text-main tracking-tight group-hover:text-white transition-colors">
          {value}
        </div>
        {unit && (
          <span className="text-xs font-mono text-nofx-text-muted opacity-60">
            {unit}
          </span>
        )}
      </div>

      {change !== undefined && (
        <div className="flex items-center gap-1">
          <div
            className={`text-sm mono font-bold flex items-center gap-1 ${positive ? 'text-green' : 'text-red'}`}
          >
            <span>{positive ? '▲' : '▼'}</span>
            <span className={positive ? 'text-green' : 'text-red'}>
              {positive ? '+' : ''}
              {change.toFixed(2)}%
            </span>
          </div>
        </div>
      )}
      {subtitle && (
        <div className="text-xs mt-2 mono text-nofx-text-muted opacity-80">
          {subtitle}
        </div>
      )}
    </div>
  )
})
