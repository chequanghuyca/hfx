package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"time"
)

// Entry quality thresholds - code-enforced filters to prevent FOMO entries
const (
	// MaxBOLLPositionLong blocks LONG entries only near obvious Bollinger exhaustion.
	MaxBOLLPositionLong = 92.0

	// MinBOLLPositionShort blocks SHORT entries only near obvious Bollinger exhaustion.
	MinBOLLPositionShort = 8.0

	// MaxRSI7ForLong marks LONG entries as short-term overbought caution.
	MaxRSI7ForLong = 72.0

	// ExtremeMaxRSI7ForLong blocks LONG entries only at clear exhaustion.
	ExtremeMaxRSI7ForLong = 82.0

	// MinRSI7ForShort marks SHORT entries as short-term oversold caution.
	MinRSI7ForShort = 28.0

	// ExtremeMinRSI7ForShort blocks SHORT entries only at clear exhaustion.
	ExtremeMinRSI7ForShort = 18.0

	// MaxRSI14ForLong marks LONG entries as overbought caution on 1h.
	MaxRSI14ForLong = 72.0

	// ExtremeMaxRSI14ForLong blocks LONG entries only when 1h is strongly overbought.
	ExtremeMaxRSI14ForLong = 78.0

	// MinRSI14ForShort marks SHORT entries as oversold caution on 1h.
	MinRSI14ForShort = 28.0

	// ExtremeMinRSI14ForShort blocks SHORT entries only when 1h is strongly oversold.
	ExtremeMinRSI14ForShort = 22.0

	// MaxPriceChange2h blocks entries on coins that already moved too much (trend exhaustion)
	MaxPriceChange2h = 12.0 // ±12% in 2 hours → skip

	// MinCooldownMinutes minimum cooldown after closing a position on same coin
	MinCooldownMinutes = 5
)

// EntryQualityResult contains the result of entry quality validation
type EntryQualityResult struct {
	IsQualityEntry bool    // Whether the entry passes quality checks
	Reason         string  // Reason for blocking (if blocked)
	Warning        string  // Caution note when the entry is allowed but extended
	BOLLPosition   float64 // Position within Bollinger Bands (0-100%)
	RSI7           float64 // Current RSI7 value
	RSI14          float64 // Current RSI14 value (1h)
	PriceChange2h  float64 // Price change percentage in last 2 hours
}

