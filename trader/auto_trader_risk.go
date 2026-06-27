package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"strings"
	"time"
)

// startDrawdownMonitor starts drawdown monitoring
func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(1 * time.Minute) // Check every minute
		defer ticker.Stop()

		logger.Info("📊 Started position drawdown monitoring (check every minute)")

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-at.stopMonitorCh:
				logger.Info("⏹ Stopped position drawdown monitoring")
				return
			}
		}
	}()
}

// checkPositionDrawdown checks position drawdown situation
func (at *AutoTrader) checkPositionDrawdown() {
	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Infof("❌ Drawdown monitoring: failed to get positions: %v", err)
		return
	}

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}

		// Calculate current P&L percentage
		leverage := 10 // Default value
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// Construct unique position identifier (distinguish long/short)
		posKey := symbol + "_" + side

		// Get historical peak profit for this position
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// If no historical peak record, use current P&L as initial value
			peakPnLPct = currentPnLPct
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		} else {
			// Update peak cache
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}

		// Calculate drawdown (magnitude of decline from peak)
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		// Check close position conditions
		shouldClose := false
		closeReason := ""

		if peakPnLPct >= 10.0 { // Meaningful profit established
			if drawdownPct >= 40.0 {
				shouldClose = true
				closeReason = "Drawdown from peak > 40%"
			}
		}

		if shouldClose {
			logger.Infof("🚨 [%s] Drawdown condition triggered: %s %s | Current: %.2f%% | Peak: %.2f%% | Drawdown: %.2f%%",
				closeReason, symbol, side, currentPnLPct, peakPnLPct, drawdownPct)

			// Execute close position
			if err := at.emergencyClosePosition(symbol, side); err != nil {
				logger.Infof("❌ Drawdown close position failed (%s %s): %v", symbol, side, err)
			} else {
				logger.Infof("✅ Drawdown close position succeeded: %s %s", symbol, side)
				// Clear cache for this position after closing
				at.ClearPeakPnLCache(symbol, side)
			}
		} else if currentPnLPct > 5.0 {
			// Record situations close to close position condition (for debugging)
			logger.Infof("📊 Drawdown monitoring: %s %s | Profit: %.2f%% | Peak: %.2f%% | Drawdown: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// emergencyClosePosition emergency close position function
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close long position succeeded, order ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close short position succeeded, order ID: %v", order["orderId"])
	default:
		return fmt.Errorf("unknown position direction: %s", side)
	}

	return nil
}

// GetPeakPnLCache gets peak profit cache
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// Return a copy of the cache
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL updates peak profit cache
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// Update peak (if long, take larger value; if short, currentPnLPct is negative, also compare)
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// First time recording
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache clears peak cache for specified position
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	delete(at.peakPnLCache, posKey)
}

// ============================================================================
// Risk Control Helpers
// ============================================================================

// isBTCETH checks if a symbol is BTC or ETH
func isBTCETH(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	return strings.HasPrefix(symbol, "BTC") || strings.HasPrefix(symbol, "ETH")
}

func (at *AutoTrader) normalizeOpeningLeverage(decision *kernel.Decision) {
	if decision == nil || at.config.StrategyConfig == nil {
		return
	}

	riskControl := at.config.StrategyConfig.RiskControl
	targetLeverage := riskControl.AltcoinMaxLeverage
	if isBTCETH(decision.Symbol) {
		targetLeverage = riskControl.BTCETHMaxLeverage
	}
	if targetLeverage <= 0 {
		return
	}

	if decision.Leverage != targetLeverage {
		logger.Infof("  ⚙️ [RISK CONTROL] Normalizing %s leverage from %dx to configured %dx",
			decision.Symbol, decision.Leverage, targetLeverage)
		decision.Leverage = targetLeverage
	}
}

