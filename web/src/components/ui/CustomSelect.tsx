import { Check, ChevronDown } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { cn } from '../../lib/cn'

export interface SelectOption {
  value: string | number
  label: string
}

interface CustomSelectProps {
  options: SelectOption[]
  value: string | number
  onChange: (value: string | number) => void
  placeholder?: string
  disabled?: boolean
  className?: string
  searchable?: boolean
  maxHeight?: string
}

export function CustomSelect({
  options,
  value,
  onChange,
  placeholder = 'Select...',
  disabled = false,
  className,
  searchable = false,
  maxHeight = '300px',
}: CustomSelectProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [positionStyle, setPositionStyle] = useState<React.CSSProperties>({})
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const selectedOption = options.find((opt) => opt.value === value)

  const filteredOptions = options.filter((option) =>
    String(option.label).toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Calculate position and open
  const toggleOpen = () => {
    if (disabled) return

    if (!isOpen && containerRef.current) {
      // Opening
      updatePosition()
    }
    setIsOpen(!isOpen)
  }

  const updatePosition = () => {
    if (containerRef.current) {
      const rect = containerRef.current.getBoundingClientRect()
      const spaceBelow = window.innerHeight - rect.bottom
      const spaceAbove = rect.top
      const dropdownHeight = Math.min(
        parseInt(maxHeight) || 300,
        options.length * 36 + (searchable ? 50 : 0)
      )

      const showAbove =
        spaceBelow < dropdownHeight && spaceAbove > dropdownHeight

      setPositionStyle({
        position: 'fixed',
        left: `${rect.left}px`,
        top: showAbove ? 'auto' : `${rect.bottom + 8}px`,
        bottom: showAbove ? `${window.innerHeight - rect.top + 8}px` : 'auto',
        width: `${rect.width}px`,
        zIndex: 9999, // Ensure it's on top
      })
    }
  }

  // Handle click outside to close dropdown
  useEffect(() => {
    // If we use a transparent overlay for closing, or check target carefully.
    // Let's use a ref for the dropdown content.
    const handleDocumentClick = (e: MouseEvent) => {
      if (!isOpen) return

      const target = e.target as Node
      const dropdownElement = document.getElementById('custom-select-dropdown')

      if (
        containerRef.current &&
        !containerRef.current.contains(target) &&
        dropdownElement &&
        !dropdownElement.contains(target)
      ) {
        setIsOpen(false)
      }
    }

    document.addEventListener('mousedown', handleDocumentClick)

    // Also update position on scroll/resize
    const handleScrollOrResize = () => {
      if (isOpen) {
        // For simplicity, just close on scroll/resize to avoid complex tracking logic
        // or updatePosition() if we want it to stick (but sticking with fixed pos requires constant updates)
        setIsOpen(false)
      }
    }

    window.addEventListener('scroll', handleScrollOrResize, true)
    window.addEventListener('resize', handleScrollOrResize)

    return () => {
      document.removeEventListener('mousedown', handleDocumentClick)
      window.removeEventListener('scroll', handleScrollOrResize, true)
      window.removeEventListener('resize', handleScrollOrResize)
    }
  }, [isOpen])

  // Focus input when opened
  useEffect(() => {
    if (isOpen && searchable) {
      setTimeout(() => {
        inputRef.current?.focus()
      }, 50)
    }
    if (!isOpen) {
      setSearchTerm('')
    }
  }, [isOpen, searchable])

  const handleSelect = (val: string | number) => {
    onChange(val)
    setIsOpen(false)
    setSearchTerm('')
  }

  return (
    <div className={cn('relative', className)} ref={containerRef}>
      <button
        type="button"
        onClick={toggleOpen}
        disabled={disabled}
        className={cn(
          'flex items-center gap-2 px-3 py-1.5 rounded-lg transition-all duration-200',
          'bg-white/5 border border-white/10 hover:bg-white/10 hover:border-nofx-gold/30',
          'text-sm font-medium text-nofx-text-main focus:outline-none',
          'justify-between w-full',
          disabled && 'opacity-50 cursor-not-allowed',
          isOpen && 'bg-white/10 border-nofx-gold/30 ring-1 ring-nofx-gold/20'
        )}
      >
        <span className="truncate">
          {selectedOption ? selectedOption.label : placeholder}
        </span>
        <ChevronDown
          className={cn(
            'w-4 h-4 text-nofx-text-muted transition-transform duration-200 flex-shrink-0',
            isOpen && 'transform rotate-180 text-nofx-gold'
          )}
        />
      </button>

      {isOpen &&
        createPortal(
          <div
            id="custom-select-dropdown"
            className={cn(
              'overflow-hidden nofx-glass border border-white/10 rounded-xl shadow-xl animate-in fade-in zoom-in-95 duration-100 flex flex-col bg-[#0f172a]/95 backdrop-blur-xl'
            )}
            style={positionStyle}
          >
            {searchable && (
              <div className="p-2 border-b border-white/10 top-0 sticky bg-[#0f172a] z-10 w-full">
                <input
                  ref={inputRef}
                  type="text"
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  placeholder="Search..."
                  className="w-full bg-white/5 border border-white/10 rounded-md px-2 py-1.5 text-xs text-nofx-text-main focus:outline-none focus:border-nofx-gold/50"
                  onClick={(e) => e.stopPropagation()}
                />
              </div>
            )}
            <div
              className="p-1 space-y-0.5 overflow-y-auto"
              style={{ maxHeight }}
            >
              {filteredOptions.length > 0 ? (
                filteredOptions.map((option) => {
                  const isSelected = option.value === value
                  return (
                    <button
                      key={option.value}
                      onClick={() => handleSelect(option.value)}
                      className={cn(
                        'w-full text-left px-3 py-2 rounded-lg text-sm transition-colors flex items-center justify-between group',
                        isSelected
                          ? 'bg-nofx-gold/10 text-nofx-gold'
                          : 'text-nofx-text-muted hover:bg-white/5 hover:text-nofx-text-main'
                      )}
                    >
                      <span className="truncate whitespace-nowrap">
                        {option.label}
                      </span>
                      {isSelected && (
                        <Check className="w-3.5 h-3.5 shrink-0 ml-2" />
                      )}
                    </button>
                  )
                })
              ) : (
                <div className="px-3 py-4 text-center text-xs text-nofx-text-muted">
                  No results found
                </div>
              )}
            </div>
          </div>,
          document.body
        )}
    </div>
  )
}
