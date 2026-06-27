import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { api } from './lib/api'

import { ConfirmDialogProvider } from './components/ConfirmDialog'
import HeaderBar from './components/HeaderBar'
import { LoginRequiredOverlay } from './components/LoginRequiredOverlay'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { LanguageProvider, useLanguage } from './contexts/LanguageContext'
import { useSystemConfig } from './hooks/useSystemConfig'

import {
  AITradersPage,
  BacktestPage,
  CompetitionPage,
  DebateArenaPage,
  FAQPage,
  LandingPage,
  LazyRoute,
  LoadingFallback,
  LoginPage,
  NotWhitelistedPage,
  ProtectedRoute,
  ROUTES,
  RegisterPage,
  ResetPasswordPage,
  StrategyMarketPage,
  StrategyStudioPage,
  TraderDashboardPage,
} from './routes'

import type {
  AccountInfo,
  DecisionRecord,
  Exchange,
  Position,
  Statistics,
  SystemStatus,
  TraderInfo,
} from './types'

type Page =
  | 'competition'
  | 'traders'
  | 'trader'
  | 'backtest'
  | 'strategy'
  | 'strategy-market'
  | 'debate'
  | 'faq'
  | 'login'
  | 'register'

// Map route paths to page names
const pathToPage: Record<string, Page> = {
  [ROUTES.COMPETITION]: 'competition',
  [ROUTES.STRATEGY_MARKET]: 'strategy-market',
  [ROUTES.TRADERS]: 'traders',
  [ROUTES.DASHBOARD]: 'trader',
  [ROUTES.BACKTEST]: 'backtest',
  [ROUTES.STRATEGY]: 'strategy',
  [ROUTES.DEBATE]: 'debate',
  [ROUTES.FAQ]: 'faq',
  [ROUTES.LOGIN]: 'login',
  [ROUTES.REGISTER]: 'register',
}

// Map page names to route paths
const pageToPath: Record<Page, string> = {
  competition: ROUTES.COMPETITION,
  'strategy-market': ROUTES.STRATEGY_MARKET,
  traders: ROUTES.TRADERS,
  trader: ROUTES.DASHBOARD,
  backtest: ROUTES.BACKTEST,
  strategy: ROUTES.STRATEGY,
  debate: ROUTES.DEBATE,
  faq: ROUTES.FAQ,
  login: ROUTES.LOGIN,
  register: ROUTES.REGISTER,
}