// enforcePositionValueRatio checks and enforces position value ratio limits (CODE ENFORCED)
// Returns the adjusted position size (capped if necessary) and whether the position was capped
// positionSizeUSD: the original position size in USD
// equity: the account equity
// symbol: the trading symbol
func (at *AutoTrader) enforcePositionValueRatio(positionSizeUSD float64, equity float64, symbol string) (float64, bool) {
	if at.config.StrategyConfig == nil {
		return positionSizeUSD, false
	}

	riskControl := at.config.StrategyConfig.RiskControl

	// Get the appropriate position value ratio limit
	var maxPositionValueRatio float64
	if isBTCETH(symbol) {
		maxPositionValueRatio = riskControl.BTCETHMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 5.0 // Default: 5x for BTC/ETH
		}
	} else {
		maxPositionValueRatio = riskControl.AltcoinMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 1.0 // Default: 1x for altcoins
		}
	}

	// Calculate max allowed position value = equity × ratio
	maxPositionValue := equity * maxPositionValueRatio

	// Check if position size exceeds limit
	if positionSizeUSD > maxPositionValue {
		logger.Infof("  ⚠️ [RISK CONTROL] Position %.2f USDT exceeds limit (equity %.2f × %.1fx = %.2f USDT max for %s), capping",
			positionSizeUSD, equity, maxPositionValueRatio, maxPositionValue, symbol)
		return maxPositionValue, true
	}

	return positionSizeUSD, false
}

// enforceMinPositionSize checks minimum position size (CODE ENFORCED)
func (at *AutoTrader) enforceMinPositionSize(positionSizeUSD, equity float64) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
	if minSize <= 0 {
		minSize = 5
	}

	if positionSizeUSD < minSize {
		return fmt.Errorf("❌ [RISK CONTROL] Position %.2f USDT below minimum (%.2f USDT)", positionSizeUSD, minSize)
	}
	return nil
}

// validateOpeningRisk validates model output against the real current market price.
func (at *AutoTrader) validateOpeningRisk(decision *kernel.Decision, currentPrice float64) error {
	if decision == nil || at.config.StrategyConfig == nil {
		return nil
	}
	if currentPrice <= 0 {
		return fmt.Errorf("❌ [RISK CONTROL] Invalid current price for %s", decision.Symbol)
	}

	riskControl := at.config.StrategyConfig.RiskControl
	minConfidence := riskControl.MinConfidence
	if minConfidence <= 0 {
		minConfidence = 70
	}
	if decision.Confidence < minConfidence {
		return fmt.Errorf("❌ [RISK CONTROL] Confidence %d below minimum %d", decision.Confidence, minConfidence)
	}

	var riskDistance, rewardDistance float64
	switch decision.Action {
	case "open_long":
		if decision.StopLoss >= currentPrice || decision.TakeProfit <= currentPrice {
			return fmt.Errorf("❌ [RISK CONTROL] LONG requires stop_loss < current price < take_profit")
		}
		riskDistance = currentPrice - decision.StopLoss
		rewardDistance = decision.TakeProfit - currentPrice
	case "open_short":
		if decision.StopLoss <= currentPrice || decision.TakeProfit >= currentPrice {
			return fmt.Errorf("❌ [RISK CONTROL] SHORT requires take_profit < current price < stop_loss")
		}
		riskDistance = decision.StopLoss - currentPrice
		rewardDistance = currentPrice - decision.TakeProfit
	default:
		return nil
	}

	minRR := riskControl.MinRiskRewardRatio
	if minRR <= 0 {
		minRR = 2
	}
	rr := rewardDistance / riskDistance
	const rrComparisonEpsilon = 1e-6
	if rr+rrComparisonEpsilon < minRR {
		return fmt.Errorf("❌ [RISK CONTROL] Real risk/reward %.4f:1 below minimum %.4f:1", rr, minRR)
	}

	decision.RiskUSD = decision.PositionSizeUSD * (riskDistance / currentPrice)
	return nil
}

// normalizeExitLevels applies exchange-safe distances before an order is opened.
func (at *AutoTrader) normalizeExitLevels(decision *kernel.Decision, currentPrice float64) {
	if decision == nil || currentPrice <= 0 {
		return
	}

	minRR := 2.0
	if at.config.StrategyConfig != nil && at.config.StrategyConfig.RiskControl.MinRiskRewardRatio > 0 {
		minRR = at.config.StrategyConfig.RiskControl.MinRiskRewardRatio
	}

	const minStopDistance = 0.015
	maxTargetDistance := 0.10
	if isBTCETH(decision.Symbol) {
		maxTargetDistance = 0.15
	}

	switch decision.Action {
	case "open_long":
		maxStop := currentPrice * (1 - minStopDistance)
		if decision.StopLoss > maxStop {
			decision.StopLoss = maxStop
		}
		requiredTarget := currentPrice + (currentPrice-decision.StopLoss)*minRR
		if decision.TakeProfit < requiredTarget {
			decision.TakeProfit = requiredTarget
		}
		maxTarget := currentPrice * (1 + maxTargetDistance)
		if decision.TakeProfit > maxTarget {
			decision.TakeProfit = maxTarget
		}
	case "open_short":
		minStop := currentPrice * (1 + minStopDistance)
		if decision.StopLoss < minStop {
			decision.StopLoss = minStop
		}
		requiredTarget := currentPrice - (decision.StopLoss-currentPrice)*minRR
		if decision.TakeProfit > requiredTarget {
			decision.TakeProfit = requiredTarget
		}
		minTarget := currentPrice * (1 - maxTargetDistance)
		if decision.TakeProfit < minTarget {
			decision.TakeProfit = minTarget
		}
	}
}

