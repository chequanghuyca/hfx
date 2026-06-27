package trader

import (
	"math"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/store"
)

func newRiskTestTrader() *AutoTrader {
	return &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				RiskControl: store.RiskControlConfig{
					MinConfidence:      75,
					MinRiskRewardRatio: 2.2,
					MaxMarginUsage:     0.60,
					MaxRiskPerTradePct: 0.008,
				},
			},
		},
	}
}

func TestEnforceRiskBudgetCapsPosition(t *testing.T) {
	at := newRiskTestTrader()
	decision := &kernel.Decision{
		PositionSizeUSD: 1000,
		StopLoss:        98,
	}
	size, capped, err := at.enforceRiskBudget(decision, 1000, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capped || size != 400 {
		t.Fatalf("expected size capped to 400, got %.2f (capped=%v)", size, capped)
	}
}

func TestNormalizeOpeningLeverageUsesConfiguredProfile(t *testing.T) {
	at := newRiskTestTrader()
	at.config.StrategyConfig.RiskControl.BTCETHMaxLeverage = 3
	at.config.StrategyConfig.RiskControl.AltcoinMaxLeverage = 3

	altDecision := &kernel.Decision{Symbol: "ADAUSDT", Leverage: 1}
	at.normalizeOpeningLeverage(altDecision)
	if altDecision.Leverage != 3 {
		t.Fatalf("expected altcoin leverage normalized to 3x, got %dx", altDecision.Leverage)
	}

	btcDecision := &kernel.Decision{Symbol: "BTCUSDT", Leverage: 10}
	at.normalizeOpeningLeverage(btcDecision)
	if btcDecision.Leverage != 3 {
		t.Fatalf("expected BTC leverage normalized to 3x, got %dx", btcDecision.Leverage)
	}
}

func TestValidateOpeningRiskUsesCurrentPrice(t *testing.T) {
	at := newRiskTestTrader()
	decision := &kernel.Decision{
		Symbol:          "SOLUSDT",
		Action:          "open_long",
		PositionSizeUSD: 1000,
		StopLoss:        98,
		TakeProfit:      105,
		Confidence:      80,
	}

	if err := at.validateOpeningRisk(decision, 100); err != nil {
		t.Fatalf("expected valid decision: %v", err)
	}
	if decision.RiskUSD != 20 {
		t.Fatalf("expected risk_usd=20, got %.2f", decision.RiskUSD)
	}
}

func TestValidateOpeningRiskRejectsLowConfidence(t *testing.T) {
	at := newRiskTestTrader()
	decision := &kernel.Decision{
		Symbol:          "SOLUSDT",
		Action:          "open_short",
		PositionSizeUSD: 1000,
		StopLoss:        102,
		TakeProfit:      95,
		Confidence:      70,
	}

	if err := at.validateOpeningRisk(decision, 100); err == nil {
		t.Fatal("expected low-confidence decision to be rejected")
	}
}

func TestValidateOpeningRiskAllowsTinyRiskRewardRoundingNoise(t *testing.T) {
	at := newRiskTestTrader()
	at.config.StrategyConfig.RiskControl.MinRiskRewardRatio = 1.6
	decision := &kernel.Decision{
		Symbol:          "AVAXUSDT",
		Action:          "open_short",
		PositionSizeUSD: 50,
		StopLoss:        101,
		TakeProfit:      98.400000001, // RR is effectively 1.6 with tiny float noise.
		Confidence:      80,
	}

	if err := at.validateOpeningRisk(decision, 100); err != nil {
		t.Fatalf("expected tiny RR rounding noise to pass: %v", err)
	}
}

func TestValidateOpeningRiskRejectsClearlyLowRiskReward(t *testing.T) {
	at := newRiskTestTrader()
	at.config.StrategyConfig.RiskControl.MinRiskRewardRatio = 1.6
	decision := &kernel.Decision{
		Symbol:          "AVAXUSDT",
		Action:          "open_short",
		PositionSizeUSD: 50,
		StopLoss:        101,
		TakeProfit:      98.41, // RR 1.59, truly below 1.6.
		Confidence:      80,
	}

	if err := at.validateOpeningRisk(decision, 100); err == nil {
		t.Fatal("expected clearly low RR to be rejected")
	}
}

func TestEnforceMaxMarginUsageCapsPosition(t *testing.T) {
	at := newRiskTestTrader()
	size, capped, err := at.enforceMaxMarginUsage(1000, 4, 1000, 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capped {
		t.Fatal("expected position to be capped")
	}
	if math.Abs(size-784) > 0.001 {
		t.Fatalf("expected capped size 784, got %.2f", size)
	}
}

func TestEnforceMinPositionSizeUsesConfiguredNotionalFloor(t *testing.T) {
	at := newRiskTestTrader()
	at.config.StrategyConfig.RiskControl.MinPositionSize = 75

	if err := at.enforceMinPositionSize(74.99, 132); err == nil {
		t.Fatal("expected 74.99 USDT notional to be below configured 75 USDT floor")
	}
	if err := at.enforceMinPositionSize(78.54, 132); err != nil {
		t.Fatalf("expected 78.54 USDT risk-capped notional to pass configured floor: %v", err)
	}
}

func TestEnforceRiskBudgetAllowsDesiredMarginAtPointNinePctRisk(t *testing.T) {
	at := newRiskTestTrader()
	at.config.StrategyConfig.RiskControl.MaxRiskPerTradePct = 0.009
	decision := &kernel.Decision{
		PositionSizeUSD: 78,
		StopLoss:        98.5,
	}

	size, capped, err := at.enforceRiskBudget(decision, 132, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capped {
		t.Fatalf("expected 78 USDT notional to fit 0.9%% risk with 1.5%% stop, got capped to %.2f", size)
	}
}

func TestEnforceMinHoldTimeBlocksWeakEarlyClose(t *testing.T) {
	at := newRiskTestTrader()
	at.positionFirstSeenTime = map[string]int64{
		"BTCUSDT_long": time.Now().Add(-5 * time.Minute).UnixMilli(),
	}

	if err := at.enforceMinHoldTime("BTCUSDT", "long", "close weak setup"); err == nil {
		t.Fatal("expected weak early close to be blocked")
	}
}

func TestEnforceMinHoldTimeAllowsProtectiveEarlyClose(t *testing.T) {
	at := newRiskTestTrader()
	at.positionFirstSeenTime = map[string]int64{
		"BTCUSDT_long": time.Now().Add(-5 * time.Minute).UnixMilli(),
	}

	if err := at.enforceMinHoldTime("BTCUSDT", "long", "stop loss hit"); err != nil {
		t.Fatalf("protective early close should be allowed: %v", err)
	}
}
