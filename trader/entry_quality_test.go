package trader

import (
	"testing"

	"nofx/market"
)

func TestCheckEntryQualityPriceChangeIsDirectional(t *testing.T) {
	pumped := &market.Data{
		Symbol:       "TESTUSDT",
		CurrentPrice: 115,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": {
				Klines: []market.KlineBar{
					{Open: 100, Close: 100},
					{Open: 102, Close: 102},
					{Open: 104, Close: 104},
					{Open: 106, Close: 106},
					{Open: 108, Close: 108},
					{Open: 110, Close: 110},
					{Open: 112, Close: 112},
					{Open: 114, Close: 114},
					{Open: 115, Close: 115},
				},
			},
		},
	}

	longResult, err := CheckEntryQuality("TESTUSDT", "long", pumped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if longResult.IsQualityEntry {
		t.Fatal("expected pumped market to block LONG entries")
	}

	shortResult, err := CheckEntryQuality("TESTUSDT", "short", pumped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !shortResult.IsQualityEntry {
		t.Fatalf("pumped market should not automatically block SHORT entries: %s", shortResult.Reason)
	}
}

func TestCheckEntryQualityDumpIsDirectional(t *testing.T) {
	dumped := &market.Data{
		Symbol:       "TESTUSDT",
		CurrentPrice: 85,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": {
				Klines: []market.KlineBar{
					{Open: 100, Close: 100},
					{Open: 98, Close: 98},
					{Open: 96, Close: 96},
					{Open: 94, Close: 94},
					{Open: 92, Close: 92},
					{Open: 90, Close: 90},
					{Open: 88, Close: 88},
					{Open: 86, Close: 86},
					{Open: 85, Close: 85},
				},
			},
		},
	}

	shortResult, err := CheckEntryQuality("TESTUSDT", "short", dumped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shortResult.IsQualityEntry {
		t.Fatal("expected dumped market to block SHORT entries")
	}

	longResult, err := CheckEntryQuality("TESTUSDT", "long", dumped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !longResult.IsQualityEntry {
		t.Fatalf("dumped market should not automatically block LONG entries: %s", longResult.Reason)
	}
}

func TestCheckEntryQualityShortAllowsModerateOversoldContinuation(t *testing.T) {
	oversoldButNotExtreme := &market.Data{
		Symbol:       "ETHUSDT",
		CurrentPrice: 3100,
		CurrentRSI7:  19.1,
	}

	result, err := CheckEntryQuality("ETHUSDT", "short", oversoldButNotExtreme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsQualityEntry {
		t.Fatalf("expected moderate oversold SHORT to pass with caution, got block: %s", result.Reason)
	}
	if result.Warning == "" {
		t.Fatal("expected a caution warning for oversold SHORT continuation")
	}
}

func TestCheckEntryQualityShortBlocksExtremeOversold(t *testing.T) {
	extremeOversold := &market.Data{
		Symbol:       "ETHUSDT",
		CurrentPrice: 3100,
		CurrentRSI7:  15.0,
	}

	result, err := CheckEntryQuality("ETHUSDT", "short", extremeOversold)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsQualityEntry {
		t.Fatal("expected extremely oversold SHORT to be blocked")
	}
}