// enforceMaxMarginUsage caps a new position using projected account-wide margin.
func (at *AutoTrader) enforceMaxMarginUsage(positionSizeUSD float64, leverage int, equity, availableBalance float64) (float64, bool, error) {
	if at.config.StrategyConfig == nil || equity <= 0 || leverage <= 0 {
		return positionSizeUSD, false, nil
	}

	maxUsage := at.config.StrategyConfig.RiskControl.MaxMarginUsage
	if maxUsage <= 0 || maxUsage > 1 {
		maxUsage = 0.65
	}

	currentUsedMargin := math.Max(equity-availableBalance, 0)
	remainingMargin := equity*maxUsage - currentUsedMargin
	if remainingMargin <= 0 {
		return 0, false, fmt.Errorf("❌ [RISK CONTROL] Margin usage already reached configured %.0f%% limit", maxUsage*100)
	}

	// Reserve 2% for fees, slippage and mark-price movement.
	maxPositionSize := remainingMargin * float64(leverage) * 0.98
	if positionSizeUSD > maxPositionSize {
		logger.Infof("  ⚠️ [RISK CONTROL] Position %.2f exceeds projected margin limit, capping to %.2f",
			positionSizeUSD, maxPositionSize)
		return maxPositionSize, true, nil
	}

	return positionSizeUSD, false, nil
}

// enforceRiskBudget caps notional so loss at stop cannot exceed the equity risk budget.
func (at *AutoTrader) enforceRiskBudget(decision *kernel.Decision, equity, currentPrice float64) (float64, bool, error) {
	if decision == nil {
		return 0, false, fmt.Errorf("❌ [RISK CONTROL] Missing opening decision")
	}
	if at.config.StrategyConfig == nil || equity <= 0 || currentPrice <= 0 {
		return decision.PositionSizeUSD, false, nil
	}

	maxRiskPct := at.config.StrategyConfig.RiskControl.MaxRiskPerTradePct
	if maxRiskPct <= 0 || maxRiskPct > 0.05 {
		maxRiskPct = 0.01
	}

	stopDistancePct := math.Abs(currentPrice-decision.StopLoss) / currentPrice
	if stopDistancePct <= 0 {
		return 0, false, fmt.Errorf("❌ [RISK CONTROL] Stop-loss distance must be positive")
	}

	maxRiskUSD := equity * maxRiskPct
	maxPositionSize := maxRiskUSD / stopDistancePct
	if maxPositionSize <= 0 {
		return 0, false, fmt.Errorf("❌ [RISK CONTROL] Invalid risk-sized position")
	}

	adjusted := decision.PositionSizeUSD
	capped := false
	if adjusted > maxPositionSize {
		adjusted = maxPositionSize
		capped = true
		logger.Infof("  ⚠️ [RISK CONTROL] Position capped to %.2f so stop risk stays within %.2f%% equity",
			adjusted, maxRiskPct*100)
	}
	decision.RiskUSD = adjusted * stopDistancePct
	return adjusted, capped, nil
}

// enforceMaxPositions checks maximum positions count (CODE ENFORCED)
func (at *AutoTrader) enforceMaxPositions(currentPositionCount int) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	maxPositions := at.config.StrategyConfig.RiskControl.MaxPositions
	if maxPositions <= 0 {
		maxPositions = 3 // Default: 3 positions
	}

	if currentPositionCount >= maxPositions {
		return fmt.Errorf("❌ [RISK CONTROL] Already at max positions (%d/%d)", currentPositionCount, maxPositions)
	}
	return nil
}

// getSideFromAction converts order action to side (BUY/SELL)
// getSideFromAction converts order action to side (BUY/SELL)
func getSideFromAction(action string) string {
	switch action {
	case "open_long", "close_short":
		return "BUY"
	case "open_short", "close_long":
		return "SELL"
	default:
		return "BUY"
	}
}

