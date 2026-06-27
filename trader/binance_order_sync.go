package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strings"
	"sync"
	"time"
)

// syncState stores the last sync time (Unix ms) for incremental sync
var (
	binanceSyncState      = make(map[string]int64) // exchangeID -> lastSyncTimeMs (Unix ms)
	binanceSyncStateMutex sync.RWMutex
)

// SyncOrdersFromBinance syncs Binance Futures trade history to local database
// Uses COMMISSION detection + fromId for efficient incremental sync
// Also creates/updates position records to ensure orders/fills/positions data consistency
func (t *FuturesTrader) SyncOrdersFromBinance(traderID string, exchangeID string, exchangeType string, st *store.Store, closeCycle int) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	orderStore := st.Order()

	// Get last sync time (Unix ms) - first try memory, then database, then default
	binanceSyncStateMutex.RLock()
	lastSyncTimeMs, exists := binanceSyncState[exchangeID]
	binanceSyncStateMutex.RUnlock()

	nowMs := time.Now().UTC().UnixMilli()
	if !exists {
		// Try to get last fill time from database (persist across restarts)
		lastFillTimeMs, err := orderStore.GetLastFillTimeByExchange(exchangeID)
		if err == nil && lastFillTimeMs > 0 {
			// If recovered time is in the future, it's clearly wrong - use default
			if lastFillTimeMs > nowMs {
				logger.Infof("⚠️ DB sync time %d is in the future (now: %d), using default",
					lastFillTimeMs, nowMs)
				lastSyncTimeMs = nowMs - 24*60*60*1000 // 24 hours ago
			} else {
				// Add 1ms buffer to avoid re-fetching the same fill exactly
				lastSyncTimeMs = lastFillTimeMs + 1
				logger.Infof("📅 Recovered last sync time from DB: %s (UTC)",
					time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"))
			}
		} else {
			// First sync: go back 24 hours
			lastSyncTimeMs = nowMs - 24*60*60*1000
			logger.Infof("📅 First sync, starting from 24 hours ago: %s (UTC)",
				time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"))
		}
	}

	logger.Infof("🔄 Syncing Binance trades from: %s (UTC) [ms: %d, now: %d]",
		time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"), lastSyncTimeMs, nowMs)

	// Step 1: Get max trade IDs from local DB for incremental sync
	maxTradeIDs, err := orderStore.GetMaxTradeIDsByExchange(exchangeID)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get max trade IDs: %v, will use time-based query", err)
		maxTradeIDs = make(map[string]int64)
	}

	// Step 2: Detect symbols to sync using multiple methods
	// COMMISSION detection may miss trades (VIP users, BNB discount)
	symbolMap := make(map[string]bool)
	lastSyncTime := time.UnixMilli(lastSyncTimeMs)

	// LOOK-BACK: Use a 2-hour look-back for symbol detection to bridge Binance API delays
	// This ensures that even if a trade record is delayed, it will eventually be caught
	lookBackTime := lastSyncTime.Add(-2 * time.Hour)

	// Method 1: COMMISSION income detection (using look-back)
	commissionSymbols, err := t.GetCommissionSymbols(lookBackTime)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get commission symbols: %v", err)
	} else {
		logger.Infof("  📋 COMMISSION symbols found: %d - %v", len(commissionSymbols), commissionSymbols)
		for _, s := range commissionSymbols {
			symbolMap[s] = true
		}
	}

	// Method 2: Always include active positions (catches trades that COMMISSION missed)
	positionSymbols := t.getPositionSymbols()
	logger.Infof("  📋 Position symbols found: %d - %v", len(positionSymbols), positionSymbols)
	for _, s := range positionSymbols {
		symbolMap[s] = true
	}

	// Method 3: Include symbols from recent fills in DB (in case some were partially synced)
	recentSymbols, _ := orderStore.GetRecentFillSymbolsByExchange(exchangeID, lastSyncTimeMs)
	logger.Infof("  📋 Recent fill symbols found: %d - %v", len(recentSymbols), recentSymbols)
	for _, s := range recentSymbols {
		symbolMap[s] = true
	}

	// Method 4: ALWAYS query REALIZED_PNL income to find symbols with closed trades
	// This catches trades that COMMISSION missed (VIP users, BNB fee discount)
	pnlSymbols, err := t.GetPnLSymbols(lookBackTime) // Use look-back
	if err != nil {
		logger.Infof("  ⚠️ Failed to get PnL symbols: %v", err)
	} else {
		logger.Infof("  📋 REALIZED_PNL symbols found: %d - %v", len(pnlSymbols), pnlSymbols)
		for _, s := range pnlSymbols {
			symbolMap[s] = true
		}
	}

	// Method 5: Always include symbols for positions currently 'OPEN' in local database
	// This acts as a reconciliation mechanism for any previously missed trades
	dbOpenPositions, err := st.Position().GetOpenPositions(traderID)
	if err == nil && len(dbOpenPositions) > 0 {
		var openSymbols []string
		for _, p := range dbOpenPositions {
			symbolMap[p.Symbol] = true
			openSymbols = append(openSymbols, p.Symbol)
		}
		logger.Infof("  📋 DB OPEN positions symbols found: %d - %v", len(openSymbols), openSymbols)
	}

	var changedSymbols []string
	for s := range symbolMap {
		changedSymbols = append(changedSymbols, s)
	}

	if len(changedSymbols) == 0 {
		logger.Infof("📭 No symbols with new trades to sync")
		// DON'T update lastSyncTime to current time here!
		// Keep using the last actual trade time from DB to avoid creating gaps
		// The lastSyncTimeMs from DB already has 1ms buffer added
		return nil
	}

	logger.Infof("📊 Found %d symbols with new trades: %v", len(changedSymbols), changedSymbols)

	// Step 3: Query trades for changed symbols using fromId (incremental) or time-based (new symbols)
	var allTrades []TradeRecord
	var failedSymbols []string
	apiCalls := 0
	for _, symbol := range changedSymbols {
		var trades []TradeRecord
		var queryErr error

		if lastID, ok := maxTradeIDs[symbol]; ok && lastID > 0 {
			// Incremental sync: query from last known trade ID
			trades, queryErr = t.GetTradesForSymbolFromID(symbol, lastID+1, 500)
		} else {
			// New symbol or first sync: query by time
			trades, queryErr = t.GetTradesForSymbol(symbol, lastSyncTime, 500)
		}
		apiCalls++

		if queryErr != nil {
			logger.Infof("  ⚠️ Failed to get trades for %s: %v", symbol, queryErr)
			failedSymbols = append(failedSymbols, symbol)
			continue
		}
		allTrades = append(allTrades, trades...)
	}

	logger.Infof("📥 Received %d trades from Binance (%d API calls)", len(allTrades), apiCalls)

	if len(allTrades) == 0 {
		// No trades returned, but symbols were detected - might be false positive from COMMISSION/PnL detection
		// Don't update lastSyncTime, keep using DB value
		if len(failedSymbols) > 0 {
			logger.Infof("  ⚠️ %d symbols failed: %v", len(failedSymbols), failedSymbols)
		}
		return nil
	}

	// Sort trades by time ASC (oldest first) for proper position building
	sort.Slice(allTrades, func(i, j int) bool {
		return allTrades[i].Time.UnixMilli() < allTrades[j].Time.UnixMilli()
	})

	// Process trades one by one
	positionStore := st.Position()
	posBuilder := store.NewPositionBuilder(positionStore)
	syncedCount := 0

	skippedCount := 0
	for _, trade := range allTrades {
		// Check if trade already exists
		existing, err := orderStore.GetOrderByExchangeID(exchangeID, trade.TradeID)
		if err == nil && existing != nil {
			skippedCount++
			continue // Trade already exists, skip
		}

		// Normalize symbol
		symbol := market.Normalize(trade.Symbol)

		// Determine order action based on side and position side
		orderAction := t.determineOrderAction(trade.Side, trade.PositionSide, trade.RealizedPnL)

		// Determine position side for position builder
		positionSide := trade.PositionSide
		if positionSide == "" || positionSide == "BOTH" {
			// Infer from order action
			if strings.Contains(orderAction, "long") {
				positionSide = "LONG"
			} else {
				positionSide = "SHORT"
			}
		}

		// Normalize side
		side := strings.ToUpper(trade.Side)

		// Create order record - use Unix milliseconds UTC
		tradeTimeMs := trade.Time.UTC().UnixMilli()
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			ExchangeOrderID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            "MARKET",
			OrderAction:     orderAction,
			Quantity:        trade.Quantity,
			Price:           trade.Price,
			Status:          "FILLED",
			FilledQuantity:  trade.Quantity,
			AvgFillPrice:    trade.Price,
			Commission:      trade.Fee,
			FilledAt:        tradeTimeMs,
			CreatedAt:       tradeTimeMs,
			UpdatedAt:       tradeTimeMs,
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync trade %s: %v", trade.TradeID, err)
			continue
		}

		// Create fill record - use Unix milliseconds UTC
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			OrderID:         orderRecord.ID,
			ExchangeOrderID: trade.TradeID,
			ExchangeTradeID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			Price:           trade.Price,
			Quantity:        trade.Quantity,
			QuoteQuantity:   trade.Price * trade.Quantity,
			Commission:      trade.Fee,
			CommissionAsset: "USDT",
			RealizedPnL:     trade.RealizedPnL,
			IsMaker:         false,
			CreatedAt:       tradeTimeMs,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, orderAction,
			trade.Quantity, trade.Price, trade.Fee, trade.RealizedPnL,
			tradeTimeMs, trade.TradeID,
			closeCycle,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
		} else {
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.TradeID, orderAction, trade.Quantity)
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s time=%s(UTC)",
			trade.TradeID, symbol, side, trade.Quantity, trade.Price, trade.RealizedPnL, trade.Fee, orderAction,
			trade.Time.UTC().Format("01-02 15:04:05"))
	}

	// Update lastSyncTime to the LATEST trade time (not current time!)
	// This ensures next sync starts from where we left off, not from "now"
	// allTrades is already sorted by time ASC, so last element is the latest
	if len(allTrades) > 0 && len(failedSymbols) == 0 {
		latestTradeTimeMs := allTrades[len(allTrades)-1].Time.UTC().UnixMilli()
		binanceSyncStateMutex.Lock()
		binanceSyncState[exchangeID] = latestTradeTimeMs
		binanceSyncStateMutex.Unlock()
		logger.Infof("📅 Updated lastSyncTime to latest trade: %s (UTC)",
			time.UnixMilli(latestTradeTimeMs).UTC().Format("2006-01-02 15:04:05"))
	} else if len(failedSymbols) > 0 {
		logger.Infof("  ⚠️ %d symbols failed, not updating lastSyncTime to retry next time: %v", len(failedSymbols), failedSymbols)
	}

	logger.Infof("✅ Binance order sync completed: %d new trades synced, %d skipped (already exist)", syncedCount, skippedCount)
	return nil
}

