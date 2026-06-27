package store

import (
	"strings"
	"testing"
)

func TestApplySmallAccountBalancedModeUsesThreePositionProfile(t *testing.T) {
	config := GetDefaultStrategyConfig("en")
	config.RiskControl = RiskControlConfig{
		MaxPositions:                 1,
		BTCETHMaxLeverage:            10,
		AltcoinMaxLeverage:           5,
		BTCETHMaxPositionValueRatio:  5,
		AltcoinMaxPositionValueRatio: 5,
		MaxMarginUsage:               0.95,
		MinPositionSize:              40,
		MinRiskRewardRatio:           1.2,
		MinConfidence:                60,
	}

	ApplySmallAccountBalancedMode(&config)

	rc := config.RiskControl
	if rc.MaxPositions != 3 {
		t.Fatalf("expected max positions 3, got %d", rc.MaxPositions)
	}
	if rc.BTCETHMaxLeverage != 3 || rc.AltcoinMaxLeverage != 3 {
		t.Fatalf("expected 3x leverage caps, got BTC/ETH=%d altcoin=%d", rc.BTCETHMaxLeverage, rc.AltcoinMaxLeverage)
	}
	if rc.MaxMarginUsage != 0.75 {
		t.Fatalf("expected max margin usage 0.75, got %.2f", rc.MaxMarginUsage)
	}
	if rc.MinPositionSize != 75 {
		t.Fatalf("expected min position size 75, got %.2f", rc.MinPositionSize)
	}
	if rc.MinRiskRewardRatio != 1.8 {
		t.Fatalf("expected min RR 1.8, got %.2f", rc.MinRiskRewardRatio)
	}
	if rc.MinConfidence != 70 {
		t.Fatalf("expected min confidence 70, got %d", rc.MinConfidence)
	}
	if !strings.Contains(config.CustomPrompt, "Maximum 3 simultaneous positions") {
		t.Fatal("expected custom prompt to describe the three-position profile")
	}
}
