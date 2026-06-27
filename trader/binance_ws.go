package trader

import (
	"context"
	"nofx/logger"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// BinanceWsManager manages WebSocket connection for Binance Futures User Data Stream
// This provides real-time updates for account balance and positions without REST API calls
type BinanceWsManager struct {
	client    *futures.Client
	listenKey string

	// Cached data from WebSocket
	balance    map[string]interface{}
	positions  []map[string]interface{}
	markPrices map[string]float64 // symbol -> mark price (real-time)

	// Connection management
	stopC          chan struct{}
	doneC          chan struct{}
	isConnected    bool
	lastUserDataUpdate  time.Time
	lastMarkPriceUpdate time.Time
	lastInitTime        time.Time

	// Mark price stream
	markPriceStopC chan struct{}
	markPriceDoneC chan struct{}

	// Mutex for thread-safe access
	mu sync.RWMutex

	// Keep alive ticker
	keepAliveTicker *time.Ticker
	keepAliveStop   chan struct{}

	startOnce sync.Once
}

var (
	wsPool   = make(map[string]*BinanceWsManager)
	wsPoolMu sync.Mutex
)

// GetSharedBinanceWsManager returns a shared WebSocket manager for an API key
func GetSharedBinanceWsManager(client *futures.Client, apiKey string) *BinanceWsManager {
	if apiKey == "" {
		return NewBinanceWsManager(client)
	}

	wsPoolMu.Lock()
	defer wsPoolMu.Unlock()

	if mgr, ok := wsPool[apiKey]; ok {
		return mgr
	}

	mgr := NewBinanceWsManager(client)
	wsPool[apiKey] = mgr
	return mgr
}

// NewBinanceWsManager creates a new WebSocket manager for Binance Futures
func NewBinanceWsManager(client *futures.Client) *BinanceWsManager {
	return &BinanceWsManager{
		client:     client,
		balance:    make(map[string]interface{}),
		positions:  make([]map[string]interface{}, 0),
		markPrices: make(map[string]float64),
	}
}


// Start starts the WebSocket connection (idempotent)
func (m *BinanceWsManager) Start() error {
	m.mu.Lock()
	if m.isConnected {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	var err error
	m.startOnce.Do(func() {
		// Fetch initial data from REST API to seed WebSocket cache
		if err2 := m.InitializeFromREST(); err2 != nil {
			logger.Infof("⚠️ Binance WebSocket: Failed to initialize from REST: %v", err2)
		}

		// Create listen key
		listenKey, err2 := m.client.NewStartUserStreamService().Do(context.Background())
		if err2 != nil {
			err = err2
			return
		}
		m.listenKey = listenKey

		logger.Infof("🔌 Binance WebSocket: Listen key created")

		// Start keep alive goroutine (refresh every 30 minutes)
		m.keepAliveTicker = time.NewTicker(30 * time.Minute)
		m.keepAliveStop = make(chan struct{})
		go m.keepAliveLoop()

		// Connect to WebSocket
		err = m.connect()
	})

	return err
}

// connect establishes WebSocket connection
func (m *BinanceWsManager) connect() error {
	m.mu.Lock()
	if m.stopC != nil {
		close(m.stopC)
	}
	m.mu.Unlock()

	// Define handlers
	wsHandler := func(event *futures.WsUserDataEvent) {
		m.handleEvent(event)
	}

	errHandler := func(err error) {
		logger.Infof("⚠️ Binance WebSocket error: %v", err)
		m.mu.Lock()
		m.isConnected = false
		m.mu.Unlock()

		// Attempt reconnection after 5 seconds
		go func() {
			time.Sleep(5 * time.Second)
			logger.Infof("🔄 Binance WebSocket: Attempting reconnection...")
			if err := m.connect(); err != nil {
				logger.Infof("❌ Binance WebSocket: Reconnection failed: %v", err)
			}
		}()
	}

	// Start WebSocket
	doneC, stopC, err := futures.WsUserDataServe(m.listenKey, wsHandler, errHandler)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.doneC = doneC
	m.stopC = stopC
	m.isConnected = true
	m.mu.Unlock()

	logger.Infof("✅ Binance WebSocket connected (User Data Stream)")

	// Start mark price stream for real-time price updates
	go m.startMarkPriceStream()

	return nil
}

// startMarkPriceStream starts the all mark price stream for real-time price updates
func (m *BinanceWsManager) startMarkPriceStream() {
	handler := func(events futures.WsAllMarkPriceEvent) {
		m.mu.Lock()
		defer m.mu.Unlock()

		for _, event := range events {
			markPrice, err := strconv.ParseFloat(event.MarkPrice, 64)
			if err == nil && markPrice > 0 {
				m.markPrices[event.Symbol] = markPrice
				m.lastMarkPriceUpdate = time.Now()
			}
		}
	}

	errHandler := func(err error) {
		logger.Infof("⚠️ Mark Price WebSocket error: %v", err)
		// Attempt reconnection after 5 seconds
		go func() {
			time.Sleep(5 * time.Second)
			if m.isConnected {
				logger.Infof("🔄 Mark Price WebSocket: Attempting reconnection...")
				m.startMarkPriceStream()
			}
		}()
	}

	doneC, stopC, err := futures.WsAllMarkPriceServe(handler, errHandler)
	if err != nil {
		logger.Infof("⚠️ Failed to start mark price stream: %v", err)
		return
	}

	m.mu.Lock()
	m.markPriceDoneC = doneC
	m.markPriceStopC = stopC
	m.mu.Unlock()

	logger.Infof("✅ Binance Mark Price WebSocket connected (real-time prices)")
}

// handleEvent processes WebSocket events
func (m *BinanceWsManager) handleEvent(event *futures.WsUserDataEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastUserDataUpdate = time.Now()

	switch event.Event {
	case futures.UserDataEventTypeAccountUpdate:
		m.handleAccountUpdate(event)
	case futures.UserDataEventTypeOrderTradeUpdate:
		m.handleOrderTradeUpdate(event)
	case futures.UserDataEventTypeListenKeyExpired:
		logger.Infof("⚠️ Binance WebSocket: Listen key expired, reconnecting...")
		go func() {
			if err := m.Start(); err != nil {
				logger.Infof("❌ Binance WebSocket: Failed to restart: %v", err)
			}
		}()
	}
}

// handleAccountUpdate processes ACCOUNT_UPDATE events
func (m *BinanceWsManager) handleAccountUpdate(event *futures.WsUserDataEvent) {
	update := event.AccountUpdate
	if update.Reason == "" && len(update.Balances) == 0 && len(update.Positions) == 0 {
		return
	}

	// Update balances
	for _, b := range update.Balances {
		if b.Asset == "USDT" {
			balance, _ := strconv.ParseFloat(b.Balance, 64)
			crossWallet, _ := strconv.ParseFloat(b.CrossWalletBalance, 64)

			m.balance["totalWalletBalance"] = balance
			m.balance["availableBalance"] = crossWallet

			logger.Infof("📊 WebSocket: Balance update - USDT wallet=%.2f, available=%.2f", balance, crossWallet)
		}
	}

	// Update positions
	for _, p := range update.Positions {
		posAmt, _ := strconv.ParseFloat(p.Amount, 64)
		if posAmt == 0 {
			// Position closed - remove from cache
			m.removePosition(p.Symbol, string(p.Side))
			continue
		}

		entryPrice, _ := strconv.ParseFloat(p.EntryPrice, 64)
		markPrice, _ := strconv.ParseFloat(p.MarkPrice, 64)
		unrealizedPnL, _ := strconv.ParseFloat(p.UnrealizedPnL, 64)
		isolatedWallet, _ := strconv.ParseFloat(p.IsolatedWallet, 64)

		posMap := map[string]interface{}{
			"symbol":           p.Symbol,
			"positionAmt":      posAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealizedPnL,
			"isolatedWallet":   isolatedWallet,
			"marginType":       string(p.MarginType),
		}

		// Determine side
		if posAmt > 0 {
			posMap["side"] = "long"
		} else {
			posMap["side"] = "short"
		}

		m.updatePosition(posMap)
		logger.Infof("📊 WebSocket: Position update - %s %s qty=%.4f entry=%.4f mark=%.4f pnl=%.2f",
			p.Symbol, posMap["side"], posAmt, entryPrice, markPrice, unrealizedPnL)
	}

	// Calculate total unrealized PnL
	totalUnrealizedPnL := 0.0
	for _, pos := range m.positions {
		if pnl, ok := pos["unRealizedProfit"].(float64); ok {
			totalUnrealizedPnL += pnl
		}
	}
	m.balance["totalUnrealizedProfit"] = totalUnrealizedPnL
}

// handleOrderTradeUpdate processes ORDER_TRADE_UPDATE events
func (m *BinanceWsManager) handleOrderTradeUpdate(event *futures.WsUserDataEvent) {
	order := event.OrderTradeUpdate
	logger.Infof("📊 WebSocket: Order update - %s %s %s status=%s filled=%s",
		order.Symbol, order.Side, order.Type, order.Status, order.AccumulatedFilledQty)
}

// updatePosition updates or adds a position in the cache
// It merges new fields with existing position data to preserve fields from REST API
func (m *BinanceWsManager) updatePosition(newPos map[string]interface{}) {
	symbol := newPos["symbol"].(string)
	side := newPos["side"].(string)

	for i, pos := range m.positions {
		if pos["symbol"] == symbol && pos["side"] == side {
			// Merge: update existing position with new values, keeping old fields
			for k, v := range newPos {
				m.positions[i][k] = v
			}
			return
		}
	}

	// Position not found - this is a new position from WebSocket
	// Add default values for fields that auto_trader expects
	if _, ok := newPos["leverage"]; !ok {
		newPos["leverage"] = 10.0 // default leverage
	}
	if _, ok := newPos["liquidationPrice"]; !ok {
		newPos["liquidationPrice"] = 0.0 // will be updated on next REST API sync
	}

	m.positions = append(m.positions, newPos)
}

// removePosition removes a position from the cache
func (m *BinanceWsManager) removePosition(symbol, positionSide string) {
	side := "long"
	if positionSide == "SHORT" {
		side = "short"
	}

	for i, pos := range m.positions {
		if pos["symbol"] == symbol && pos["side"] == side {
			m.positions = append(m.positions[:i], m.positions[i+1:]...)
			logger.Infof("📊 WebSocket: Position closed - %s %s", symbol, side)
			return
		}
	}
}

// keepAliveLoop keeps the listen key alive
func (m *BinanceWsManager) keepAliveLoop() {
	for {
		select {
		case <-m.keepAliveTicker.C:
			if m.listenKey != "" {
				err := m.client.NewKeepaliveUserStreamService().ListenKey(m.listenKey).Do(context.Background())
				if err != nil {
					logger.Infof("⚠️ Binance WebSocket: Keep alive failed: %v, reconnecting...", err)
					go func() {
						if err := m.Start(); err != nil {
							logger.Infof("❌ Binance WebSocket: Failed to restart: %v", err)
						}
					}()
				} else {
					logger.Infof("✅ Binance WebSocket: Listen key refreshed")
				}
			}
		case <-m.keepAliveStop:
			return
		}
	}
}

// Stop stops the WebSocket connection
func (m *BinanceWsManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.keepAliveTicker != nil {
		m.keepAliveTicker.Stop()
	}
	if m.keepAliveStop != nil {
		close(m.keepAliveStop)
	}

	if m.stopC != nil {
		close(m.stopC)
	}

	// Close mark price stream
	if m.markPriceStopC != nil {
		close(m.markPriceStopC)
	}

	// Close listen key
	if m.listenKey != "" {
		m.client.NewCloseUserStreamService().ListenKey(m.listenKey).Do(context.Background())
	}


	m.isConnected = false
	logger.Infof("🔌 Binance WebSocket: Disconnected")
}

// IsConnected returns whether WebSocket is connected
func (m *BinanceWsManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isConnected
}

// GetBalance returns cached balance from WebSocket
// Returns nil if WebSocket is not connected or data is stale (> 60s)
func (m *BinanceWsManager) GetBalance() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.isConnected {
		return nil
	}

	// Stale data check: if no User Data update in 10 minutes, fallback to REST
	if time.Since(m.lastUserDataUpdate) > 10*time.Minute {
		return nil
	}

	// Return a copy to prevent race conditions
	result := make(map[string]interface{})
	for k, v := range m.balance {
		result[k] = v
	}
	return result
}

// GetPositions returns cached positions from WebSocket
// Returns nil if WebSocket is not connected
// Mark prices are updated in real-time from mark price stream
func (m *BinanceWsManager) GetPositions() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.isConnected {
		return nil
	}

	// Stale data check: if no User Data update in 10 minutes, fallback to REST
	if time.Since(m.lastUserDataUpdate) > 10*time.Minute {
		return nil
	}

	// Return a copy with real-time mark prices
	result := make([]map[string]interface{}, len(m.positions))
	for i, pos := range m.positions {
		posCopy := make(map[string]interface{})
		for k, v := range pos {
			posCopy[k] = v
		}

		// Update mark price from real-time stream if available
		symbol := pos["symbol"].(string)
		if realTimeMarkPrice, ok := m.markPrices[symbol]; ok && realTimeMarkPrice > 0 {
			posCopy["markPrice"] = realTimeMarkPrice

			// Recalculate unrealized PnL with real-time mark price
			if entryPrice, ok := pos["entryPrice"].(float64); ok {
				if posAmt, ok := pos["positionAmt"].(float64); ok {
					unrealizedPnL := (realTimeMarkPrice - entryPrice) * posAmt
					posCopy["unRealizedProfit"] = unrealizedPnL
				}
			}
		}

		result[i] = posCopy
	}
	return result
}