// getPositionSymbols returns list of symbols that have active positions
// Used as fallback when COMMISSION detection fails
func (t *FuturesTrader) getPositionSymbols() []string {
	positions, err := t.GetPositions()
	if err != nil {
		return nil
	}

	var symbols []string
	for _, pos := range positions {
		if symbol, ok := pos["symbol"].(string); ok && symbol != "" {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

// determineOrderAction determines the order action based on trade data
// In Hedge Mode (Dual-Side), positionSide tells us exactly which side is being affected.
func (t *FuturesTrader) determineOrderAction(side, positionSide string, realizedPnL float64) string {
	side = strings.ToUpper(side)
	positionSide = strings.ToUpper(positionSide)

	// In HedgeMode (Dual-Side position mode):
	// LONG + BUY = open_long
	// LONG + SELL = close_long
	// SHORT + SELL = open_short
	// SHORT + BUY = close_short
	if positionSide == "LONG" {
		if side == "BUY" {
			return "open_long"
		}
		return "close_long"
	} else if positionSide == "SHORT" {
		if side == "SELL" {
			return "open_short"
		}
		return "close_short"
	}

	// For One-Way Mode (positionSide is "BOTH" or empty), we must use realizedPnL to distinguish
	isClose := realizedPnL != 0
	if side == "BUY" {
		if isClose {
			return "close_short"
		}
		return "open_long"
	}
	// side == "SELL"
	if isClose {
		return "close_long"
	}
	return "open_short"
}

// StartOrderSync starts background order sync task for Binance
func (t *FuturesTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	// Run first sync immediately (use 0 as cycle for initial sync)
	go func() {
		logger.Infof("🔄 Running initial Binance order sync...")
		if err := t.SyncOrdersFromBinance(traderID, exchangeID, exchangeType, st, 0); err != nil {
			logger.Errorf("  ❌ Sync failed: %v", err)
		} else {
			logger.Infof("✅ Initial sync completed successfully.")
		}
	}()

	// Then run periodically
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromBinance(traderID, exchangeID, exchangeType, st, 0); err != nil {
				logger.Infof("⚠️  Binance order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Binance order sync started (interval: %v)", interval)
}

// SyncSymbolOrders syncs trades for a specific symbol (e.g., after detecting external closure)
func (t *FuturesTrader) SyncSymbolOrders(traderID string, exchangeID string, exchangeType string, st *store.Store, symbol string, closeCycle int) error {
	logger.Infof("🔍 Syncing specific symbol trades for %s (cycle: %d)", symbol, closeCycle)

	// Determine start time (last 24 hours for safety if no max ID found)
	startTime := time.Now().Add(-24 * time.Hour)
	orderStore := st.Order()

	// Try to get last trade ID for this symbol
	maxTradeIDs, err := orderStore.GetMaxTradeIDsByExchange(exchangeID)
	lastID := int64(0)
	if err == nil {
		lastID = maxTradeIDs[symbol]
	}

	var trades []TradeRecord
	if lastID > 0 {
		trades, err = t.GetTradesForSymbolFromID(symbol, lastID+1, 500)
	} else {
		trades, err = t.GetTradesForSymbol(symbol, startTime, 500)
	}

	if err != nil {
		return fmt.Errorf("failed to fetch trades for %s: %w", symbol, err)
	}

	if len(trades) == 0 {
		logger.Infof("  ⚪ No new trades found for %s", symbol)
		return nil
	}

	// Sort and process trades
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].Time.UnixMilli() < trades[j].Time.UnixMilli()
	})

	posBuilder := store.NewPositionBuilder(st.Position())
	for _, trade := range trades {
		// Existing trade check
		existing, _ := orderStore.GetOrderByExchangeID(exchangeID, trade.TradeID)
		if existing != nil {
			continue
		}

		normSymbol := market.Normalize(trade.Symbol)
		orderAction := t.determineOrderAction(trade.Side, trade.PositionSide, trade.RealizedPnL)
		tradeTimeMs := trade.Time.UTC().UnixMilli()

		// Position Side inference
		posSide := trade.PositionSide
		if posSide == "" || posSide == "BOTH" {
			if strings.Contains(orderAction, "long") {
				posSide = "LONG"
			} else {
				posSide = "SHORT"
			}
		}

		// Create record
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			ExchangeOrderID: trade.TradeID,
			Symbol:          normSymbol,
			Side:            strings.ToUpper(trade.Side),
			PositionSide:    posSide,
			Type:            "MARKET",
			OrderAction:     orderAction,
			Quantity:        trade.Quantity,
			Price:           trade.Price,
			Status:          "FILLED",
			FilledQuantity:  trade.Quantity,
			AvgFillPrice:    trade.Price,
			Commission:      trade.Fee,
			FilledAt:        tradeTimeMs,
			CreatedAt:       tradeTimeMs,
			UpdatedAt:       tradeTimeMs,
		}

		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Error syncing specific order %s: %v", trade.TradeID, err)
			continue
		}

		// Fill record
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			OrderID:         orderRecord.ID,
			ExchangeOrderID: trade.TradeID,
			ExchangeTradeID: trade.TradeID,
			Symbol:          normSymbol,
			Side:            strings.ToUpper(trade.Side),
			Price:           trade.Price,
			Quantity:        trade.Quantity,
			QuoteQuantity:   trade.Price * trade.Quantity,
			Commission:      trade.Fee,
			CommissionAsset: "USDT",
			RealizedPnL:     trade.RealizedPnL,
			CreatedAt:       tradeTimeMs,
		}
		_ = orderStore.CreateFill(fillRecord)

		// Update position
		_ = posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			normSymbol, posSide, orderAction,
			trade.Quantity, trade.Price, trade.Fee, trade.RealizedPnL,
			tradeTimeMs, trade.TradeID,
			closeCycle,
		)
	}

	logger.Infof("  ✅ Successfully synced %d trades for %s", len(trades), symbol)
	return nil
}
