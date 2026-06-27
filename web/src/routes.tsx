import { motion } from 'framer-motion'
import { Suspense, lazy, type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from './contexts/AuthContext'

// Route paths as constants
export const ROUTES = {
  HOME: '/',
  LOGIN: '/login',
  REGISTER: '/register',
  RESET_PASSWORD: '/reset-password',
  FAQ: '/faq',
  COMPETITION: '/competition',
  STRATEGY_MARKET: '/strategy-market',
  TRADERS: '/traders',
  DASHBOARD: '/dashboard',
  BACKTEST: '/backtest',
  STRATEGY: '/strategy',
  DEBATE: '/debate',
  NOT_WHITELISTED: '/not-whitelisted',
} as const

// Lazy loaded components
export const LandingPage = lazy(() =>
  import('./pages/LandingPage').then((m) => ({ default: m.LandingPage }))
)
export const LoginPage = lazy(() =>
  import('./components/LoginPage').then((m) => ({ default: m.LoginPage }))
)
export const RegisterPage = lazy(() =>
  import('./components/RegisterPage').then((m) => ({ default: m.RegisterPage }))
)
export const ResetPasswordPage = lazy(() =>
  import('./components/ResetPasswordPage').then((m) => ({
    default: m.ResetPasswordPage,
  }))
)
export const FAQPage = lazy(() =>
  import('./pages/FAQPage').then((m) => ({ default: m.FAQPage }))
)
export const CompetitionPage = lazy(() =>
  import('./components/CompetitionPage').then((m) => ({
    default: m.CompetitionPage,
  }))
)
export const StrategyMarketPage = lazy(() =>
  import('./pages/StrategyMarketPage').then((m) => ({
    default: m.StrategyMarketPage,
  }))
)
export const AITradersPage = lazy(() =>
  import('./components/AITradersPage').then((m) => ({
    default: m.AITradersPage,
  }))
)
export const BacktestPage = lazy(() =>
  import('./components/BacktestPage').then((m) => ({ default: m.BacktestPage }))
)
export const StrategyStudioPage = lazy(() =>
  import('./pages/StrategyStudioPage').then((m) => ({
    default: m.StrategyStudioPage,
  }))
)
export const NotWhitelistedPage = lazy(() =>
  import('./pages/NotWhitelistedPage').then((m) => ({
    default: m.NotWhitelistedPage,
  }))
)
export const DebateArenaPage = lazy(() =>
  import('./pages/DebateArenaPage').then((m) => ({
    default: m.DebateArenaPage,
  }))
)
export const TraderDashboardPage = lazy(() =>
  import('./pages/TraderDashboardPage').then((m) => ({
    default: m.TraderDashboardPage,
  }))
)

// Loading fallback component
export function LoadingFallback() {
  return (
    <div className="fixed inset-0 z-[9999] flex items-center justify-center overflow-hidden bg-[#05070a]">
      {/* Background Cinematic FX */}
      <div className="absolute inset-0 bg-vignette opacity-60" />
      <div
        className="absolute inset-0 opacity-20"
        style={{
          backgroundImage:
            'radial-gradient(circle at 50% 50%, var(--nofx-gold) 0%, transparent 70%)',
        }}
      />

      {/* Decorative Rotating Rings */}
      <motion.div
        animate={{ rotate: 360 }}
        transition={{ duration: 20, repeat: Infinity, ease: 'linear' }}
        className="absolute w-[280px] h-[280px] md:w-[500px] md:h-[500px] border border-nofx-gold/5 rounded-full"
      />
      <motion.div
        animate={{ rotate: -360 }}
        transition={{ duration: 15, repeat: Infinity, ease: 'linear' }}
        className="absolute w-[220px] h-[220px] md:w-[400px] md:h-[400px] border border-nofx-gold/10 rounded-full border-dashed"
      />

      <div className="relative z-10 flex flex-col items-center">
        {/* Animated Logo Container */}
        <motion.div
          initial={{ scale: 0.8, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          transition={{ duration: 0.8, ease: 'easeOut' }}
          className="relative mb-8"
        >
          {/* Logo Glow */}
          <motion.div
            animate={{
              scale: [1, 1.2, 1],
              opacity: [0.3, 0.6, 0.3],
            }}
            transition={{ duration: 3, repeat: Infinity, ease: 'easeInOut' }}
            className="absolute inset-0 bg-nofx-gold blur-2xl rounded-full opacity-30"
          />

          <motion.img
            src="/icons/favicon.ico"
            alt="HFX Logo"
            className="w-16 h-16 md:w-24 md:h-24 relative z-10 drop-shadow-[0_0_15px_rgba(240,185,11,0.5)]"
            animate={{
              y: [0, -10, 0],
            }}
            transition={{ duration: 4, repeat: Infinity, ease: 'easeInOut' }}
          />
        </motion.div>

        {/* Loading Info */}
        <div className="flex flex-col items-center gap-4">
          <motion.p
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.5 }}
            className="text-nofx-gold font-bold tracking-[0.2em] uppercase text-[10px] md:text-sm text-glow"
          >
            System Initializing
          </motion.p>

          {/* Tech Progress Bar */}
          <div className="w-32 md:w-48 h-[2px] bg-white/5 rounded-full overflow-hidden relative">
            <motion.div
              className="absolute inset-0 bg-nofx-gold shadow-[0_0_10px_var(--nofx-gold)]"
              animate={{
                x: ['-100%', '100%'],
              }}
              transition={{
                duration: 1.5,
                repeat: Infinity,
                ease: 'easeInOut',
              }}
            />
          </div>

          <motion.div
            animate={{ opacity: [0.4, 1, 0.4] }}
            transition={{ duration: 2, repeat: Infinity }}
            className="items-center gap-2 mt-2 hidden md:flex"
          >
            <span className="w-1.5 h-1.5 rounded-full bg-nofx-gold animate-pulse" />
            <p className="text-xs text-secondary font-mono">
              Loading modules...
            </p>
          </motion.div>
        </div>
      </div>

      {/* CRT Overlay Effect */}
      <div className="absolute inset-0 crt-overlay opacity-[0.03] pointer-events-none" />
    </div>
  )
}

// Suspense wrapper for lazy components
export function LazyRoute({ children }: { children: ReactNode }) {
  return <Suspense fallback={<LoadingFallback />}>{children}</Suspense>
}

// Protected route wrapper - redirects to login if not authenticated
interface ProtectedRouteProps {
  children: ReactNode
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { user, token, isLoading } = useAuth()
  const location = useLocation()

  if (isLoading) {
    return <LoadingFallback />
  }

  if (!user || !token) {
    // Save the attempted URL for redirecting after login
    sessionStorage.setItem('returnUrl', location.pathname)
    return <Navigate to={ROUTES.HOME} replace />
  }

  return <>{children}</>
}

// Public route wrapper - redirects to dashboard if already authenticated
interface PublicRouteProps {
  children: ReactNode
  redirectTo?: string
}

export function PublicRoute({
  children,
  redirectTo = ROUTES.DASHBOARD,
}: PublicRouteProps) {
  const { user, token, isLoading } = useAuth()

  if (isLoading) {
    return <LoadingFallback />
  }

  if (user && token) {
    return <Navigate to={redirectTo} replace />
  }

  return <>{children}</>
}
