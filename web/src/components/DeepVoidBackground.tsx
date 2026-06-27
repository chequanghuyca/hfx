import React from 'react'

interface DeepVoidBackgroundProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode
  className?: string
}

export function DeepVoidBackground({
  children,
  className = '',
  ...props
}: DeepVoidBackgroundProps) {
  return (
    <div
      className={`relative w-full text-nofx-text overflow-hidden flex flex-col ${className}`}
      {...props}
    >
      {/* BACKGROUND LAYERS */}

      {/* 1. Grain/Noise Texture - lighter on mobile */}
      <div className="absolute inset-0 bg-[url('https://grainy-gradients.vercel.app/noise.svg')] opacity-10 sm:opacity-20 mix-blend-soft-light pointer-events-none fixed z-0"></div>

      {/* 2. Grid System - simplified on mobile */}
      <div className="fixed inset-0 pointer-events-none z-0">
        <div
          className="absolute inset-x-0 bottom-0 h-[30vh] sm:h-[50vh] bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:24px_24px] sm:bg-[size:40px_40px] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_0%,#000_70%,transparent_100%)] opacity-30 sm:opacity-50"
          style={{
            transform:
              'perspective(500px) rotateX(60deg) translateY(100px) scale(2)',
          }}
        ></div>
        <div className="absolute inset-0 bg-grid-pattern opacity-[0.02] sm:opacity-[0.03]"></div>
      </div>

      {/* 3. Ambient Glow Spots - smaller and less blur on mobile for performance */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none z-0">
        <div className="absolute top-[-5%] sm:top-[-10%] left-[-5%] sm:left-[-10%] w-[50vw] sm:w-[40vw] h-[50vw] sm:h-[40vw] bg-nofx-gold/5 sm:bg-nofx-gold/10 rounded-full blur-[60px] sm:blur-[120px] mix-blend-screen"></div>
        <div className="absolute bottom-[-5%] sm:bottom-[-10%] right-[-5%] sm:right-[-10%] w-[50vw] sm:w-[40vw] h-[50vw] sm:h-[40vw] bg-nofx-accent/3 sm:bg-nofx-accent/5 rounded-full blur-[60px] sm:blur-[120px] mix-blend-screen"></div>
      </div>

      {/* 4. CRT/Scanline Overlay - hidden on mobile for performance, lighter on tablet */}
      <div className="hidden sm:block fixed inset-0 pointer-events-none z-[9999] opacity-20 md:opacity-40">
        <div className="absolute inset-0 bg-[linear-gradient(rgba(18,16,16,0)_50%,rgba(0,0,0,0.25)_50%),linear-gradient(90deg,rgba(255,0,0,0.06),rgba(0,255,0,0.02),rgba(0,0,255,0.06))] bg-[length:100%_4px,3px_100%] pointer-events-none"></div>
      </div>

      {/* Content Layer */}
      <div className="relative z-10 flex-1 flex flex-col h-full w-full">
        {children}
      </div>
    </div>
  )
}
