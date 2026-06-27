import type { RiskControlConfig } from '../../types'

interface RiskControlEditorProps {
  config: RiskControlConfig
  onChange: (config: RiskControlConfig) => void
  disabled?: boolean
}

export function RiskControlEditor({
  config,
  onChange,
  disabled,
}: RiskControlEditorProps) {
  // English labels
  const labels = {
    positionLimits: 'Position Limits',
    maxPositions: 'Max Positions',
    maxPositionsDesc: 'Maximum coins held simultaneously',
    tradingLeverage: 'Trading Leverage (Exchange)',
    btcEthLeverage: 'BTC/ETH Trading Leverage',
    btcEthLeverageDesc: 'Exchange leverage for opening positions',
    altcoinLeverage: 'Altcoin Trading Leverage',
    altcoinLeverageDesc: 'Exchange leverage for opening positions',
    positionValueRatio: 'Position Value Ratio (CODE ENFORCED)',
    positionValueRatioDesc:
      'Position notional value / equity, enforced by code',
    btcEthPositionValueRatio: 'BTC/ETH Position Value Ratio',
    btcEthPositionValueRatioDesc:
      'Max position value = equity × this ratio (CODE ENFORCED)',
    altcoinPositionValueRatio: 'Altcoin Position Value Ratio',
    altcoinPositionValueRatioDesc:
      'Max position value = equity × this ratio (CODE ENFORCED)',
    riskParameters: 'Risk Parameters',
    minRiskReward: 'Min Risk/Reward Ratio',
    minRiskRewardDesc: 'Minimum profit ratio for opening',
    maxMarginUsage: 'Max Margin Usage (CODE ENFORCED)',
    maxMarginUsageDesc: 'Maximum margin utilization, enforced by code',
    entryRequirements: 'Entry Requirements',
    minPositionSize: 'Min Position Size',
    minPositionSizeDesc: 'Minimum notional value in USDT',
    minConfidence: 'Min Confidence',
    minConfidenceDesc: 'AI confidence threshold for entry',
  }

  const updateField = <K extends keyof RiskControlConfig>(
    key: K,
    value: RiskControlConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
  }

  return (
    <div className="space-y-6">
      {/* Position Limits */}
      <div className="grid grid-cols-1 gap-4 mb-4">
        <div
          className="p-4 rounded-lg mt-2"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
            {labels.maxPositions}
          </label>
          <p className="text-xs md:text-sm mb-2" style={{ color: '#848E9C' }}>
            {labels.maxPositionsDesc}
          </p>
          <input
            type="number"
            value={config.max_positions ?? 3}
            onChange={(e) =>
              updateField('max_positions', parseInt(e.target.value) || 3)
            }
            disabled={disabled}
            min={1}
            max={10}
            className="w-32 px-3 py-2 rounded"
            style={{
              background: '#1E2329',
              border: '1px solid #2B3139',
              color: '#EAECEF',
            }}
          />
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {labels.btcEthLeverage}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {labels.btcEthLeverageDesc}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('btc_eth_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.btc_eth_max_leverage ?? 5}x
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {labels.altcoinLeverage}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {labels.altcoinLeverageDesc}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('altcoin_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.altcoin_max_leverage ?? 5}x
              </span>
            </div>
          </div>
        </div>

        {/* Position Value Ratio (Risk Control - CODE ENFORCED) */}
        <div className="flex flex-col items-start">
          <p className="text-xs font-medium" style={{ color: '#0ECB81' }}>
            {labels.positionValueRatio}
          </p>
          <p
            className="text-[10px] md:text-[12px] mt-1"
            style={{ color: '#848E9C' }}
          >
            {labels.positionValueRatioDesc}
          </p>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {labels.btcEthPositionValueRatio}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {labels.btcEthPositionValueRatioDesc}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_position_value_ratio ?? 5}
                onChange={(e) =>
                  updateField(
                    'btc_eth_max_position_value_ratio',
                    parseFloat(e.target.value)
                  )
                }
                disabled={disabled}
                min={0.5}
                max={10}
                step={0.5}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.btc_eth_max_position_value_ratio ?? 5}x
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {labels.altcoinPositionValueRatio}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {labels.altcoinPositionValueRatioDesc}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_position_value_ratio ?? 1}
                onChange={(e) =>
                  updateField(
                    'altcoin_max_position_value_ratio',
                    parseFloat(e.target.value)
                  )
                }
                disabled={disabled}
                min={0.5}
                max={10}
                step={0.5}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.altcoin_max_position_value_ratio ?? 1}x
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Risk Parameters */}
      <div>
        <p className="text-xs font-medium mb-4 text-red-500">Risk Parameters</p>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {labels.minRiskReward}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {labels.minRiskRewardDesc}
            </p>
            <div className="flex items-center">
              <span style={{ color: '#848E9C' }}>1:</span>
              <input
                type="number"
                value={config.min_risk_reward_ratio ?? 3}
                onChange={(e) =>
                  updateField(
                    'min_risk_reward_ratio',
                    parseFloat(e.target.value) || 3
                  )
                }
                disabled={disabled}
                min={1}
                max={10}
                step={0.5}
                className="w-20 px-3 py-2 rounded ml-2"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {labels.maxMarginUsage}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {labels.maxMarginUsageDesc}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={(config.max_margin_usage ?? 0.9) * 100}
                onChange={(e) =>
                  updateField(
                    'max_margin_usage',
                    parseInt(e.target.value) / 100
                  )
                }
                disabled={disabled}
                min={10}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {Math.round((config.max_margin_usage ?? 0.9) * 100)}%
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Entry Requirements */}
      <div>
        <p className="text-xs font-medium mb-4 text-orange-500">
          Entry Requirements
        </p>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {labels.minPositionSize}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {labels.minPositionSizeDesc}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.min_position_size ?? 12}
                onChange={(e) =>
                  updateField(
                    'min_position_size',
                    parseFloat(e.target.value) || 12
                  )
                }
                disabled={disabled}
                min={10}
                max={1000}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span className="ml-2" style={{ color: '#848E9C' }}>
                USDT
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {labels.minConfidence}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {labels.minConfidenceDesc}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.min_confidence ?? 75}
                onChange={(e) =>
                  updateField('min_confidence', parseInt(e.target.value))
                }
                disabled={disabled}
                min={50}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.min_confidence ?? 75}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
