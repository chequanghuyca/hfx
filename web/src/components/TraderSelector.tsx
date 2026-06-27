import { Check, ChevronDown } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { cn } from '../lib/cn'
import { TraderInfo } from '../types'

interface TraderSelectorProps {
  traders: TraderInfo[]
  selectedTraderId?: string
  onSelect: (traderId: string) => void
  disabled?: boolean
  className?: string
}

export function TraderSelector({
  traders,
  selectedTraderId,
  onSelect,
  disabled = false,
  className,
}: TraderSelectorProps) {
  const [isOpen, setIsOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const selectedTrader = traders.find((t) => t.trader_id === selectedTraderId)

  // Handle click outside to close dropdown
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [])

  const handleSelect = (traderId: string) => {
    onSelect(traderId)
    setIsOpen(false)
  }

  if (!traders || traders.length === 0) return null

  return (
    <div className={cn('relative', className)} ref={containerRef}>
      <button
        type="button"
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
        className={cn(
          'flex items-center gap-2 px-3 py-1.5 rounded-lg transition-all duration-200',
          'bg-white/5 border border-white/10 hover:bg-white/10 hover:border-nofx-gold/30',
          'text-sm font-medium text-nofx-text-main focus:outline-none',
          'min-w-[140px] justify-between',
          disabled && 'opacity-50 cursor-not-allowed',
          isOpen && 'bg-white/10 border-nofx-gold/30 ring-1 ring-nofx-gold/20'
        )}
      >
        <span className="truncate max-w-[160px]">
          {selectedTrader ? selectedTrader.trader_name : 'Select Trader'}
        </span>
        <ChevronDown
          className={cn(
            'w-4 h-4 text-nofx-text-muted transition-transform duration-200 flex-shrink-0',
            isOpen && 'transform rotate-180 text-nofx-gold'
          )}
        />
      </button>

      {isOpen && (
        <div className="absolute top-full right-0 mt-2 w-full min-w-[200px] max-h-[300px] overflow-y-auto nofx-glass border border-white/10 rounded-xl shadow-xl z-50 animate-in fade-in zoom-in-95 duration-100 flex flex-col bg-[#0f172a]/95 backdrop-blur-xl">
          <div className="p-1 space-y-0.5">
            {traders.map((trader) => {
              const isSelected = trader.trader_id === selectedTraderId
              return (
                <button
                  key={trader.trader_id}
                  onClick={() => handleSelect(trader.trader_id)}
                  className={cn(
                    'w-full text-left px-3 py-2.5 rounded-lg text-sm transition-colors flex items-center justify-between group',
                    isSelected
                      ? 'bg-nofx-gold/10 text-nofx-gold'
                      : 'text-nofx-text-muted hover:bg-white/5 hover:text-nofx-text-main'
                  )}
                >
                  <div className="flex items-center gap-2 truncate">
                    <div
                      className={cn(
                        'w-1.5 h-1.5 rounded-full shrink-0 transition-colors',
                        isSelected
                          ? 'bg-nofx-gold'
                          : 'bg-white/20 group-hover:bg-white/40'
                      )}
                    />
                    <div className="flex flex-col truncate">
                      <span className="truncate font-medium">
                        {trader.trader_name}
                      </span>
                      <span className="text-[10px] opacity-50 font-mono truncate">
                        ID: {trader.trader_id.slice(0, 18)}...
                      </span>
                    </div>
                  </div>
                  {isSelected && <Check className="w-3.5 h-3.5 shrink-0" />}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