// GetMarketPrice returns real-time mark price for a symbol
func (m *BinanceWsManager) GetMarketPrice(symbol string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if data is fresh (less than 30 seconds old)
	if time.Since(m.lastMarkPriceUpdate) > 30*time.Second {
		return 0
	}

	return m.markPrices[symbol]
}

// InitializeFromREST fetches initial data from REST API to populate cache
func (m *BinanceWsManager) InitializeFromREST() error {
	m.mu.Lock()
	if time.Since(m.lastInitTime) < 1*time.Minute && len(m.balance) > 0 {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	// Get initial account info
	account, err := m.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Set balance
	m.balance["totalWalletBalance"], _ = strconv.ParseFloat(account.TotalWalletBalance, 64)
	m.balance["availableBalance"], _ = strconv.ParseFloat(account.AvailableBalance, 64)
	m.balance["totalUnrealizedProfit"], _ = strconv.ParseFloat(account.TotalUnrealizedProfit, 64)

	// Get positions
	positions, err := m.client.NewGetPositionRiskService().Do(context.Background())
	if err != nil {
		return err
	}

	m.positions = make([]map[string]interface{}, 0)
	for _, pos := range positions {
		posAmt, _ := strconv.ParseFloat(pos.PositionAmt, 64)
		if posAmt == 0 {
			continue
		}

		posMap := make(map[string]interface{})
		posMap["symbol"] = pos.Symbol
		posMap["positionAmt"], _ = strconv.ParseFloat(pos.PositionAmt, 64)
		posMap["entryPrice"], _ = strconv.ParseFloat(pos.EntryPrice, 64)
		posMap["markPrice"], _ = strconv.ParseFloat(pos.MarkPrice, 64)
		posMap["unRealizedProfit"], _ = strconv.ParseFloat(pos.UnRealizedProfit, 64)
		posMap["leverage"], _ = strconv.ParseFloat(pos.Leverage, 64)
		posMap["liquidationPrice"], _ = strconv.ParseFloat(pos.LiquidationPrice, 64)

		if posAmt > 0 {
			posMap["side"] = "long"
		} else {
			posMap["side"] = "short"
		}

		m.positions = append(m.positions, posMap)
	}

	m.lastUserDataUpdate = time.Now()
	m.lastMarkPriceUpdate = time.Now()
	m.lastInitTime = time.Now()
	logger.Infof("✅ Binance WebSocket: Initialized from REST API - balance=%.2f, positions=%d",
		m.balance["totalWalletBalance"], len(m.positions))

	return nil
}
