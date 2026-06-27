import { Ban, Database, List, Plus, TrendingUp, X, Zap } from 'lucide-react'
import { useState } from 'react'
import type { CoinSourceConfig } from '../../types'
import { CustomSelect } from '../ui/CustomSelect'

interface CoinSourceEditorProps {
  config: CoinSourceConfig
  onChange: (config: CoinSourceConfig) => void
  disabled?: boolean
}

export function CoinSourceEditor({
  config,
  onChange,
  disabled,
}: CoinSourceEditorProps) {
  const [newCoin, setNewCoin] = useState('')
  const [newExcludedCoin, setNewExcludedCoin] = useState('')

  // English labels
  const labels = {
    sourceType: 'Source Type',
    static: 'Static List',
    ai500: 'AI500 Data Provider',
    oi_top: 'OI Top',
    mixed: 'Mixed Mode',
    staticCoins: 'Custom Coins',
    addCoin: 'Add Coin',
    useAI500: 'Enable AI500 Data Provider',
    ai500Limit: 'Limit',
    useOITop: 'Enable OI Top',
    oiTopLimit: 'Limit',
    staticDesc: 'Manually specify trading coins',
    ai500Desc: 'Use AI500 smart-filtered popular coins',
    oiTopDesc: 'Use coins with fastest OI growth',
    mixedDesc: 'Combine multiple sources: AI500 + OI Top + Custom',
    dataSourceConfig: 'Data Source Configuration',
    excludedCoins: 'Excluded Coins',
    excludedCoinsDesc:
      'These coins will be excluded from all sources and will not be traded',
    addExcludedCoin: 'Add Excluded',
    none: 'None',
  }

  const sourceTypes = [
    { value: 'static', icon: List, color: '#848E9C' },
    { value: 'ai500', icon: Database, color: '#F0B90B' },
    { value: 'oi_top', icon: TrendingUp, color: '#0ECB81' },
    { value: 'mixed', icon: Database, color: '#60a5fa' },
  ] as const

  // xyz dex assets (stocks, forex, commodities) - should NOT get USDT suffix
  const xyzDexAssets = new Set([
    // Stocks
    'TSLA',
    'NVDA',
    'AAPL',
    'MSFT',
    'META',
    'AMZN',
    'GOOGL',
    'AMD',
    'COIN',
    'NFLX',
    'PLTR',
    'HOOD',
    'INTC',
    'MSTR',
    'TSM',
    'ORCL',
    'MU',
    'RIVN',
    'COST',
    'LLY',
    'CRCL',
    'SKHX',
    'SNDK',
    // Forex
    'EUR',
    'JPY',
    // Commodities
    'GOLD',
    'SILVER',
    // Index
    'XYZ100',
  ])

  const isXyzDexAsset = (symbol: string): boolean => {
    const base = symbol
      .toUpperCase()
      .replace(/^XYZ:/, '')
      .replace(/USDT$|USD$|-USDC$/, '')
    return xyzDexAssets.has(base)
  }

  const handleAddCoin = () => {
    if (!newCoin.trim()) return
    const symbol = newCoin.toUpperCase().trim()

    // For xyz dex assets (stocks, forex, commodities), use xyz: prefix without USDT
    let formattedSymbol: string
    if (isXyzDexAsset(symbol)) {
      // Remove xyz: prefix (case-insensitive) and any USD suffixes
      const base = symbol
        .replace(/^xyz:/i, '')
        .replace(/USDT$|USD$|-USDC$/i, '')
      formattedSymbol = `xyz:${base}`
    } else {
      formattedSymbol = symbol.endsWith('USDT') ? symbol : `${symbol}USDT`
    }

    const currentCoins = config.static_coins || []
    if (!currentCoins.includes(formattedSymbol)) {
      onChange({
        ...config,
        static_coins: [...currentCoins, formattedSymbol],
      })
    }
    setNewCoin('')
  }

  const handleRemoveCoin = (coin: string) => {
    onChange({
      ...config,
      static_coins: (config.static_coins || []).filter((c) => c !== coin),
    })
  }

  const handleAddExcludedCoin = () => {
    if (!newExcludedCoin.trim()) return
    const symbol = newExcludedCoin.toUpperCase().trim()

    // For xyz dex assets, use xyz: prefix without USDT
    let formattedSymbol: string
    if (isXyzDexAsset(symbol)) {
      // Remove xyz: prefix (case-insensitive) and any USD suffixes
      const base = symbol
        .replace(/^xyz:/i, '')
        .replace(/USDT$|USD$|-USDC$/i, '')
      formattedSymbol = `xyz:${base}`
    } else {
      formattedSymbol = symbol.endsWith('USDT') ? symbol : `${symbol}USDT`
    }

    const currentExcluded = config.excluded_coins || []
    if (!currentExcluded.includes(formattedSymbol)) {
      onChange({
        ...config,
        excluded_coins: [...currentExcluded, formattedSymbol],
      })
    }
    setNewExcludedCoin('')
  }

  const handleRemoveExcludedCoin = (coin: string) => {
    onChange({
      ...config,
      excluded_coins: (config.excluded_coins || []).filter((c) => c !== coin),
    })
  }

  return (
    <div className="space-y-6">
      {/* Source Type Selector */}
      <div className="flex gap-2 mt-2 overflow-x-auto custom-scrollbar pb-2 pt-1 -mx-1 px-1 sm:grid sm:grid-cols-4 sm:gap-3 sm:overflow-visible sm:pb-0">
        {sourceTypes.map(({ value, icon: Icon, color }) => (
          <button
            key={value}
            onClick={() =>
              !disabled &&
              onChange({
                ...config,
                source_type: value as CoinSourceConfig['source_type'],
              })
            }
            disabled={disabled}
            className={`flex-shrink-0 w-[100px] sm:w-auto p-3 sm:p-4 rounded-lg border transition-all ${
              config.source_type === value
                ? 'ring-2 ring-nofx-gold bg-nofx-gold/10'
                : 'hover:bg-white/5 bg-nofx-bg'
            } border-nofx-gold/20`}
          >
            <Icon
              className="w-5 h-5 sm:w-6 sm:h-6 mx-auto mb-1 sm:mb-2"
              style={{ color }}
            />
            <div className="text-xs sm:text-sm font-medium text-nofx-text truncate">
              {labels[value as keyof typeof labels]}
            </div>
            <div className="text-[10px] sm:text-xs mt-0.5 sm:mt-1 text-nofx-text-muted line-clamp-2">
              {labels[`${value}Desc` as keyof typeof labels]}
            </div>
          </button>
        ))}
      </div>

      {/* Static Coins */}
      {(config.source_type === 'static' || config.source_type === 'mixed') && (
        <div>
          <label className="block text-sm font-medium mb-3 text-nofx-text">
            {labels.staticCoins}
          </label>
          <div className="flex flex-wrap gap-2 mb-3">
            {(config.static_coins || []).map((coin) => (
              <span
                key={coin}
                className="flex items-center gap-1 px-3 py-1.5 rounded-full text-sm bg-nofx-bg-lighter text-nofx-text"
              >
                {coin}
                {!disabled && (
                  <button
                    onClick={() => handleRemoveCoin(coin)}
                    className="ml-1 hover:text-red-400 transition-colors"
                  >
                    <X className="w-3 h-3" />
                  </button>
                )}
              </span>
            ))}
          </div>
          {!disabled && (
            <div className="flex flex-col sm:flex-row gap-2">
              <input
                type="text"
                value={newCoin}
                onChange={(e) => setNewCoin(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleAddCoin()}
                placeholder="BTC, ETH, SOL..."
                className="flex-1 px-3 sm:px-4 py-2 rounded-lg bg-nofx-bg border border-nofx-gold/20 text-nofx-text text-sm"
              />
              <button
                onClick={handleAddCoin}
                className="px-4 py-2 rounded-lg flex items-center justify-center gap-2 transition-colors bg-nofx-gold text-black hover:bg-yellow-500 text-sm"
              >
                <Plus className="w-4 h-4" />
                {labels.addCoin}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Excluded Coins */}
      <div>
        <div className="flex items-center gap-2 mb-3">
          <Ban className="w-4 h-4 text-nofx-danger" />
          <label className="text-sm font-medium text-nofx-text">
            {labels.excludedCoins}
          </label>
        </div>
        <p className="text-xs mb-3 text-nofx-text-muted hidden sm:block">
          {labels.excludedCoinsDesc}
        </p>
        <div className="flex flex-wrap gap-2 mb-3">
          {(config.excluded_coins || []).map((coin) => (
            <span
              key={coin}
              className="flex items-center gap-1 px-3 py-1.5 rounded-full text-sm bg-nofx-danger/15 text-nofx-danger"
            >
              {coin}
              {!disabled && (
                <button
                  onClick={() => handleRemoveExcludedCoin(coin)}
                  className="ml-1 hover:text-white transition-colors"
                >
                  <X className="w-3 h-3" />
                </button>
              )}
            </span>
          ))}
        </div>
        {!disabled && (
          <div className="flex flex-col sm:flex-row gap-2">
            <input
              type="text"
              value={newExcludedCoin}
              onChange={(e) => setNewExcludedCoin(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddExcludedCoin()}
              placeholder="BTC, ETH, DOGE..."
              className="flex-1 px-3 sm:px-4 py-2 rounded-lg text-sm bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
            />
            <button
              onClick={handleAddExcludedCoin}
              className="px-4 py-2 rounded-lg flex items-center justify-center gap-2 transition-colors text-sm bg-nofx-danger text-white hover:bg-red-600"
            >
              <Ban className="w-4 h-4" />
              {labels.addExcludedCoin}
            </button>
          </div>
        )}
      </div>

      {/* AI500 Options */}
      {(config.source_type === 'ai500' || config.source_type === 'mixed') && (
        <div className="p-4 rounded-lg bg-nofx-gold/5 border border-nofx-gold/20">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Zap className="w-4 h-4 text-nofx-gold" />
              <span className="text-sm font-medium text-nofx-text">
                AI500 {labels.dataSourceConfig}
              </span>
            </div>
          </div>

          <div className="space-y-3">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={config.use_ai500}
                onChange={(e) =>
                  !disabled &&
                  onChange({ ...config, use_ai500: e.target.checked })
                }
                disabled={disabled}
                className="w-5 h-5 rounded accent-nofx-gold"
              />
              <span className="text-nofx-text text-xs md:text-sm">
                {labels.useAI500}
              </span>
            </label>

            {config.use_ai500 && (
              <div className="flex items-center gap-3 px-8">
                <span className="text-sm text-nofx-text-muted">
                  {labels.ai500Limit}:
                </span>
                <CustomSelect
                  value={config.ai500_limit || 10}
                  onChange={(val) =>
                    !disabled &&
                    onChange({
                      ...config,
                      ai500_limit: Number(val) || 10,
                    })
                  }
                  options={[5, 10, 15, 20, 30, 50].map((n) => ({
                    value: n,
                    label: n.toString(),
                  }))}
                  disabled={disabled}
                  className="w-24"
                />
              </div>
            )}

            <p className="text-xs pl-8 text-nofx-text-muted hidden md:block">
              Uses NofxOS API Key (set in Indicators config)
            </p>
          </div>
        </div>
      )}

      {/* OI Top Options */}
      {(config.source_type === 'oi_top' || config.source_type === 'mixed') && (
        <div className="p-4 rounded-lg bg-nofx-success/5 border border-nofx-success/20">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <TrendingUp className="w-4 h-4 text-nofx-success" />
              <span className="text-sm font-medium text-nofx-text">
                OI Top {labels.dataSourceConfig}
              </span>
            </div>
          </div>

          <div className="space-y-3">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={config.use_oi_top}
                onChange={(e) =>
                  !disabled &&
                  onChange({ ...config, use_oi_top: e.target.checked })
                }
                disabled={disabled}
                className="w-5 h-5 rounded accent-nofx-success"
              />
              <span className="text-nofx-text md:text-sm text-xs">
                {labels.useOITop}
              </span>
            </label>

            {config.use_oi_top && (
              <div className="flex items-center gap-3 px-8">
                <span className="text-sm text-nofx-text-muted">
                  {labels.oiTopLimit}:
                </span>
                <CustomSelect
                  value={config.oi_top_limit || 20}
                  onChange={(val) =>
                    !disabled &&
                    onChange({
                      ...config,
                      oi_top_limit: Number(val) || 20,
                    })
                  }
                  options={[5, 10, 15, 20, 30, 50].map((n) => ({
                    value: n,
                    label: n.toString(),
                  }))}
                  disabled={disabled}
                  className="w-24"
                />
              </div>
            )}

            <p className="text-xs pl-8 text-nofx-text-muted hidden md:block">
              Uses HFXOS API Key (set in Indicators config)
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