// enforceMinHoldTime blocks churn from opening and then immediately closing
// because the model changed its mind. Protective exits may still close early.
func (at *AutoTrader) enforceMinHoldTime(symbol, side string, reasoning string) error {
	// 1. Check if strategy config exists
	if at.config.StrategyConfig == nil {
		return nil
	}

	// 2. Emergency/SL/TP bypass: if reasoning indicates a hard protective exit,
	// allow it. Do not include broad terms like "risk"; those are too easy for
	// discretionary churn to match.
	reasoningUpper := strings.ToUpper(reasoning)
	bypassKeywords := []string{
		"EMERGENCY",
		"STOP LOSS",
		"STOP-LOSS",
		"HARD STOP",
		"STOP HIT",
		"TAKE PROFIT",
		"TAKE-PROFIT",
		"TP HIT",
		"TARGET HIT",
		"SL/TP",
		"LIQUIDATION",
		"MARGIN CALL",
		"STRUCTURE BREAK",
		"INVALIDATION",
	}
	for _, kw := range bypassKeywords {
		if strings.Contains(reasoningUpper, kw) {
			logger.Infof("  ⚡ [RISK CONTROL] %s close allowed: %s", kw, symbol)
			return nil
		}
	}

	// 3. Get entry time
	posKey := symbol + "_" + strings.ToLower(side)
	var entryTimeMs int64

	// Try local cache first
	at.peakPnLCacheMutex.RLock() // Use this mutex or any existing one if appropriate, or just access if thread-safe
	entryTimeMs = at.positionFirstSeenTime[posKey]
	at.peakPnLCacheMutex.RUnlock()

	// First try to get from local database (more accurate for quantity)
	if at.store != nil {
		if dbPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, strings.ToUpper(side)); err == nil && dbPos != nil {
			entryTimeMs = dbPos.EntryTime
		}
	}

	// 4. Validate hold duration
	if entryTimeMs > 0 {
		holdTimeMinutes := time.Since(time.UnixMilli(entryTimeMs)).Minutes()
		minHoldTimeMinutes := 15.0

		if holdTimeMinutes < minHoldTimeMinutes {
			return fmt.Errorf("❌ [HOLD TIME] %s %s held only %.0f min (< %.0f min); close blocked unless stop-loss, take-profit, liquidation, structure break, or invalidation is explicit",
				symbol, side, holdTimeMinutes, minHoldTimeMinutes)
		}
	}

	return nil
}

// checkRecentCloseCooldown checks if a position on this symbol was recently closed
// Prevents re-entering a coin within MinCooldownMinutes after closing a position on it
func (at *AutoTrader) checkRecentCloseCooldown(symbol string) error {
	if at.store == nil {
		return nil
	}

	// Get recent 10 closed trades
	recentTrades, err := at.store.Position().GetRecentTrades(at.id, 10)
	if err != nil {
		// Don't block trading if we can't check — just log
		logger.Infof("  ⚠️ [COOLDOWN] Could not check recent trades: %v", err)
		return nil
	}

	// Normalize symbol for comparison
	normalizedSymbol := strings.ToUpper(strings.TrimSpace(symbol))

	for _, trade := range recentTrades {
		tradeSymbol := strings.ToUpper(strings.TrimSpace(trade.Symbol))
		if tradeSymbol == normalizedSymbol && trade.ExitTime > 0 {
			exitTime := unixTimeFromSecondsOrMillis(trade.ExitTime)
			elapsed := time.Since(exitTime)
			cooldownDuration := time.Duration(MinCooldownMinutes) * time.Minute

			if elapsed < cooldownDuration {
				remaining := cooldownDuration - elapsed
				if remaining <= 5*time.Second {
					logger.Infof("  ℹ️ [COOLDOWN] %s cooldown has %.0fs remaining; allowing entry within grace window",
						symbol, remaining.Seconds())
					return nil
				}
				return fmt.Errorf("❌ [COOLDOWN] %s was closed %s ago, wait %s more (min cooldown: %d min)",
					symbol, formatCooldownDuration(elapsed), formatCooldownDuration(remaining), MinCooldownMinutes)
			}
		}
	}

	return nil
}

func unixTimeFromSecondsOrMillis(ts int64) time.Time {
	if ts > 1_000_000_000_000 {
		return time.UnixMilli(ts)
	}
	return time.Unix(ts, 0)
}

func formatCooldownDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int64(math.Ceil(d.Seconds()))
	if totalSeconds < 60 {
		return fmt.Sprintf("%ds", totalSeconds)
	}
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm %02ds", minutes, seconds)
}
