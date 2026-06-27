import { ChevronDown, ChevronRight, RotateCcw } from 'lucide-react'
import { useState } from 'react'
import type { PromptSectionsConfig } from '../../types'

interface PromptSectionsEditorProps {
  config: PromptSectionsConfig | undefined
  onChange: (config: PromptSectionsConfig) => void
  disabled?: boolean
}

// Default prompt sections (same as backend defaults)
const defaultSections: PromptSectionsConfig = {
  role_definition: `# You are a professional cryptocurrency trading AI
 
 You specialize in technical analysis and risk management, making rational trading decisions based on market data.
 Your goal is to capture high-probability trading opportunities while controlling risk.`,

  trading_frequency: `# ⏱️ Trading Frequency Awareness
 
 - Excellent traders: 2-4 trades per day ≈ 0.1-0.2 trades per hour
 - >2 trades per hour = Overtrading
 - Single position holding time ≥30-60 minutes
 If you find yourself trading every cycle → standards are too low; if closing <30 minutes → too impatient.`,

  entry_standards: `# 🎯 Entry Standards (Strict)
 
 Only open positions when multiple signals resonate:
 - Clear trend direction (EMA alignment, price position)
 - Momentum confirmation (MACD, RSI synergy)
 - Moderate volatility (ATR reasonable range)
 - Volume confirmation (Volume supports direction)
 
 Avoid: Single indicators, conflicting signals, sideways chop, re-entering immediately after closing.`,

  decision_process: `# 📋 Decision Process
 
 1. Check positions → Should take profit/stop loss?
 2. Scan candidate coins + multiple timeframes → Are there strong signals?
 3. Evaluate Risk/Reward Ratio → Does it meet minimum requirements?
 4. Write Chain of Thought first, then output structured JSON`,
}

export function PromptSectionsEditor({
  config,
  onChange,
  disabled,
}: PromptSectionsEditorProps) {
  const [expandedSections, setExpandedSections] = useState<
    Record<string, boolean>
  >({
    role_definition: false,
    trading_frequency: false,
    entry_standards: false,
    decision_process: false,
  })

  const sections = [
    {
      key: 'role_definition',
      label: 'Role Definition',
      desc: 'Define AI identity and core objectives',
    },
    {
      key: 'trading_frequency',
      label: 'Trading Frequency',
      desc: 'Set trading frequency expectations and overtrading warnings',
    },
    {
      key: 'entry_standards',
      label: 'Entry Standards',
      desc: 'Define entry signal conditions and avoidances',
    },
    {
      key: 'decision_process',
      label: 'Decision Process',
      desc: 'Set decision steps and thinking process',
    },
  ]

  const currentConfig = config || {}

  const updateSection = (key: keyof PromptSectionsConfig, value: string) => {
    if (!disabled) {
      onChange({ ...currentConfig, [key]: value })
    }
  }

  const resetSection = (key: keyof PromptSectionsConfig) => {
    if (!disabled) {
      onChange({ ...currentConfig, [key]: defaultSections[key] })
    }
  }

  const toggleSection = (key: string) => {
    setExpandedSections((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  const getValue = (key: keyof PromptSectionsConfig): string => {
    return currentConfig[key] || defaultSections[key] || ''
  }

  return (
    <div className="space-y-4">
      <p className="text-[10px] md:text-xs mt-1" style={{ color: '#848E9C' }}>
        Customize AI behavior and decision logic (output format and risk rules
        are fixed)
      </p>

      <div className="space-y-2">
        {sections.map(({ key, label, desc }) => {
          const sectionKey = key as keyof PromptSectionsConfig
          const isExpanded = expandedSections[key]
          const value = getValue(sectionKey)
          const isModified =
            currentConfig[sectionKey] !== undefined &&
            currentConfig[sectionKey] !== defaultSections[sectionKey]

          return (
            <div
              key={key}
              className="rounded-lg overflow-hidden"
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
            >
              <button
                onClick={() => toggleSection(key)}
                className="w-full flex items-center justify-between px-3 py-2.5 hover:bg-white/5 transition-colors text-left"
              >
                <div className="flex items-center gap-2">
                  {isExpanded ? (
                    <ChevronDown
                      className="w-4 h-4"
                      style={{ color: '#848E9C' }}
                    />
                  ) : (
                    <ChevronRight
                      className="w-4 h-4"
                      style={{ color: '#848E9C' }}
                    />
                  )}
                  <span
                    className="text-sm font-medium"
                    style={{ color: '#EAECEF' }}
                  >
                    {label}
                  </span>
                  {isModified && (
                    <span
                      className="px-1.5 py-0.5 text-[10px] rounded"
                      style={{
                        background: 'rgba(168, 85, 247, 0.15)',
                        color: '#a855f7',
                      }}
                    >
                      Modified
                    </span>
                  )}
                </div>
                <span className="text-[10px]" style={{ color: '#848E9C' }}>
                  {value.length} chars
                </span>
              </button>

              {isExpanded && (
                <div className="px-3 pb-3">
                  <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                    {desc}
                  </p>
                  <textarea
                    value={value}
                    onChange={(e) => updateSection(sectionKey, e.target.value)}
                    disabled={disabled}
                    rows={6}
                    className="w-full px-3 py-2 rounded-lg resize-y font-mono text-xs"
                    style={{
                      background: '#1E2329',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                      minHeight: '120px',
                    }}
                  />
                  <div className="flex justify-end mt-2">
                    <button
                      onClick={() => resetSection(sectionKey)}
                      disabled={disabled || !isModified}
                      className="flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors hover:bg-white/5 disabled:opacity-30"
                      style={{ color: '#848E9C' }}
                    >
                      <RotateCcw className="w-3 h-3" />
                      Reset to Default
                    </button>
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