// CheckEntryQuality validates if the current price is a quality entry point
// This prevents FOMO entries at peaks/bottoms using BOLL, RSI, and price-change filters
func CheckEntryQuality(symbol string, side string, mktData *market.Data) (*EntryQualityResult, error) {
	result := &EntryQualityResult{IsQualityEntry: true}

	if mktData == nil {
		result.IsQualityEntry = false
		result.Reason = "No market data available"
		return result, nil
	}

	currentPrice := mktData.CurrentPrice

	// 1. Check Bollinger Band position (if available from any timeframe)
	if len(mktData.TimeframeData) > 0 {
		// Try to get Bollinger data from 15m or 1h timeframe
		preferredTimeframes := []string{"15m", "1h", "5m", "30m"}
		for _, tf := range preferredTimeframes {
			if tfData, ok := mktData.TimeframeData[tf]; ok {
				if len(tfData.BOLLUpper) > 0 && len(tfData.BOLLLower) > 0 {
					latestUpper := tfData.BOLLUpper[len(tfData.BOLLUpper)-1]
					latestLower := tfData.BOLLLower[len(tfData.BOLLLower)-1]

					if latestUpper > latestLower {
						result.BOLLPosition = (currentPrice - latestLower) / (latestUpper - latestLower) * 100

						if side == "long" && result.BOLLPosition > MaxBOLLPositionLong {
							result.IsQualityEntry = false
							result.Reason = fmt.Sprintf("Price at %.1f%% Bollinger position (%s) - too high for LONG (max: %.0f%%)",
								result.BOLLPosition, tf, MaxBOLLPositionLong)
							return result, nil
						}

						if side == "short" && result.BOLLPosition < MinBOLLPositionShort {
							result.IsQualityEntry = false
							result.Reason = fmt.Sprintf("Price at %.1f%% Bollinger position (%s) - too low for SHORT (min: %.0f%%)",
								result.BOLLPosition, tf, MinBOLLPositionShort)
							return result, nil
						}
					}
					break // Use first available timeframe with BOLL data
				}
			}
		}
	}

	// 2. Check RSI7 (short-term overbought/oversold)
	if mktData.CurrentRSI7 > 0 {
		result.RSI7 = mktData.CurrentRSI7

		if side == "long" && result.RSI7 > ExtremeMaxRSI7ForLong {
			result.IsQualityEntry = false
			result.Reason = fmt.Sprintf("RSI7 at %.1f - extremely overbought, too extended for new LONG (hard max: %.0f)",
				result.RSI7, ExtremeMaxRSI7ForLong)
			return result, nil
		}

		if side == "long" && result.RSI7 > MaxRSI7ForLong {
			result.Warning = fmt.Sprintf("RSI7 %.1f is overbought; LONG allowed only if AI found a continuation/pullback trigger with valid SL/RR",
				result.RSI7)
		}

		if side == "short" && result.RSI7 < ExtremeMinRSI7ForShort {
			result.IsQualityEntry = false
			result.Reason = fmt.Sprintf("RSI7 at %.1f - extremely oversold, too extended for new SHORT (hard min: %.0f)",
				result.RSI7, ExtremeMinRSI7ForShort)
			return result, nil
		}

		if side == "short" && result.RSI7 < MinRSI7ForShort {
			result.Warning = fmt.Sprintf("RSI7 %.1f is oversold; SHORT allowed only if AI found a continuation/breakdown trigger with valid SL/RR",
				result.RSI7)
		}
	}

	// 3. Check RSI14 on 1h timeframe (trend-exhaustion filter)
	if len(mktData.TimeframeData) > 0 {
		if tfData, ok := mktData.TimeframeData["1h"]; ok {
			if len(tfData.RSI14Values) > 0 {
				rsi14 := tfData.RSI14Values[len(tfData.RSI14Values)-1]
				result.RSI14 = rsi14

				if side == "long" && rsi14 > ExtremeMaxRSI14ForLong {
					result.IsQualityEntry = false
					result.Reason = fmt.Sprintf("RSI14(1h) at %.1f - strongly overbought on higher timeframe, too extended for new LONG (hard max: %.0f)",
						rsi14, ExtremeMaxRSI14ForLong)
					return result, nil
				}

				if side == "long" && rsi14 > MaxRSI14ForLong {
					result.Warning = appendEntryQualityWarning(result.Warning,
						fmt.Sprintf("RSI14(1h) %.1f is overbought; require clean continuation and tight invalidation", rsi14))
				}

				if side == "short" && rsi14 < ExtremeMinRSI14ForShort {
					result.IsQualityEntry = false
					result.Reason = fmt.Sprintf("RSI14(1h) at %.1f - strongly oversold on higher timeframe, too extended for new SHORT (hard min: %.0f)",
						rsi14, ExtremeMinRSI14ForShort)
					return result, nil
				}

				if side == "short" && rsi14 < MinRSI14ForShort {
					result.Warning = appendEntryQualityWarning(result.Warning,
						fmt.Sprintf("RSI14(1h) %.1f is oversold; require clean continuation and tight invalidation", rsi14))
				}
			}
		}
	}

	// 4. Check price change in last 2 hours (trend exhaustion / FOMO prevention)
	priceChange2h := calculatePriceChange2h(mktData)
	result.PriceChange2h = priceChange2h

	if side == "long" && priceChange2h > MaxPriceChange2h {
		result.IsQualityEntry = false
		result.Reason = fmt.Sprintf("Price pumped +%.1f%% in 2h - too late for LONG entry, wait for pullback (max: ±%.0f%%)",
			priceChange2h, MaxPriceChange2h)
		return result, nil
	}

	if side == "short" && priceChange2h < -MaxPriceChange2h {
		result.IsQualityEntry = false
		result.Reason = fmt.Sprintf("Price dumped %.1f%% in 2h - too late for SHORT entry, wait for bounce (max: ±%.0f%%)",
			priceChange2h, MaxPriceChange2h)
		return result, nil
	}

	return result, nil
}