// Layout wrapper for authenticated pages - defined OUTSIDE App to prevent re-mounting
function AuthenticatedLayout({ children }: { children: React.ReactNode }) {
  const { language, setLanguage } = useLanguage()
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [loginOverlayOpen, setLoginOverlayOpen] = useState(false)
  const [loginOverlayFeature, setLoginOverlayFeature] = useState('')

  // Get current page from path
  const currentPage = (pathToPage[location.pathname] || 'competition') as Page

  const handleLoginRequired = useCallback((featureName: string) => {
    setLoginOverlayFeature(featureName)
    setLoginOverlayOpen(true)
  }, [])

  const navigateToPage = useCallback(
    (page: Page) => {
      const path = pageToPath[page]
      if (path) {
        navigate(path)
      }
    },
    [navigate]
  )

  return (
    <div
      style={{
        background: '#0B0E11',
        color: '#EAECEF',
      }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={currentPage}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onLoginRequired={handleLoginRequired}
        onPageChange={navigateToPage}
      />
      <main className="pt-16 min-h-screen">{children}</main>
      <LoginRequiredOverlay
        isOpen={loginOverlayOpen}
        onClose={() => setLoginOverlayOpen(false)}
        featureName={loginOverlayFeature}
      />
    </div>
  )
}

// Public layout for FAQ page - defined OUTSIDE App to prevent re-mounting
function PublicLayout({ children }: { children: React.ReactNode }) {
  const { language, setLanguage } = useLanguage()
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [loginOverlayOpen, setLoginOverlayOpen] = useState(false)
  const [loginOverlayFeature, setLoginOverlayFeature] = useState('')

  const handleLoginRequired = useCallback((featureName: string) => {
    setLoginOverlayFeature(featureName)
    setLoginOverlayOpen(true)
  }, [])

  const navigateToPage = useCallback(
    (page: Page) => {
      const path = pageToPath[page]
      if (path) {
        navigate(path)
      }
    },
    [navigate]
  )

  return (
    <div
      className="min-h-screen"
      style={{ background: '#0B0E11', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage="faq"
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onLoginRequired={handleLoginRequired}
        onPageChange={navigateToPage}
      />
      {children}
      <LoginRequiredOverlay
        isOpen={loginOverlayOpen}
        onClose={() => setLoginOverlayOpen(false)}
        featureName={loginOverlayFeature}
      />
    </div>
  )
}

function App() {
  const { language } = useLanguage()
  const { user, token, isLoading } = useAuth()
  const { loading: configLoading, config } = useSystemConfig()
  const navigate = useNavigate()
  const location = useLocation()

  // Wails Event Listener for Desktop Notifications
  useEffect(() => {
    if ((window as any).runtime && (window as any).runtime.EventsOn) {
      const runtime = (window as any).runtime
      runtime.EventsOn(
        'notification',
        (data: { title: string; message: string }) => {
          // You can use your existing toast library here
          // or just rely on the native Dialog already called in Go.
          console.log('Desktop Notification Received:', data)
        }
      )
    }
  }, [])

  // Read trader slug from URL params
  const [selectedTraderSlug, setSelectedTraderSlug] = useState<
    string | undefined
  >(() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('trader') || undefined
  })
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>()
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')
  const [decisionsLimit, setDecisionsLimit] = useState<number>(5)

  // Generate trader URL slug (name + first 4 chars of ID)
  const getTraderSlug = (trader: TraderInfo) => {
    const idPrefix = trader.trader_id.slice(0, 4)
    return `${trader.trader_name}-${idPrefix}`
  }

  // Find trader by slug
  const findTraderBySlug = (slug: string, traderList: TraderInfo[]) => {
    const lastDashIndex = slug.lastIndexOf('-')
    if (lastDashIndex === -1) {
      return traderList.find((t) => t.trader_name === slug)
    }
    const name = slug.slice(0, lastDashIndex)
    const idPrefix = slug.slice(lastDashIndex + 1)
    return traderList.find(
      (t) => t.trader_name === name && t.trader_id.startsWith(idPrefix)
    )
  }

  // Fetch trader list (only when logged in)
  const { data: traders, error: tradersError } = useQuery<TraderInfo[]>({
    queryKey: ['traders'],
    queryFn: api.getTraders,
    enabled: !!(user && token),
    staleTime: 5000,
    refetchInterval: 15000, // Faster refresh to detect Start/Stop status
    placeholderData: keepPreviousData,
    retry: 1,
    notifyOnChangeProps: ['data'],
  })

  // Fetch exchange list
  const { data: exchanges } = useQuery<Exchange[]>({
    queryKey: ['exchanges'],
    queryFn: api.getExchangeConfigs,
    enabled: !!(user && token),
    staleTime: 120000,
    refetchInterval: 300000,
    placeholderData: keepPreviousData,
    notifyOnChangeProps: ['data'],
  })

  // Set selected trader when data is fetched
  useEffect(() => {
    if (traders && traders.length > 0 && !selectedTraderId) {
      if (selectedTraderSlug) {
        const trader = findTraderBySlug(selectedTraderSlug, traders)
        if (trader) {
          setSelectedTraderId(trader.trader_id)
        } else {
          setSelectedTraderId(traders[0].trader_id)
        }
      } else {
        setSelectedTraderId(traders[0].trader_id)
      }
    }
  }, [traders, selectedTraderId, selectedTraderSlug])

  // Update trader slug from URL params
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const traderParam = params.get('trader')
    if (traderParam && traderParam !== selectedTraderSlug) {
      setSelectedTraderSlug(traderParam)
    }
  }, [location.search, selectedTraderSlug])

  // Memoized handlers
  const handleTraderSelect = useCallback(
    (traderId: string) => {
      setSelectedTraderId(traderId)
      const trader = traders?.find((t) => t.trader_id === traderId)
      if (trader) {
        const url = new URL(window.location.href)
        url.searchParams.set('trader', getTraderSlug(trader))
        window.history.replaceState({}, '', url.toString())
      }
    },
    [traders]
  )

  const handleNavigateToTraders = useCallback(() => {
    navigate(ROUTES.TRADERS)
  }, [navigate])

  // Fetch trader data when on dashboard page
  const isOnDashboard = location.pathname === ROUTES.DASHBOARD

  // Find current trader status - moved up to gate all trader queries
  const activeTrader = useMemo(() => {
    return traders?.find((t) => t.trader_id === selectedTraderId)
  }, [traders, selectedTraderId])

  const isTraderRunning = activeTrader?.is_running ?? false

  const { data: status } = useQuery<SystemStatus>({
    queryKey: ['status', selectedTraderId],
    queryFn: () => api.getStatus(selectedTraderId),
    enabled: isOnDashboard && !!selectedTraderId && isTraderRunning,
    placeholderData: keepPreviousData,
    notifyOnChangeProps: ['data'],
  })

  const { data: account } = useQuery<AccountInfo>({
    queryKey: ['account', selectedTraderId],
    queryFn: () => api.getAccount(selectedTraderId),
    enabled: !!selectedTraderId && isTraderRunning,
    staleTime: 5000,
    refetchInterval: 15000, // Sync account every 15s
    placeholderData: keepPreviousData,
    refetchIntervalInBackground: true,
    notifyOnChangeProps: ['data'],
  })

  const { data: positions } = useQuery<Position[]>({
    queryKey: ['positions', selectedTraderId],
    queryFn: () => api.getPositions(selectedTraderId),
    enabled: !!selectedTraderId && isTraderRunning,
    staleTime: 2000,
    refetchInterval: 5000, // Current positions are priority - 5s
    refetchIntervalInBackground: true,
    placeholderData: keepPreviousData,
    notifyOnChangeProps: ['data'],
  })

  // Global Dynamic Browser Title
  const totalPnL = useMemo(
    () => positions?.reduce((acc, pos) => acc + pos.unrealized_pnl, 0) || 0,
    [positions]
  )

  useEffect(() => {
    const originalTitle = 'HFX - AI Auto Trading'

    if (positions && positions.length > 0) {
      const pnlStr =
        totalPnL >= 0 ? `+${totalPnL.toFixed(2)}` : totalPnL.toFixed(2)
      document.title = `${pnlStr} | ${
        positions.length > 1
          ? positions.length + ' positions'
          : positions[0].symbol
      } USDⓈ-Margined`
    } else {
      document.title = originalTitle
    }

    return () => {
      document.title = originalTitle
    }
  }, [totalPnL, positions])

  const { data: decisions, refetch: refetchDecisions } = useQuery<
    DecisionRecord[]
  >({
    queryKey: ['decisions', selectedTraderId, decisionsLimit],
    queryFn: () => api.getLatestDecisions(selectedTraderId, decisionsLimit),
    enabled: isOnDashboard && !!selectedTraderId && isTraderRunning,
    staleTime: 10000,
    refetchInterval: 20000, // Check for new decisions every 20s
    placeholderData: keepPreviousData,
    notifyOnChangeProps: ['data'],
  })

  const { data: stats } = useQuery<Statistics>({
    queryKey: ['statistics', selectedTraderId],
    queryFn: () => api.getStatistics(selectedTraderId),
    enabled: isOnDashboard && !!selectedTraderId && isTraderRunning,
    staleTime: 10000,
    refetchInterval: 15000, // 15s - stats don't need real-time updates
    placeholderData: keepPreviousData,
    notifyOnChangeProps: ['data'],
  })

  useEffect(() => {
    if (account) {
      const now = new Date().toLocaleTimeString()
      setLastUpdate(now)
    }
  }, [account])

  // Find selected trader with fallback to prevent flickering
  const foundTrader = traders?.find((t) => t.trader_id === selectedTraderId)
  /* Safe fallback to prevent flickering using state */
  const [lastValidTrader, setLastValidTrader] = useState<
    TraderInfo | undefined
  >(undefined)

  useEffect(() => {
    if (foundTrader) {
      setLastValidTrader(foundTrader)
    }
  }, [foundTrader])

  const selectedTrader =
    foundTrader ||
    (lastValidTrader?.trader_id === selectedTraderId
      ? lastValidTrader
      : undefined)

  // Show loading spinner while checking auth or config
  // Relaxed check: Only show loading if we really have no data
  if ((isLoading && !user) || (configLoading && !config)) {
    return <LoadingFallback />
  }

  return (
    <Routes>
      {/* Public routes */}
      <Route
        path={ROUTES.HOME}
        element={
          <LazyRoute>
            <LandingPage />
          </LazyRoute>
        }
      />
      <Route
        path={ROUTES.LOGIN}
        element={
          <LazyRoute>
            <LoginPage />
          </LazyRoute>
        }
      />
      <Route
        path={ROUTES.REGISTER}
        element={
          <LazyRoute>
            <RegisterPage />
          </LazyRoute>
        }
      />
      <Route
        path={ROUTES.RESET_PASSWORD}
        element={
          <LazyRoute>
            <ResetPasswordPage />
          </LazyRoute>
        }
      />
      <Route
        path={ROUTES.NOT_WHITELISTED}
        element={
          <LazyRoute>
            <NotWhitelistedPage />
          </LazyRoute>
        }
      />
      <Route
        path={ROUTES.FAQ}
        element={
          <PublicLayout>
            <LazyRoute>
              <FAQPage />
            </LazyRoute>
          </PublicLayout>
        }
      />

      {/* Protected routes */}
      <Route
        path={ROUTES.COMPETITION}
        element={
          <ProtectedRoute>
            <AuthenticatedLayout>
              <LazyRoute>
                <CompetitionPage />
              </LazyRoute>
            </AuthenticatedLayout>
          </ProtectedRoute>
        }
      />
      <Route
        path={ROUTES.STRATEGY_MARKET}
        element={
          <ProtectedRoute>
            <AuthenticatedLayout>
              <LazyRoute>
                <StrategyMarketPage />
              </LazyRoute>
            </AuthenticatedLayout>
          </ProtectedRoute>
        }
      />
      <Route
        path={ROUTES.TRADERS}
        element={
          <ProtectedRoute>
            <AuthenticatedLayout>
              <LazyRoute>
                <AITradersPage
                  onTraderSelect={(traderId) => {
                    setSelectedTraderId(traderId)
                    navigate(ROUTES.DASHBOARD)
                  }}
                />
              </LazyRoute>
            </AuthenticatedLayout>
          </ProtectedRoute>
        }
      />
      <Route
        path={ROUTES.BACKTEST}
        element={
          <ProtectedRoute>
            <AuthenticatedLayout>
              <LazyRoute>
                <BacktestPage />
              </LazyRoute>
            </AuthenticatedLayout>
          </ProtectedRoute>
        }
      />
      <Route
        path={ROUTES.STRATEGY}
        element={
          <ProtectedRoute>
            <AuthenticatedLayout>
              <LazyRoute>
                <StrategyStudioPage />
              </LazyRoute>
            </AuthenticatedLayout>
          </ProtectedRoute>
        }
      />
      <Route
        path={ROUTES.DEBATE}
        element={
          <ProtectedRoute>
            <AuthenticatedLayout>
              <LazyRoute>
                <DebateArenaPage />
              </LazyRoute>
            </AuthenticatedLayout>
          </ProtectedRoute>
        }
      />
      <Route
        path={ROUTES.DASHBOARD}
        element={
          <ProtectedRoute>
            <AuthenticatedLayout>
              <LazyRoute>
                <TraderDashboardPage
                  selectedTrader={selectedTrader}
                  status={status}
                  account={account}
                  positions={positions}
                  decisions={decisions}
                  refetchDecisions={refetchDecisions}
                  decisionsLimit={decisionsLimit}
                  onDecisionsLimitChange={setDecisionsLimit}
                  stats={stats}
                  lastUpdate={lastUpdate}
                  language={language}
                  traders={traders}
                  tradersError={tradersError || undefined}
                  selectedTraderId={selectedTraderId}
                  onTraderSelect={handleTraderSelect}
                  onNavigateToTraders={handleNavigateToTraders}
                  exchanges={exchanges}
                />
              </LazyRoute>
            </AuthenticatedLayout>
          </ProtectedRoute>
        }
      />

      {/* Fallback - redirect to home */}
      <Route
        path="*"
        element={
          <LazyRoute>
            <LandingPage />
          </LazyRoute>
        }
      />
    </Routes>
  )
}

// Wrap App with providers
export default function AppWithProviders() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <ConfirmDialogProvider>
          <App />
        </ConfirmDialogProvider>
      </AuthProvider>
    </LanguageProvider>
  )
}
