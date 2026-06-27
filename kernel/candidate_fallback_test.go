package kernel

import (
	"testing"

	"nofx/store"
)

func TestFallbackCandidateCoinsUsesLiquidListAndExclusions(t *testing.T) {
	config := store.GetDefaultStrategyConfig("en")
	config.CoinSource.StaticCoins = nil
	config.CoinSource.ExcludedCoins = []string{"XRPUSDT"}

	engine := NewStrategyEngine(&config)
	coins := engine.fallbackCandidateCoins()

	if len(coins) == 0 {
		t.Fatal("expected liquid fallback candidates")
	}
	for _, coin := range coins {
		if coin.Symbol == "XRPUSDT" {
			t.Fatal("expected excluded coin to be removed")
		}
		if len(coin.Sources) != 1 || coin.Sources[0] != "liquid_fallback" {
			t.Fatalf("unexpected fallback source for %s: %v", coin.Symbol, coin.Sources)
		}
	}
}

func TestFallbackCandidateCoinsIsCappedForCompactPrompts(t *testing.T) {
	config := store.GetDefaultStrategyConfig("en")
	config.CoinSource.StaticCoins = []string{
		"BTCUSDT", "ETHUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT",
		"AVAXUSDT", "LINKUSDT", "SUIUSDT", "LTCUSDT", "NEARUSDT",
		"UNIUSDT", "AAVEUSDT",
	}

	engine := NewStrategyEngine(&config)
	coins := engine.fallbackCandidateCoins()
	if len(coins) != 8 {
		t.Fatalf("expected compact fallback cap of 8, got %d: %#v", len(coins), coins)
	}
}

func TestMixedCandidateCoinsUsesStaticOnlyAsFallback(t *testing.T) {
	config := store.GetDefaultStrategyConfig("en")
	config.CoinSource.SourceType = "mixed"
	config.CoinSource.UseAI500 = false
	config.CoinSource.UseOITop = false
	config.CoinSource.UseOILow = false
	config.CoinSource.StaticCoins = []string{"DOGEUSDT"}

	engine := NewStrategyEngine(&config)
	coins, err := engine.GetCandidateCoins()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(coins) != 1 {
		t.Fatalf("expected one fallback coin, got %d: %#v", len(coins), coins)
	}
	if coins[0].Symbol != "DOGEUSDT" {
		t.Fatalf("expected DOGEUSDT fallback, got %s", coins[0].Symbol)
	}
	if len(coins[0].Sources) != 1 || coins[0].Sources[0] != "static_fallback" {
		t.Fatalf("expected fallback source only, got %v", coins[0].Sources)
	}
}

func TestMixedCandidatesTopUpWithStaticAfterExclusions(t *testing.T) {
	config := store.GetDefaultStrategyConfig("en")
	config.CoinSource.StaticCoins = []string{"XRPUSDT", "DOGEUSDT", "ADAUSDT"}
	config.CoinSource.ExcludedCoins = []string{"BTCUSDT"}

	engine := NewStrategyEngine(&config)
	got := engine.topUpWithStaticFallback([]CandidateCoin{
		{Symbol: "BTCUSDT", Sources: []string{"ai500"}},
		{Symbol: "XRPUSDT", Sources: []string{"oi_top"}},
	}, 3)

	want := []string{"XRPUSDT", "DOGEUSDT", "ADAUSDT"}
	if len(got) != len(want) {
		t.Fatalf("unexpected top-up count: got %d want %d (%#v)", len(got), len(want), got)
	}
	for i, symbol := range want {
		if got[i].Symbol != symbol {
			t.Fatalf("unexpected symbol at %d: got %s want %s", i, got[i].Symbol, symbol)
		}
	}
	if got[1].Sources[0] != "static_fallback" {
		t.Fatalf("expected static_fallback source for top-up coin, got %v", got[1].Sources)
	}
}

func TestMergeRankedCandidateListsIsBalancedDeterministicAndCapped(t *testing.T) {
	lists := [][]CandidateCoin{
		{
			{Symbol: "AAAUSDT", Sources: []string{"ai500"}},
			{Symbol: "BBBUSDT", Sources: []string{"ai500"}},
			{Symbol: "CCCUSDT", Sources: []string{"ai500"}},
		},
		{
			{Symbol: "DDDUSDT", Sources: []string{"oi_top"}},
			{Symbol: "AAAUSDT", Sources: []string{"oi_top"}},
			{Symbol: "EEEUSDT", Sources: []string{"oi_top"}},
		},
		{
			{Symbol: "FFFUSDT", Sources: []string{"oi_low"}},
			{Symbol: "GGGUSDT", Sources: []string{"oi_low"}},
		},
	}

	got := mergeRankedCandidateLists(lists, 6)
	want := []string{"AAAUSDT", "DDDUSDT", "FFFUSDT", "BBBUSDT", "GGGUSDT", "CCCUSDT"}
	if len(got) != len(want) {
		t.Fatalf("unexpected candidate count: got %d want %d (%#v)", len(got), len(want), got)
	}
	for i, symbol := range want {
		if got[i].Symbol != symbol {
			t.Fatalf("unexpected symbol at %d: got %s want %s", i, got[i].Symbol, symbol)
		}
	}
	if len(got[0].Sources) != 2 || got[0].Sources[0] != "ai500" || got[0].Sources[1] != "oi_top" {
		t.Fatalf("expected merged source tags for AAAUSDT, got %v", got[0].Sources)
	}
}