func appendEntryQualityWarning(existing string, next string) string {
	if existing == "" {
		return next
	}
	return existing + "; " + next
}

// calculatePriceChange2h calculates price change percentage in the last 2 hours from kline data
func calculatePriceChange2h(mktData *market.Data) float64 {
	if len(mktData.TimeframeData) == 0 {
		return 0
	}

	// Try 15m klines first (8 candles = 2 hours), then 1h (2 candles)
	timeframeConfigs := []struct {
		tf           string
		candlesFor2h int
	}{
		{"15m", 8},
		{"1h", 2},
		{"5m", 24},
		{"30m", 4},
	}

	for _, cfg := range timeframeConfigs {
		if tfData, ok := mktData.TimeframeData[cfg.tf]; ok && len(tfData.Klines) >= cfg.candlesFor2h {
			// Get price from 2 hours ago
			idx2hAgo := len(tfData.Klines) - cfg.candlesFor2h
			price2hAgo := tfData.Klines[idx2hAgo].Open
			currentPrice := tfData.Klines[len(tfData.Klines)-1].Close

			if price2hAgo > 0 {
				return ((currentPrice - price2hAgo) / price2hAgo) * 100
			}
		}
	}

	return 0
}

// CheckCooldown checks if a position on this symbol was recently closed (cooldown period)
// Returns error if within cooldown period, nil if OK to trade
func CheckCooldown(symbol string, recentExitTimes map[string]time.Time) error {
	if exitTime, exists := recentExitTimes[symbol]; exists {
		elapsed := time.Since(exitTime)
		if elapsed < time.Duration(MinCooldownMinutes)*time.Minute {
			remaining := time.Duration(MinCooldownMinutes)*time.Minute - elapsed
			return fmt.Errorf("❌ [COOLDOWN] %s was closed %.0f min ago, must wait %.0f more min (min: %d min)",
				symbol, elapsed.Minutes(), remaining.Minutes(), MinCooldownMinutes)
		}
	}
	return nil
}

// LogEntryQualityCheck logs the entry quality check result for monitoring
func LogEntryQualityCheck(symbol, side string, result *EntryQualityResult) {
	if result.IsQualityEntry {
		if result.Warning != "" {
			logger.Warnf("[EntryQuality] ⚠️ %s %s PASSED WITH CAUTION: %s | BOLL pos=%.1f%%, RSI7=%.1f, RSI14=%.1f, PriceChg2h=%.1f%%",
				symbol, side, result.Warning, result.BOLLPosition, result.RSI7, result.RSI14, result.PriceChange2h)
		} else {
			logger.Infof("[EntryQuality] ✅ %s %s PASSED: BOLL pos=%.1f%%, RSI7=%.1f, RSI14=%.1f, PriceChg2h=%.1f%%",
				symbol, side, result.BOLLPosition, result.RSI7, result.RSI14, result.PriceChange2h)
		}
	} else {
		logger.Warnf("[EntryQuality] ❌ %s %s BLOCKED: %s | BOLL pos=%.1f%%, RSI7=%.1f, RSI14=%.1f, PriceChg2h=%.1f%%",
			symbol, side, result.Reason, result.BOLLPosition, result.RSI7, result.RSI14, result.PriceChange2h)
	}
}
