package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/provider/nofxos"
	"nofx/security"
	"nofx/store"
	"regexp"
	"strings"
	"time"
)

// ============================================================================
// Pre-compiled regular expressions (performance optimization)
// ============================================================================

var (
	// Safe regex: extract JSON decision arrays. Raw array matching requires at
	// least one object so prose like "[ and end with ]" is not treated as JSON.
	reJSONFence          = regexp.MustCompile(`(?is)` + "```(?:json)?\\s*(\\{.*\\}|\\[.*\\])\\s*```")
	reJSONDecisionObject = regexp.MustCompile(`(?is)\{\s*"decisions"\s*:\s*\[.*\]\s*\}`)
	reJSONArray          = regexp.MustCompile(`(?is)\[\s*\{.*\}\s*\]`)
	reArrayHead          = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace     = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes     = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// XML tag extraction (supports any characters in reasoning chain)
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)

// ============================================================================
// Type Definitions
// ============================================================================

// PositionInfo position information
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	PeakPnLPct       float64 `json:"peak_pnl_pct"` // Historical peak profit percentage
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // Position update timestamp (milliseconds)
}

// AccountInfo account information
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // Account equity
	AvailableBalance float64 `json:"available_balance"` // Available balance
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // Unrealized profit/loss
	TotalPnL         float64 `json:"total_pnl"`         // Total profit/loss
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total profit/loss percentage
	MarginUsed       float64 `json:"margin_used"`       // Used margin
	MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin usage rate
	PositionCount    int     `json:"position_count"`    // Number of positions
}

// CandidateCoin candidate coin (from coin pool)
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // Sources: "ai500" and/or "oi_top"
}

// OITopData open interest growth top data (for AI decision reference)
type OITopData struct {
	Rank              int     // OI Top ranking
	OIDeltaPercent    float64 // Open interest change percentage (1 hour)
	OIDeltaValue      float64 // Open interest change value
	PriceDeltaPercent float64 // Price change percentage
}

// TradingStats trading statistics (for AI input)
type TradingStats struct {
	TotalTrades    int     `json:"total_trades"`     // Total number of trades (closed)
	WinRate        float64 `json:"win_rate"`         // Win rate (%)
	ProfitFactor   float64 `json:"profit_factor"`    // Profit factor
	SharpeRatio    float64 `json:"sharpe_ratio"`     // Sharpe ratio
	TotalPnL       float64 `json:"total_pnl"`        // Total profit/loss
	AvgWin         float64 `json:"avg_win"`          // Average win
	AvgLoss        float64 `json:"avg_loss"`         // Average loss
	MaxDrawdownPct float64 `json:"max_drawdown_pct"` // Maximum drawdown (%)
}

// RecentOrder recently completed order (for AI input)
type RecentOrder struct {
	Symbol       string  `json:"symbol"`        // Trading pair
	Side         string  `json:"side"`          // long/short
	EntryPrice   float64 `json:"entry_price"`   // Entry price
	ExitPrice    float64 `json:"exit_price"`    // Exit price
	RealizedPnL  float64 `json:"realized_pnl"`  // Realized profit/loss
	PnLPct       float64 `json:"pnl_pct"`       // Profit/loss percentage
	EntryTime    string  `json:"entry_time"`    // Entry time
	ExitTime     string  `json:"exit_time"`     // Exit time
	HoldDuration string  `json:"hold_duration"` // Hold duration, e.g. "2h30m"
	CyclesPassed int     `json:"cycles_passed"` // Number of cycles passed since closure
}

// Context trading context (complete information passed to AI)
type Context struct {
	CurrentTime        string                             `json:"current_time"`
	RuntimeMinutes     int                                `json:"runtime_minutes"`
	CallCount          int                                `json:"call_count"`
	Account            AccountInfo                        `json:"account"`
	Positions          []PositionInfo                     `json:"positions"`
	CandidateCoins     []CandidateCoin                    `json:"candidate_coins"`
	PromptVariant      string                             `json:"prompt_variant,omitempty"`
	TradingStats       *TradingStats                      `json:"trading_stats,omitempty"`
	RecentOrders       []RecentOrder                      `json:"recent_orders,omitempty"`
	MarketDataMap      map[string]*market.Data            `json:"-"`
	MultiTFMarket      map[string]map[string]*market.Data `json:"-"`
	OITopDataMap       map[string]*OITopData              `json:"-"`
	QuantDataMap       map[string]*QuantData              `json:"-"`
	OIRankingData      *nofxos.OIRankingData              `json:"-"` // Market-wide OI ranking data
	NetFlowRankingData *nofxos.NetFlowRankingData         `json:"-"` // Market-wide fund flow ranking data
	PriceRankingData   *nofxos.PriceRankingData           `json:"-"` // Market-wide price gainers/losers
	BTCETHLeverage     int                                `json:"-"`
	AltcoinLeverage    int                                `json:"-"`
	Timeframes         []string                           `json:"-"`
}

// Decision AI trading decision
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // Standard: "open_long", "open_short", "close_long", "close_short", "hold", "wait"
	// Grid actions: "place_buy_limit", "place_sell_limit", "cancel_order", "cancel_all_orders", "pause_grid", "resume_grid", "adjust_grid"

	// Opening position parameters
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// Grid trading parameters
	Price      float64 `json:"price,omitempty"`       // Limit order price (for grid)
	Quantity   float64 `json:"quantity,omitempty"`    // Order quantity (for grid)
	LevelIndex int     `json:"level_index,omitempty"` // Grid level index
	OrderID    string  `json:"order_id,omitempty"`    // Order ID (for cancel)

	// Common parameters
	Confidence int     `json:"confidence,omitempty"` // Confidence level (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // Maximum USD risk
	Reasoning  string  `json:"reasoning"`
}

// FullDecision AI's complete decision (including chain of thought)
type FullDecision struct {
	SystemPrompt        string     `json:"system_prompt"`
	UserPrompt          string     `json:"user_prompt"`
	CoTTrace            string     `json:"cot_trace"`
	Decisions           []Decision `json:"decisions"`
	RawResponse         string     `json:"raw_response"`
	Timestamp           time.Time  `json:"timestamp"`
	AIRequestDurationMs int64      `json:"ai_request_duration_ms,omitempty"`
}

// QuantData quantitative data structure (fund flow, position changes, price changes)
type QuantData struct {
	Symbol      string             `json:"symbol"`
	Price       float64            `json:"price"`
	Netflow     *NetflowData       `json:"netflow,omitempty"`
	OI          map[string]*OIData `json:"oi,omitempty"`
	PriceChange map[string]float64 `json:"price_change,omitempty"`
}

type NetflowData struct {
	Institution *FlowTypeData `json:"institution,omitempty"`
	Personal    *FlowTypeData `json:"personal,omitempty"`
}

type FlowTypeData struct {
	Future map[string]float64 `json:"future,omitempty"`
	Spot   map[string]float64 `json:"spot,omitempty"`
}

type OIData struct {
	CurrentOI float64                 `json:"current_oi"`
	Delta     map[string]*OIDeltaData `json:"delta,omitempty"`
}

type OIDeltaData struct {
	OIDelta        float64 `json:"oi_delta"`
	OIDeltaValue   float64 `json:"oi_delta_value"`
	OIDeltaPercent float64 `json:"oi_delta_percent"`
}

// ============================================================================
// StrategyEngine - Core Strategy Execution Engine
// ============================================================================

// StrategyEngine strategy execution engine
type StrategyEngine struct {
	config       *store.StrategyConfig
	nofxosClient *nofxos.Client
}

// NewStrategyEngine creates strategy execution engine
func NewStrategyEngine(config *store.StrategyConfig) *StrategyEngine {
	// Create NofxOS client with API key from config
	apiKey := config.Indicators.NofxOSAPIKey
	if apiKey == "" {
		apiKey = nofxos.DefaultAuthKey
	}
	client := nofxos.NewClient(nofxos.DefaultBaseURL, apiKey)

	return &StrategyEngine{
		config:       config,
		nofxosClient: client,
	}
}

// GetRiskControlConfig gets risk control configuration
func (e *StrategyEngine) GetRiskControlConfig() store.RiskControlConfig {
	return e.config.RiskControl
}

// GetLanguage returns the language from config or falls back to auto-detection
func (e *StrategyEngine) GetLanguage() Language {
	switch e.config.Language {
	case "zh":
		return LangChinese
	case "en":
		return LangEnglish
	default:
		// Fall back to auto-detection from prompt content for backward compatibility
		return detectLanguage(e.config.PromptSections.RoleDefinition)
	}
}

// GetConfig gets complete strategy configuration
func (e *StrategyEngine) GetConfig() *store.StrategyConfig {
	return e.config
}

// ============================================================================
// Entry Functions - Main API
// ============================================================================

// GetFullDecision gets AI's complete trading decision (batch analysis of all coins and positions)
// Uses default strategy configuration - for production use GetFullDecisionWithStrategy with explicit config
func GetFullDecision(ctx *Context, mcpClient mcp.AIClient) (*FullDecision, error) {
	defaultConfig := store.GetDefaultStrategyConfig("en")
	engine := NewStrategyEngine(&defaultConfig)
	return GetFullDecisionWithStrategy(ctx, mcpClient, engine, "")
}

// GetFullDecisionWithStrategy uses StrategyEngine to get AI decision (unified prompt generation)
func GetFullDecisionWithStrategy(ctx *Context, mcpClient mcp.AIClient, engine *StrategyEngine, variant string) (*FullDecision, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if engine == nil {
		defaultConfig := store.GetDefaultStrategyConfig("en")
		engine = NewStrategyEngine(&defaultConfig)
	}

	// 1. Fetch market data using strategy config
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataWithStrategy(ctx, engine); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
	}

	// Ensure OITopDataMap is initialized
	if ctx.OITopDataMap == nil {
		ctx.OITopDataMap = make(map[string]*OITopData)
		oiPositions, err := engine.nofxosClient.GetOITopPositions()
		if err == nil {
			for _, pos := range oiPositions {
				ctx.OITopDataMap[pos.Symbol] = &OITopData{
					Rank:              pos.Rank,
					OIDeltaPercent:    pos.OIDeltaPercent,
					OIDeltaValue:      pos.OIDeltaValue,
					PriceDeltaPercent: pos.PriceDeltaPercent,
				}
			}
		}
	}

	// 2. Build System Prompt using strategy engine
	riskConfig := engine.GetRiskControlConfig()
	systemPrompt := engine.BuildSystemPrompt(ctx.Account.TotalEquity, variant)

	// 3. Build User Prompt using strategy engine
	userPrompt := engine.BuildUserPrompt(ctx)

	// 4. Call AI API and parse response. Some free/reasoning models occasionally
	// answer with prose that restates the format rules instead of JSON. Treat that
	// as a format failure and try the next configured fallback model before giving
	// up into safe-wait.
	const maxDecisionFormatAttempts = 6
	currentSystemPrompt := systemPrompt
	currentUserPrompt := userPrompt
	var (
		decision           *FullDecision
		lastParseErr       error
		totalAIRequestTime time.Duration
		truncationRepairs  int
	)

	for attempt := 1; attempt <= maxDecisionFormatAttempts; attempt++ {
		aiCallStart := time.Now()
		aiResponse, err := mcpClient.CallWithMessages(currentSystemPrompt, currentUserPrompt)
		aiCallDuration := time.Since(aiCallStart)
		totalAIRequestTime += aiCallDuration
		if err != nil {
			var truncatedErr *mcp.TruncatedResponseError
			if errors.As(err, &truncatedErr) {
				truncationRepairs++
				partial := strings.TrimSpace(truncatedErr.Content)
				if partial == "" {
					partial = strings.TrimSpace(aiResponse)
				}
				if !isMeaningfulTruncatedPartial(partial) {
					logger.Warnf("⚠️  AI output was truncated before useful analysis; returning safe wait without repair")
					return truncatedSafeWaitDecision(systemPrompt, userPrompt, partial, totalAIRequestTime), nil
				}

				if waitDecision, ok := inferExplicitWaitFromAnalysis(partial); ok {
					logger.Infof("✓ Truncated Gemma analysis explicitly concluded no trade; converting directly to valid wait")
					return analysisDerivedWaitDecision(
						systemPrompt,
						userPrompt,
						partial,
						waitDecision,
						totalAIRequestTime,
					), nil
				}

				if attempt < maxDecisionFormatAttempts && truncationRepairs <= 3 {
					logger.Warnf("⚠️  AI response reached token limit; running compact JSON repair (%d/3)", truncationRepairs)
					if truncationRepairs > 1 {
						if switcher, ok := mcpClient.(interface{ ForceNextFallbackModel(string) bool }); ok {
							switcher.ForceNextFallbackModel("compact JSON repair was also truncated")
						}
					}
					currentSystemPrompt = decisionRepairSystemPrompt()
					currentUserPrompt = buildTruncatedDecisionRepairPrompt(partial, ctx.Account.TotalEquity, riskConfig)
					continue
				}

				logger.Warnf("⚠️  Could not safely repair truncated AI output; returning a valid wait decision")
				return truncatedSafeWaitDecision(systemPrompt, userPrompt, partial, totalAIRequestTime), nil
			}
			if isTemporaryAIProviderError(err) {
				logger.Warnf("⚠️  AI provider unavailable after retries; entering safe wait instead of failing cycle")
				return aiProviderUnavailableWaitDecision(systemPrompt, userPrompt, err, totalAIRequestTime), nil
			}
			return nil, fmt.Errorf("AI API call failed: %w", err)
		}

		decision, lastParseErr = parseFullDecisionResponse(
			aiResponse,
			ctx.Account.TotalEquity,
			riskConfig.BTCETHMaxLeverage,
			riskConfig.AltcoinMaxLeverage,
			riskConfig.BTCETHMaxPositionValueRatio,
			riskConfig.AltcoinMaxPositionValueRatio,
		)

		if decision != nil {
			decision.Timestamp = time.Now()
			decision.SystemPrompt = currentSystemPrompt
			decision.UserPrompt = currentUserPrompt
			decision.AIRequestDurationMs = totalAIRequestTime.Milliseconds()
			if len(decision.Decisions) == 1 && isAnalysisDerivedWait(decision.Decisions[0]) {
				decision.RawResponse = marshalDecisionEnvelope(decision.Decisions)
			} else {
				decision.RawResponse = aiResponse
			}
		}

		if lastParseErr == nil && !isInvalidJSONSafeWaitDecision(decision) {
			return decision, nil
		}

		formatReason := "AI output did not contain a valid JSON decision"
		if lastParseErr != nil {
			formatReason = lastParseErr.Error()
		} else if decision != nil && len(decision.Decisions) > 0 {
			formatReason = decision.Decisions[0].Reasoning
		}

		if attempt < maxDecisionFormatAttempts {
			if switcher, ok := mcpClient.(interface{ ForceNextFallbackModel(string) bool }); ok {
				if switcher.ForceNextFallbackModel(formatReason) {
					currentSystemPrompt = systemPrompt
					currentUserPrompt = appendDecisionFormatRetryPrompt(userPrompt, attempt+1)
					continue
				}
			}

			if attempt == 1 {
				logger.Warnf("⚠️  AI response had no valid decision JSON; retrying once with stricter format reminder")
				currentSystemPrompt = systemPrompt
				currentUserPrompt = appendDecisionFormatRetryPrompt(userPrompt, attempt+1)
				continue
			}
		}

		break
	}

	if lastParseErr != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", lastParseErr)
	}
	return decision, nil
}

func analysisDerivedWaitDecision(systemPrompt, userPrompt, analysis string, wait Decision, duration time.Duration) *FullDecision {
	decisions := []Decision{wait}
	return &FullDecision{
		Timestamp:           time.Now(),
		CoTTrace:            analysis,
		Decisions:           decisions,
		SystemPrompt:        systemPrompt,
		UserPrompt:          userPrompt,
		RawResponse:         marshalDecisionEnvelope(decisions),
		AIRequestDurationMs: duration.Milliseconds(),
	}
}

func marshalDecisionEnvelope(decisions []Decision) string {
	payload, err := json.Marshal(struct {
		Decisions []Decision `json:"decisions"`
	}{Decisions: decisions})
	if err != nil {
		return `{"decisions":[{"symbol":"ALL","action":"wait","confidence":60,"reasoning":"No executable trigger yet"}]}`
	}
	return string(payload)
}

func decisionRepairSystemPrompt() string {
	return `You are a strict JSON repair service for a crypto trading engine.
Return exactly one raw JSON object with a non-empty "decisions" array.
Do not redo market analysis. Preserve a complete executable decision only when the supplied partial output contains its symbol, side, entry context, stop loss, take profit, confidence, and size.
If those opening fields are incomplete or uncertain, return a wait decision. Never invent prices or a trade.`
}

func buildTruncatedDecisionRepairPrompt(partial string, equity float64, risk store.RiskControlConfig) string {
	const maxPartialChars = 16000
	partial = strings.TrimSpace(partial)
	if len(partial) > maxPartialChars {
		partial = partial[len(partial)-maxPartialChars:]
	}

	maxRiskPct := risk.MaxRiskPerTradePct
	if maxRiskPct <= 0 || maxRiskPct > 0.05 {
		maxRiskPct = 0.009
	}
	maxRiskUSD := equity * maxRiskPct

	return fmt.Sprintf(`Repair the partial output below into valid machine JSON.

Allowed actions: open_long, open_short, close_long, close_short, hold, wait.
Every item requires: symbol, action, confidence, reasoning.
Opening additionally requires: leverage, position_size_usd, stop_loss, take_profit, risk_usd.
Opening limits: max leverage %d altcoins / %d BTC-ETH, confidence >= %d, risk_usd <= %.2f, RR >= %.1f.
If the partial output only concludes that no trigger exists, return:
{"decisions":[{"symbol":"ALL","action":"wait","leverage":0,"position_size_usd":0,"stop_loss":0,"take_profit":0,"confidence":60,"risk_usd":0,"reasoning":"No executable trigger yet"}]}

PARTIAL OUTPUT:
%s`,
		risk.AltcoinMaxLeverage,
		risk.BTCETHMaxLeverage,
		risk.MinConfidence,
		maxRiskUSD,
		risk.MinRiskRewardRatio,
		partial,
	)
}

func isMeaningfulTruncatedPartial(partial string) bool {
	trimmed := strings.TrimSpace(partial)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	tooShortFragments := []string{
		`{"decisions":`,
		`{"decisions":[`,
		`{"decision":`,
		"```json",
		"```json\n",
		"[",
		"{",
	}
	for _, fragment := range tooShortFragments {
		if lower == fragment {
			return false
		}
	}

	// Useful repair needs either natural-language market analysis or a nearly
	// complete decision object. A tiny JSON prefix has neither, so repairing it
	// only wastes another slow free-model call and still ends in wait.
	hasMarketContext := strings.Contains(lower, "analysis") ||
		strings.Contains(lower, "market") ||
		strings.Contains(lower, "candidate") ||
		strings.Contains(lower, "conclusion") ||
		strings.Contains(lower, "trigger") ||
		strings.Contains(lower, "confidence") ||
		strings.Contains(lower, "risk-reward") ||
		strings.Contains(lower, "rsi") ||
		strings.Contains(lower, "ema") ||
		strings.Contains(lower, "boll")
	if !hasMarketContext && len(trimmed) < 200 {
		return false
	}
	return true
}

func truncatedSafeWaitDecision(systemPrompt, userPrompt, partial string, duration time.Duration) *FullDecision {
	reason := "AI output reached its token limit and could not be safely repaired"
	if strings.Contains(strings.ToLower(partial), "no candidate") ||
		strings.Contains(strings.ToLower(partial), "no executable") ||
		strings.Contains(strings.ToLower(partial), "wait") {
		reason = "No executable trigger found; truncated analysis was safely converted to wait"
	}

	return &FullDecision{
		Timestamp: time.Now(),
		Decisions: []Decision{{
			Symbol:     "ALL",
			Action:     "wait",
			Confidence: 60,
			Reasoning:  reason,
		}},
		SystemPrompt:        systemPrompt,
		UserPrompt:          userPrompt,
		RawResponse:         partial,
		AIRequestDurationMs: duration.Milliseconds(),
	}
}

func isTemporaryAIProviderError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "empty response") ||
		strings.Contains(lower, "empty assistant content") ||
		strings.Contains(lower, "status 429") ||
		strings.Contains(lower, "rate-limited") ||
		strings.Contains(lower, "rate limited") ||
		strings.Contains(lower, "temporarily rate-limited") ||
		strings.Contains(lower, "temporarily unavailable") ||
		strings.Contains(lower, "provider returned error") ||
		strings.Contains(lower, "status 502") ||
		strings.Contains(lower, "status 503") ||
		strings.Contains(lower, "status 520") ||
		strings.Contains(lower, "status 524")
}

func aiProviderUnavailableWaitDecision(systemPrompt, userPrompt string, err error, duration time.Duration) *FullDecision {
	reason := "AI provider temporarily unavailable; safe wait"
	return &FullDecision{
		Timestamp: time.Now(),
		Decisions: []Decision{{
			Symbol:          "ALL",
			Action:          "wait",
			Leverage:        0,
			PositionSizeUSD: 0,
			StopLoss:        0,
			TakeProfit:      0,
			Confidence:      0,
			RiskUSD:         0,
			Reasoning:       reason,
		}},
		SystemPrompt:        systemPrompt,
		UserPrompt:          userPrompt,
		RawResponse:         fmt.Sprintf(`{"decisions":[{"symbol":"ALL","action":"wait","leverage":0,"position_size_usd":0,"stop_loss":0,"take_profit":0,"confidence":0,"risk_usd":0,"reasoning":%q}],"provider_error":%q}`, reason, err.Error()),
		AIRequestDurationMs: duration.Milliseconds(),
	}
}

// ============================================================================
// Market Data Fetching
// ============================================================================

// fetchMarketDataWithStrategy fetches market data using strategy config (multiple timeframes)
func fetchMarketDataWithStrategy(ctx *Context, engine *StrategyEngine) error {
	config := engine.GetConfig()
	ctx.MarketDataMap = make(map[string]*market.Data)

	timeframes := config.Indicators.Klines.SelectedTimeframes
	primaryTimeframe := config.Indicators.Klines.PrimaryTimeframe
	klineCount := config.Indicators.Klines.PrimaryCount

	// Compatible with old configuration
	if len(timeframes) == 0 {
		if primaryTimeframe != "" {
			timeframes = append(timeframes, primaryTimeframe)
		} else {
			timeframes = append(timeframes, "3m")
		}
		if config.Indicators.Klines.LongerTimeframe != "" {
			timeframes = append(timeframes, config.Indicators.Klines.LongerTimeframe)
		}
	}
	if primaryTimeframe == "" {
		primaryTimeframe = timeframes[0]
	}
	if klineCount <= 0 {
		klineCount = 30
	}

	logger.Infof("📊 Strategy timeframes: %v, Primary: %s, Kline count: %d", timeframes, primaryTimeframe, klineCount)

	// 1. First fetch data for position coins (must fetch)
	for _, pos := range ctx.Positions {
		data, err := market.GetWithTimeframes(pos.Symbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for position %s: %v", pos.Symbol, err)
			continue
		}
		ctx.MarketDataMap[pos.Symbol] = data
	}

	// 2. Fetch data for all candidate coins
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	const minOIThresholdMillions = 25.0 // Small-account mode: keep only basic liquidity, not institutional-only OI.

	for _, coin := range ctx.CandidateCoins {
		if _, exists := ctx.MarketDataMap[coin.Symbol]; exists {
			continue
		}

		data, err := market.GetWithTimeframes(coin.Symbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for %s: %v", coin.Symbol, err)
			continue
		}

		// Liquidity filter (skip for xyz dex assets - they don't have OI data from Binance)
		isExistingPosition := positionSymbols[coin.Symbol]
		isXyzAsset := market.IsXyzDexAsset(coin.Symbol)
		if !isExistingPosition && !isXyzAsset && data.OpenInterest != nil && data.CurrentPrice > 0 {
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000
			if oiValueInMillions < minOIThresholdMillions {
				logger.Infof("⚠️  %s OI value too low (%.2fM USD < %.1fM), skipping coin",
					coin.Symbol, oiValueInMillions, minOIThresholdMillions)
				continue
			}
		}

		ctx.MarketDataMap[coin.Symbol] = data
	}

	logger.Infof("📊 Successfully fetched multi-timeframe market data for %d coins", len(ctx.MarketDataMap))
	return nil
}

// ============================================================================
// Candidate Coins
// ============================================================================

// GetCandidateCoins gets candidate coins based on strategy configuration
func (e *StrategyEngine) GetCandidateCoins() ([]CandidateCoin, error) {
	var candidates []CandidateCoin

	coinSource := e.config.CoinSource

	switch coinSource.SourceType {
	case "static":
		for _, symbol := range coinSource.StaticCoins {
			symbol = market.Normalize(symbol)
			candidates = append(candidates, CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"static"},
			})
		}

		return e.filterExcludedCoins(candidates), nil

	case "ai500":
		// 检查 use_ai500 标志，如果为 false 则回退到静态币种
		if !coinSource.UseAI500 {
			logger.Infof("⚠️  source_type is 'ai500' but use_ai500 is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getAI500Coins(coinSource.AI500Limit)
		if err != nil {
			logger.Warnf("⚠️ AI500 unavailable, using liquid fallback coins: %v", err)
			return e.fallbackCandidateCoins(), nil
		}
		filtered := e.filterExcludedCoins(coins)
		if len(filtered) == 0 {
			return e.fallbackCandidateCoins(), nil
		}
		return filtered, nil

	case "oi_top":
		// 检查 use_oi_top 标志，如果为 false 则回退到静态币种
		if !coinSource.UseOITop {
			logger.Infof("⚠️  source_type is 'oi_top' but use_oi_top is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getOITopCoins(coinSource.OITopLimit)
		if err != nil {
			logger.Warnf("⚠️ OI Top unavailable, using liquid fallback coins: %v", err)
			return e.fallbackCandidateCoins(), nil
		}
		filtered := e.filterExcludedCoins(coins)
		if len(filtered) == 0 {
			return e.fallbackCandidateCoins(), nil
		}
		return filtered, nil

	case "oi_low":
		// 持仓减少榜，适合做空
		if !coinSource.UseOILow {
			logger.Infof("⚠️  source_type is 'oi_low' but use_oi_low is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getOILowCoins(coinSource.OILowLimit)
		if err != nil {
			logger.Warnf("⚠️ OI Low unavailable, using liquid fallback coins: %v", err)
			return e.fallbackCandidateCoins(), nil
		}
		filtered := e.filterExcludedCoins(coins)
		if len(filtered) == 0 {
			return e.fallbackCandidateCoins(), nil
		}
		return filtered, nil

	case "mixed":
		var rankedLists [][]CandidateCoin
		if coinSource.UseAI500 {
			poolCoins, err := e.getAI500Coins(coinSource.AI500Limit)
			if err != nil {
				logger.Infof("⚠️  Failed to get AI500 coins: %v", err)
			} else {
				rankedLists = append(rankedLists, poolCoins)
			}
		}

		if coinSource.UseOITop {
			oiCoins, err := e.getOITopCoins(coinSource.OITopLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get OI Top: %v", err)
			} else {
				rankedLists = append(rankedLists, oiCoins)
			}
		}

		if coinSource.UseOILow {
			oiLowCoins, err := e.getOILowCoins(coinSource.OILowLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get OI Low: %v", err)
			} else {
				rankedLists = append(rankedLists, oiLowCoins)
			}
		}

		const maxMixedCandidates = 10
		candidates = mergeRankedCandidateLists(rankedLists, maxMixedCandidates)
		filtered := e.filterExcludedCoins(candidates)
		filled := e.topUpWithStaticFallback(filtered, maxMixedCandidates)
		if len(filled) == 0 {
			logger.Warnf("⚠️ Mixed dynamic/static coin sources returned no candidates; using liquid fallback coins")
			return e.fallbackCandidateCoins(), nil
		}
		return filled, nil

	default:
		return nil, fmt.Errorf("unknown coin source type: %s", coinSource.SourceType)
	}
}

// mergeRankedCandidateLists round-robins ranked sources so AI500, OI gainers,
// and OI decliners all remain represented. It also keeps source tags for coins
// that appear in multiple lists and caps prompt size deterministically.
func mergeRankedCandidateLists(lists [][]CandidateCoin, limit int) []CandidateCoin {
	if limit <= 0 {
		return nil
	}

	symbolSources := make(map[string][]string)
	for _, list := range lists {
		for _, coin := range list {
			symbol := market.Normalize(coin.Symbol)
			for _, source := range coin.Sources {
				if !containsString(symbolSources[symbol], source) {
					symbolSources[symbol] = append(symbolSources[symbol], source)
				}
			}
		}
	}

	seen := make(map[string]bool)
	result := make([]CandidateCoin, 0, limit)
	for rank := 0; len(result) < limit; rank++ {
		addedAtRank := false
		for _, list := range lists {
			if rank >= len(list) {
				continue
			}
			addedAtRank = true
			symbol := market.Normalize(list[rank].Symbol)
			if symbol == "" || seen[symbol] {
				continue
			}
			seen[symbol] = true
			result = append(result, CandidateCoin{
				Symbol:  symbol,
				Sources: symbolSources[symbol],
			})
			if len(result) >= limit {
				break
			}
		}
		if !addedAtRank {
			break
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (e *StrategyEngine) topUpWithStaticFallback(candidates []CandidateCoin, limit int) []CandidateCoin {
	if limit <= 0 || len(candidates) >= limit {
		return candidates
	}

	seen := make(map[string]bool, limit)
	result := make([]CandidateCoin, 0, limit)
	for _, coin := range candidates {
		symbol := market.Normalize(coin.Symbol)
		if symbol == "" || seen[symbol] || e.isExcludedSymbol(symbol) {
			continue
		}
		seen[symbol] = true
		coin.Symbol = symbol
		result = append(result, coin)
		if len(result) >= limit {
			return result
		}
	}

	staticSymbols := e.config.CoinSource.StaticCoins
	if len(staticSymbols) == 0 {
		staticSymbols = liquidFallbackSymbols
	}
	for _, symbol := range staticSymbols {
		symbol = market.Normalize(symbol)
		if symbol == "" || seen[symbol] || e.isExcludedSymbol(symbol) {
			continue
		}
		seen[symbol] = true
		result = append(result, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"static_fallback"},
		})
		if len(result) >= limit {
			break
		}
	}

	return result
}

func isLargeCapSymbol(symbol string) bool {
	symbol = market.Normalize(symbol)
	return symbol == "BTCUSDT" || symbol == "ETHUSDT"
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

var liquidFallbackSymbols = []string{
	"XRPUSDT",
	"DOGEUSDT",
	"ADAUSDT",
	"AVAXUSDT",
	"LINKUSDT",
	"SUIUSDT",
	"LTCUSDT",
	"NEARUSDT",
	"UNIUSDT",
	"AAVEUSDT",
}

func (e *StrategyEngine) fallbackCandidateCoins() []CandidateCoin {
	symbols := e.config.CoinSource.StaticCoins
	if len(symbols) == 0 {
		symbols = liquidFallbackSymbols
	}
	const maxFallbackCandidates = 8

	candidates := make([]CandidateCoin, 0, min(len(symbols), maxFallbackCandidates))
	for _, symbol := range symbols {
		candidates = append(candidates, CandidateCoin{
			Symbol:  market.Normalize(symbol),
			Sources: []string{"liquid_fallback"},
		})
		if len(candidates) >= maxFallbackCandidates {
			break
		}
	}
	return e.filterExcludedCoins(candidates)
}

// filterExcludedCoins removes excluded coins from the candidates list
func (e *StrategyEngine) filterExcludedCoins(candidates []CandidateCoin) []CandidateCoin {
	if len(e.config.CoinSource.ExcludedCoins) == 0 {
		return candidates
	}

	// Filter out excluded coins
	filtered := make([]CandidateCoin, 0, len(candidates))
	for _, c := range candidates {
		normalized := market.Normalize(c.Symbol)
		if !e.isExcludedSymbol(normalized) {
			filtered = append(filtered, c)
		} else {
			logger.Infof("🚫 Excluded coin: %s", c.Symbol)
		}
	}

	return filtered
}

func (e *StrategyEngine) isExcludedSymbol(symbol string) bool {
	normalizedSymbol := market.Normalize(symbol)
	for _, coin := range e.config.CoinSource.ExcludedCoins {
		if market.Normalize(coin) == normalizedSymbol {
			return true
		}
	}
	return false
}

func (e *StrategyEngine) getAI500Coins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 30
	}

	symbols, err := e.nofxosClient.GetTopRatedCoins(limit)
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		candidates = append(candidates, CandidateCoin{
			Symbol:  market.Normalize(symbol),
			Sources: []string{"ai500"},
		})
	}
	return candidates, nil
}

func (e *StrategyEngine) getOITopCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 10
	}

	positions, err := e.nofxosClient.GetOITopPositions()
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for i, pos := range positions {
		if i >= limit {
			break
		}
		symbol := market.Normalize(pos.Symbol)
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"oi_top"},
		})
	}
	return candidates, nil
}

func (e *StrategyEngine) getOILowCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 10
	}

	positions, err := e.nofxosClient.GetOILowPositions()
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for i, pos := range positions {
		if i >= limit {
			break
		}
		symbol := market.Normalize(pos.Symbol)
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"oi_low"},
		})
	}
	return candidates, nil
}

// ============================================================================
// External & Quant Data
// ============================================================================

// FetchMarketData fetches market data based on strategy configuration
func (e *StrategyEngine) FetchMarketData(symbol string) (*market.Data, error) {
	return market.Get(symbol)
}

// FetchExternalData fetches external data sources
func (e *StrategyEngine) FetchExternalData() (map[string]interface{}, error) {
	externalData := make(map[string]interface{})

	for _, source := range e.config.Indicators.ExternalDataSources {
		data, err := e.fetchSingleExternalSource(source)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch external data source [%s]: %v", source.Name, err)
			continue
		}
		externalData[source.Name] = data
	}

	return externalData, nil
}

func (e *StrategyEngine) fetchSingleExternalSource(source store.ExternalDataSource) (interface{}, error) {
	// SSRF Protection: Validate URL before making request
	if err := security.ValidateURL(source.URL); err != nil {
		return nil, fmt.Errorf("external source URL validation failed: %w", err)
	}

	timeout := time.Duration(source.RefreshSecs) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Use SSRF-safe HTTP client
	client := security.SafeHTTPClient(timeout)

	req, err := http.NewRequest(source.Method, source.URL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range source.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if source.DataPath != "" {
		result = extractJSONPath(result, source.DataPath)
	}

	return result, nil
}

func extractJSONPath(data interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

// FetchQuantData fetches quantitative data for a single coin
func (e *StrategyEngine) FetchQuantData(symbol string) (*QuantData, error) {
	if !e.config.Indicators.EnableQuantData {
		return nil, nil
	}

	// Use nofxos client with unified API key
	include := "oi,price"
	if e.config.Indicators.EnableQuantNetflow {
		include = "netflow,oi,price"
	}

	nofxosData, err := e.nofxosClient.GetCoinData(symbol, include)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quant data: %w", err)
	}

	if nofxosData == nil {
		return nil, nil
	}

	// Convert nofxos.QuantData to kernel.QuantData
	quantData := &QuantData{
		Symbol:      nofxosData.Symbol,
		Price:       nofxosData.Price,
		PriceChange: nofxosData.PriceChange,
	}

	// Convert OI data
	if nofxosData.OI != nil {
		quantData.OI = make(map[string]*OIData)
		for exchange, oiData := range nofxosData.OI {
			if oiData != nil {
				kData := &OIData{
					CurrentOI: oiData.CurrentOI,
				}
				if oiData.Delta != nil {
					kData.Delta = make(map[string]*OIDeltaData)
					for dur, delta := range oiData.Delta {
						if delta != nil {
							kData.Delta[dur] = &OIDeltaData{
								OIDelta:        delta.OIDelta,
								OIDeltaValue:   delta.OIDeltaValue,
								OIDeltaPercent: delta.OIDeltaPercent,
							}
						}
					}
				}
				quantData.OI[exchange] = kData
			}
		}
	}

	// Convert Netflow data
	if nofxosData.Netflow != nil {
		quantData.Netflow = &NetflowData{}
		if nofxosData.Netflow.Institution != nil {
			quantData.Netflow.Institution = &FlowTypeData{
				Future: nofxosData.Netflow.Institution.Future,
				Spot:   nofxosData.Netflow.Institution.Spot,
			}
		}
		if nofxosData.Netflow.Personal != nil {
			quantData.Netflow.Personal = &FlowTypeData{
				Future: nofxosData.Netflow.Personal.Future,
				Spot:   nofxosData.Netflow.Personal.Spot,
			}
		}
	}

	return quantData, nil
}

// FetchQuantDataBatch batch fetches quantitative data
func (e *StrategyEngine) FetchQuantDataBatch(symbols []string) map[string]*QuantData {
	result := make(map[string]*QuantData)

	if !e.config.Indicators.EnableQuantData {
		return result
	}

	for _, symbol := range symbols {
		data, err := e.FetchQuantData(symbol)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch quantitative data for %s: %v", symbol, err)
			continue
		}
		if data != nil {
			result[symbol] = data
		}
	}

	return result
}

// FetchOIRankingData fetches market-wide OI ranking data
func (e *StrategyEngine) FetchOIRankingData() *nofxos.OIRankingData {
	indicators := e.config.Indicators
	if !indicators.EnableOIRanking {
		return nil
	}

	duration := indicators.OIRankingDuration
	if duration == "" {
		duration = "1h"
	}

	limit := indicators.OIRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("📊 Fetching OI ranking data (duration: %s, limit: %d)", duration, limit)

	data, err := e.nofxosClient.GetOIRanking(duration, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch OI ranking data: %v", err)
		return nil
	}

	logger.Infof("✓ OI ranking data ready: %d top, %d low positions",
		len(data.TopPositions), len(data.LowPositions))

	return data
}

// FetchNetFlowRankingData fetches market-wide NetFlow ranking data
func (e *StrategyEngine) FetchNetFlowRankingData() *nofxos.NetFlowRankingData {
	indicators := e.config.Indicators
	if !indicators.EnableNetFlowRanking {
		return nil
	}

	duration := indicators.NetFlowRankingDuration
	if duration == "" {
		duration = "1h"
	}

	limit := indicators.NetFlowRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("💰 Fetching NetFlow ranking data (duration: %s, limit: %d)", duration, limit)

	data, err := e.nofxosClient.GetNetFlowRanking(duration, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch NetFlow ranking data: %v", err)
		return nil
	}

	logger.Infof("✓ NetFlow ranking data ready: inst_in=%d, inst_out=%d, retail_in=%d, retail_out=%d",
		len(data.InstitutionFutureTop), len(data.InstitutionFutureLow),
		len(data.PersonalFutureTop), len(data.PersonalFutureLow))

	return data
}

// FetchPriceRankingData fetches market-wide price ranking data (gainers/losers)
func (e *StrategyEngine) FetchPriceRankingData() *nofxos.PriceRankingData {
	indicators := e.config.Indicators
	if !indicators.EnablePriceRanking {
		return nil
	}

	durations := indicators.PriceRankingDuration
	if durations == "" {
		durations = "1h"
	}

	limit := indicators.PriceRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("📈 Fetching Price ranking data (durations: %s, limit: %d)", durations, limit)

	data, err := e.nofxosClient.GetPriceRanking(durations, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch Price ranking data: %v", err)
		return nil
	}

	logger.Infof("✓ Price ranking data ready for %d durations", len(data.Durations))

	return data
}

// ============================================================================
// Prompt Building - System Prompt
// ============================================================================

// BuildSystemPrompt builds System Prompt according to strategy configuration
func (e *StrategyEngine) BuildSystemPrompt(accountEquity float64, variant string) string {
	var sb strings.Builder
	riskControl := e.config.RiskControl
	promptSections := e.config.PromptSections

	// 0. Compact data guide (replaces verbose schema dictionary to save tokens)
	sb.WriteString("# Key Data Guide\n\n")
	sb.WriteString("- **PnL%**: Unrealized profit/loss including leverage effect. +5% at 3x leverage = price moved +1.67%\n")
	sb.WriteString("- **Peak PnL%**: Historical max PnL for this position. Compare with current PnL to detect drawdown.\n")
	sb.WriteString("- **OI (Open Interest)**: OI up + Price up = strong bullish. OI up + Price down = strong bearish. OI down = positions closing.\n")
	sb.WriteString("- **Funding Rate**: High positive = expensive to long (crowded). Negative = expensive to short.\n")
	sb.WriteString("- **Institutional Netflow**: Positive = smart money buying. Negative = smart money selling. Most reliable signal.\n")
	sb.WriteString("- **EMA20/50**: Price > EMA20 = short-term uptrend. EMA20 > EMA50 = medium-term bullish.\n")
	sb.WriteString("- **RSI**: <30 oversold (bounce likely), >70 overbought (pullback likely), 40-60 = neutral zone.\n")
	sb.WriteString("- **MACD**: Above signal = bullish momentum. Below signal = bearish momentum.\n")
	sb.WriteString("- **ATR**: Average True Range = volatility measure. Use ATR*1.5 for stop-loss distance.\n")
	sb.WriteString("- **BOLL**: Price near upper band = overbought. Near lower band = oversold. Squeeze = breakout imminent.\n\n")
	sb.WriteString("---\n\n")

	// 1. Role definition (editable)
	if promptSections.RoleDefinition != "" {
		sb.WriteString(promptSections.RoleDefinition)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# Expert Crypto Futures Trader\n\n")
		sb.WriteString("You are a disciplined prop-trader AI. Your edge: reading price action + order flow (OI, funding, institutional netflow) to find high-probability setups. You think in risk/reward, not predictions. Capital preservation is your #1 priority. You evaluate ALL coins equally.\n\n")
	}

	// 2. Trading mode variant
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥ 70\n- Allow higher positions, but must strictly set stop-loss and explain risk-reward ratio\n\n")
	case "conservative":
		sb.WriteString("## Mode: Conservative\n- Only open positions when multiple signals resonate\n- Prioritize cash preservation, must pause for multiple periods after consecutive losses\n\n")
	case "scalping":
		sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, smaller profit targets but require quick action\n- If price doesn't move as expected within two bars, immediately reduce position or stop-loss\n\n")
	}

	// 3. Hard constraints (risk control)
	btcEthPosValueRatio := riskControl.BTCETHMaxPositionValueRatio
	if btcEthPosValueRatio <= 0 {
		btcEthPosValueRatio = 5.0
	}
	altcoinPosValueRatio := riskControl.AltcoinMaxPositionValueRatio
	if altcoinPosValueRatio <= 0 {
		altcoinPosValueRatio = 1.0
	}

	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString("## CODE ENFORCED (Backend validation, cannot be bypassed):\n")
	sb.WriteString(fmt.Sprintf("- Max Positions: %d coins simultaneously\n", riskControl.MaxPositions))
	sb.WriteString(fmt.Sprintf("- Position Value Limit (Altcoins): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*altcoinPosValueRatio, accountEquity, altcoinPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Position Value Limit (BTC/ETH): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*btcEthPosValueRatio, accountEquity, btcEthPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Max Margin Usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
	effectiveMinPosition := riskControl.MinPositionSize
	if effectiveMinPosition <= 0 {
		effectiveMinPosition = 5
	}
	sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT notional\n\n", effectiveMinPosition))

	sb.WriteString("## AI GUIDED (Recommended, you should follow):\n")
	sb.WriteString(fmt.Sprintf("- Trading Leverage: Altcoins max %dx | BTC/ETH max %dx\n",
		riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
	sb.WriteString(fmt.Sprintf("- Risk-Reward Ratio: ≥1:%.1f (take_profit / stop_loss)\n", riskControl.MinRiskRewardRatio))
	sb.WriteString(fmt.Sprintf("- Min Confidence: ≥%d to open position\n\n", riskControl.MinConfidence))

	sb.WriteString("## Directional Neutrality (Long/Short Balance)\n")
	sb.WriteString("- You must evaluate both LONG and SHORT scenarios for every candidate; do not default to only one side.\n")
	sb.WriteString("- Balance means no directional bias, not forced 50/50 trading frequency. Choose the side with current edge, or wait.\n")
	sb.WriteString("- Avoid new BTCUSDT/ETHUSDT entries in this profile unless they are already open and need management; recent live performance on large caps is poor.\n")
	sb.WriteString("- Overbought RSI/Bollinger makes LONG lower quality and raises pullback/rejection risk; it is NOT enough to short unless there is a clean bearish trigger and valid RR.\n")
	sb.WriteString("- Oversold RSI/Bollinger makes SHORT lower quality and raises bounce risk; it is NOT enough to long unless there is a clean bullish trigger and valid RR.\n")
	sb.WriteString("- If most candidates are overbought on 15m, prioritize: avoid chasing longs, look for confirmed rejection/breakdown short setups, otherwise wait.\n")
	sb.WriteString("- If most candidates are oversold on 15m, prioritize: avoid chasing shorts, look for confirmed reclaim/failed-breakdown long setups, otherwise wait.\n\n")

	// Position sizing guidance. Size is driven by loss-at-stop, not by the
	// largest notional the account is technically allowed to open.
	maxRiskPct := riskControl.MaxRiskPerTradePct
	if maxRiskPct <= 0 || maxRiskPct > 0.05 {
		maxRiskPct = 0.009
	}
	riskBudgetUSD := accountEquity * maxRiskPct
	sb.WriteString("## Position Sizing Guidance\n")
	sb.WriteString(fmt.Sprintf("- Maximum loss at stop for this cycle: %.2f USDT (= %.2f equity × %.2f%%).\n",
		riskBudgetUSD, accountEquity, maxRiskPct*100))
	sb.WriteString("- Compute stop_distance_pct = abs(entry - stop_loss) / entry.\n")
	sb.WriteString("- Compute risk-sized notional = allowed_risk_usd / stop_distance_pct.\n")
	sb.WriteString("- `position_size_usd` is NOTIONAL value, not margin and not available balance.\n")
	if accountEquity >= 100 && accountEquity <= 160 {
		sb.WriteString("- Binance displays margin. At 3x leverage, 25-30 USDT margin means about 75-90 USDT notional.\n")
		sb.WriteString("- Preferred new-trade notional for this account: 75-90 USDT; 40 USDT is the hard executable floor. If risk sizing falls below 75, prefer a tighter-stop setup next cycle.\n")
	}
	sb.WriteString("- Confidence 68-71 starter/probe: use about 40-60% of the risk budget only when the 5m trigger is clean; 72-84: 60-85%; 85+: 85-100%.\n")
	sb.WriteString("- Final size must be the smallest of: risk-sized notional, position-value limit, and margin limit.\n")
	if riskBudgetUSD > 0 {
		sb.WriteString(fmt.Sprintf("- For reference, a 1.5%% stop permits at most %.0f USDT notional; a 2%% stop %.0f; a 3%% stop %.0f.\n",
			riskBudgetUSD/0.015, riskBudgetUSD/0.02, riskBudgetUSD/0.03))
	}
	sb.WriteString("- Never enlarge size merely to create more trading activity.\n\n")

	// 4. Trading frequency (editable)
	if promptSections.TradingFrequency != "" {
		sb.WriteString(promptSections.TradingFrequency)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# Trading Discipline\n\n")
		sb.WriteString("- Quality over quantity. 1-3 trades per day is ideal.\n")
		sb.WriteString("- Only trade when data gives a CLEAR signal with multiple confirmations.\n")
		sb.WriteString("- \"wait\" is your best trade when uncertain. Never force entries.\n\n")
	}

	// 5. Entry standards (editable)
	if promptSections.EntryStandards != "" {
		sb.WriteString(promptSections.EntryStandards)
		sb.WriteString("\n\nYou have the following indicator data:\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\n**Confidence ≥ %d** required to open positions.\n\n", riskControl.MinConfidence))
	} else {
		sb.WriteString("# Entry Checklist (Strict)\n\n")
		sb.WriteString("**LONG setup** (need multiple confirmations):\n")
		sb.WriteString("- Price > EMA20 on 1h (uptrend confirmed)\n")
		sb.WriteString("- RSI14 between 40-65 (room to run, not overbought)\n")
		sb.WriteString("- OI increasing + price rising\n")
		sb.WriteString("- Institutional netflow positive\n\n")
		sb.WriteString("**SHORT setup** (need multiple confirmations):\n")
		sb.WriteString("- Price < EMA20 on 1h (downtrend confirmed)\n")
		sb.WriteString("- RSI14 between 35-60 (room to fall, not oversold)\n")
		sb.WriteString("- OI increasing + price dropping\n")
		sb.WriteString("- Institutional netflow negative\n\n")
		sb.WriteString("**EXIT rules**:\n")
		sb.WriteString("- Close if PnL < -3%\n")
		sb.WriteString("- Close 50% at +3% profit\n")
		sb.WriteString("- Close all if drawdown from Peak PnL > 40%\n\n")
		sb.WriteString("You have the following indicator data:\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\n**Confidence ≥ %d** required to open positions.\n\n", riskControl.MinConfidence))
	}

	// 6. Decision process (editable)
	if promptSections.DecisionProcess != "" {
		sb.WriteString(promptSections.DecisionProcess)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# Decision Steps\n\n")
		sb.WriteString("1. Check existing positions: any need to SL/TP/exit?\n")
		sb.WriteString("2. Scan ALL candidate coins equally. NEVER fixate on BTC/ETH.\n")
		sb.WriteString("3. For each candidate, compare LONG edge vs SHORT edge vs WAIT using trend, trigger, exhaustion, and RR.\n")
		sb.WriteString("4. Pick BEST 1-2 setups only. Never force trades.\n")
		sb.WriteString("5. Output one compact JSON decision array only.\n\n")
	}

	// 7. Custom Prompt
	if e.config.CustomPrompt != "" {
		sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
		sb.WriteString(e.config.CustomPrompt)
		sb.WriteString("\n\n")
		sb.WriteString("Note: The above personalized strategy is a supplement to the basic rules and cannot violate the basic risk control principles.\n\n")
	}

	// 8. Output format. Keep this LAST so smaller/free models do not drift into prose.
	sb.WriteString("# Output Format (Strictly Follow)\n\n")
	sb.WriteString("You are in MACHINE OUTPUT MODE.\n")
	sb.WriteString("Return only one raw JSON object. Do not write analysis, markdown, XML tags, headings, apologies, or explanations outside JSON.\n")
	sb.WriteString("If there is no trade, return one wait decision.\n\n")
	sb.WriteString("## Format Requirements\n\n")
	sb.WriteString("Valid response shape:\n\n")
	sb.WriteString("{\"decisions\":[{\"symbol\":\"ALL\",\"action\":\"wait\",\"leverage\":0,\"position_size_usd\":0,\"stop_loss\":0,\"take_profit\":0,\"confidence\":60,\"risk_usd\":0,\"reasoning\":\"No executable trigger yet\"}]}\n")
	sb.WriteString("## Field Description\n\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString(fmt.Sprintf("- `confidence`: 0-100 (opening recommended ≥ %d)\n", riskControl.MinConfidence))
	sb.WriteString("- Required when opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n")
	sb.WriteString(fmt.Sprintf("- For opening decisions, `risk_usd` must not exceed %.2f for this cycle.\n", riskBudgetUSD))
	sb.WriteString("- Required for every item: symbol, action, reasoning. Keep reasoning under 160 characters.\n")
	sb.WriteString("- **IMPORTANT**: All numeric values must be calculated numbers, NOT formulas/expressions (e.g., use `27.76` not `3000 * 0.01`)\n")
	sb.WriteString("- **CRITICAL**: The symbols in the JSON above are EXAMPLES. Do not copy them unless the data supports them. Scan ALL Candidate Coins equally.\n")
	sb.WriteString("- **FINAL CHECK BEFORE ANSWERING**: Output the JSON object only, with no surrounding text.\n")

	return sb.String()
}

func (e *StrategyEngine) writeAvailableIndicators(sb *strings.Builder) {
	indicators := e.config.Indicators
	kline := indicators.Klines

	sb.WriteString(fmt.Sprintf("- %s price series", kline.PrimaryTimeframe))
	if kline.EnableMultiTimeframe {
		sb.WriteString(fmt.Sprintf(" + %s K-line series\n", kline.LongerTimeframe))
	} else {
		sb.WriteString("\n")
	}

	if indicators.EnableEMA {
		sb.WriteString("- EMA indicators")
		if len(indicators.EMAPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.EMAPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableMACD {
		sb.WriteString("- MACD indicators\n")
	}

	if indicators.EnableRSI {
		sb.WriteString("- RSI indicators")
		if len(indicators.RSIPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.RSIPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableATR {
		sb.WriteString("- ATR indicators")
		if len(indicators.ATRPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.ATRPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableBOLL {
		sb.WriteString("- Bollinger Bands (BOLL) - Upper/Middle/Lower bands")
		if len(indicators.BOLLPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.BOLLPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableVolume {
		sb.WriteString("- Volume data\n")
	}

	if indicators.EnableOI {
		sb.WriteString("- Open Interest (OI) data\n")
	}

	if indicators.EnableFundingRate {
		sb.WriteString("- Funding rate\n")
	}

	if len(e.config.CoinSource.StaticCoins) > 0 || e.config.CoinSource.UseAI500 || e.config.CoinSource.UseOITop {
		sb.WriteString("- AI500 / OI_Top filter tags (if available)\n")
	}

	if indicators.EnableQuantData {
		sb.WriteString("- Quantitative data (institutional/retail fund flow, position changes, multi-period price changes)\n")
	}
}

// ============================================================================
// Prompt Building - User Prompt
// ============================================================================

// BuildUserPrompt builds User Prompt based on strategy configuration
func (e *StrategyEngine) BuildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// System status
	sb.WriteString(fmt.Sprintf("Time: %s | Period: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC market
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// Account information
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | PnL %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// Recently completed orders (placed before positions to ensure visibility)
	if len(ctx.RecentOrders) > 0 {
		sb.WriteString("## Recent Completed Trades\n")
		recentLosses := 0
		recentBTCETHLosses := 0
		lastLossSymbols := make([]string, 0, 3)
		for i, order := range ctx.RecentOrders {
			resultStr := "Profit"
			if order.RealizedPnL < 0 {
				resultStr = "Loss"
				if i < 5 {
					recentLosses++
				}
				if i < 10 && isLargeCapSymbol(order.Symbol) {
					recentBTCETHLosses++
				}
				if len(lastLossSymbols) < 3 {
					lastLossSymbols = append(lastLossSymbols, market.Normalize(order.Symbol))
				}
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Exit %.4f | %s: %+.2f USDT (%+.2f%%) | %s→%s (%s)\n",
				i+1, order.Symbol, order.Side,
				order.EntryPrice, order.ExitPrice,
				resultStr, order.RealizedPnL, order.PnLPct,
				order.EntryTime, order.ExitTime, order.HoldDuration))
		}
		sb.WriteString("Memory rules from recent trades:\n")
		sb.WriteString("- Treat these recent trades as live memory. Do not immediately re-enter a symbol that just lost unless the new setup is materially different and stronger.\n")
		if len(lastLossSymbols) > 0 {
			sb.WriteString(fmt.Sprintf("- Recently losing symbols to be extra careful with: %s.\n", strings.Join(uniqueStrings(lastLossSymbols), ", ")))
		}
		if recentLosses >= 3 {
			sb.WriteString("- Recent loss cluster detected: require cleaner trigger, prefer wait, and do not use full risk budget.\n")
		}
		if recentBTCETHLosses > 0 {
			sb.WriteString("- BTC/ETH recent losses detected: avoid new BTC/ETH entries; focus on altcoin candidates or wait.\n")
		}
		sb.WriteString("\n")
	}

	// Historical trading statistics (helps AI understand past performance)
	if ctx.TradingStats != nil && ctx.TradingStats.TotalTrades > 0 {
		// Get language from strategy config
		lang := e.GetLanguage()

		// Win/Loss ratio
		var winLossRatio float64
		if ctx.TradingStats.AvgLoss > 0 {
			winLossRatio = ctx.TradingStats.AvgWin / ctx.TradingStats.AvgLoss
		}

		if lang == LangChinese {
			sb.WriteString("## 历史交易统计\n")
			sb.WriteString(fmt.Sprintf("总交易: %d 笔 | 盈利因子: %.2f | 夏普比率: %.2f | 盈亏比: %.2f\n",
				ctx.TradingStats.TotalTrades,
				ctx.TradingStats.ProfitFactor,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			sb.WriteString(fmt.Sprintf("总盈亏: %+.2f USDT | 平均盈利: +%.2f | 平均亏损: -%.2f | 最大回撤: %.1f%%\n",
				ctx.TradingStats.TotalPnL,
				ctx.TradingStats.AvgWin,
				ctx.TradingStats.AvgLoss,
				ctx.TradingStats.MaxDrawdownPct))

			// Performance hints based on profit factor, sharpe, and drawdown
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("表现: 良好 - 保持当前策略\n")
			} else if ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("表现: 需改进 - 提高盈亏比，优化止盈止损\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("表现: 风险偏高 - 减少仓位，控制回撤\n")
			} else {
				sb.WriteString("表现: 正常 - 有优化空间\n")
			}
		} else {
			sb.WriteString("## Historical Trading Statistics\n")
			sb.WriteString(fmt.Sprintf("Total Trades: %d | Profit Factor: %.2f | Sharpe: %.2f | Win/Loss Ratio: %.2f\n",
				ctx.TradingStats.TotalTrades,
				ctx.TradingStats.ProfitFactor,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			sb.WriteString(fmt.Sprintf("Total PnL: %+.2f USDT | Avg Win: +%.2f | Avg Loss: -%.2f | Max Drawdown: %.1f%%\n",
				ctx.TradingStats.TotalPnL,
				ctx.TradingStats.AvgWin,
				ctx.TradingStats.AvgLoss,
				ctx.TradingStats.MaxDrawdownPct))

			// Performance hints based on profit factor, sharpe, and drawdown
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("Performance: GOOD - maintain current strategy\n")
			} else if ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("Performance: NEEDS IMPROVEMENT - improve win/loss ratio, optimize TP/SL\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("Performance: HIGH RISK - reduce position size, control drawdown\n")
			} else {
				sb.WriteString("Performance: NORMAL - room for optimization\n")
			}
		}
		sb.WriteString("\n")
	}

	// Position information
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(e.formatPositionInfo(i+1, pos, ctx))
		}
	} else {
		sb.WriteString("Current Positions: None\n\n")
	}

	// Candidate coins (exclude coins already in positions to avoid duplicate data)
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		// Normalize symbol to handle both "ETH" and "ETHUSDT" formats
		normalizedSymbol := market.Normalize(pos.Symbol)
		positionSymbols[normalizedSymbol] = true
	}

	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d coins)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		// Skip if this coin is already a position (data already shown in positions section)
		normalizedCoinSymbol := market.Normalize(coin.Symbol)
		if positionSymbols[normalizedCoinSymbol] {
			continue
		}

		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := e.formatCoinSourceTag(coin.Sources)
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[coin.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Get language for market data formatting
	nofxosLang := nofxos.LangEnglish
	if e.GetLanguage() == LangChinese {
		nofxosLang = nofxos.LangChinese
	}

	// OI Ranking data (market-wide open interest changes)
	if ctx.OIRankingData != nil {
		sb.WriteString(nofxos.FormatOIRankingForAI(ctx.OIRankingData, nofxosLang))
	}

	// NetFlow Ranking data (market-wide fund flow)
	if ctx.NetFlowRankingData != nil {
		sb.WriteString(nofxos.FormatNetFlowRankingForAI(ctx.NetFlowRankingData, nofxosLang))
	}

	// Price Ranking data (market-wide gainers/losers)
	if ctx.PriceRankingData != nil {
		sb.WriteString(nofxos.FormatPriceRankingForAI(ctx.PriceRankingData, nofxosLang))
	}

	sb.WriteString("---\n\n")
	sb.WriteString("Now please analyze and output your decision (Chain of Thought + JSON)\n")

	return sb.String()
}

func (e *StrategyEngine) formatPositionInfo(index int, pos PositionInfo, ctx *Context) string {
	var sb strings.Builder

	holdingDuration := ""
	if pos.UpdateTime > 0 {
		durationMs := time.Now().UnixMilli() - pos.UpdateTime
		durationMin := durationMs / (1000 * 60)
		if durationMin < 60 {
			holdingDuration = fmt.Sprintf(" | Holding Duration %d min", durationMin)
		} else {
			durationHour := durationMin / 60
			durationMinRemainder := durationMin % 60
			holdingDuration = fmt.Sprintf(" | Holding Duration %dh %dm", durationHour, durationMinRemainder)
		}
	}

	positionValue := pos.Quantity * pos.MarkPrice
	if positionValue < 0 {
		positionValue = -positionValue
	}

	sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | Qty %.4f | Position Value %.2f USDT | PnL%+.2f%% | PnL Amount%+.2f USDT | Peak PnL%.2f%% | Leverage %dx | Margin %.0f | Liq Price %.4f%s\n\n",
		index, pos.Symbol, strings.ToUpper(pos.Side),
		pos.EntryPrice, pos.MarkPrice, pos.Quantity, positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
		pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

	if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[pos.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (e *StrategyEngine) formatCoinSourceTag(sources []string) string {
	if len(sources) > 1 {
		// 多信号源组合
		hasAI500 := false
		hasOITop := false
		hasOILow := false
		for _, s := range sources {
			switch s {
			case "ai500":
				hasAI500 = true
			case "oi_top":
				hasOITop = true
			case "oi_low":
				hasOILow = true
			}
		}
		if hasAI500 && hasOITop {
			return " (AI500+OI_Top dual signal)"
		}
		if hasAI500 && hasOILow {
			return " (AI500+OI_Low dual signal)"
		}
		if hasOITop && hasOILow {
			return " (OI_Top+OI_Low)"
		}
		return " (Multiple sources)"
	} else if len(sources) == 1 {
		switch sources[0] {
		case "ai500":
			return " (AI500)"
		case "oi_top":
			return " (OI_Top 持仓增加)"
		case "oi_low":
			return " (OI_Low 持仓减少)"
		case "static":
			return " (Manual selection)"
		}
	}
	return ""
}

// ============================================================================
// Market Data Formatting
// ============================================================================

func (e *StrategyEngine) formatMarketData(data *market.Data) string {
	var sb strings.Builder
	indicators := e.config.Indicators

	// 明确标注币种
	sb.WriteString(fmt.Sprintf("=== %s Market Data ===\n\n", data.Symbol))
	sb.WriteString(fmt.Sprintf("current_price = %.4f", data.CurrentPrice))

	if indicators.EnableEMA {
		sb.WriteString(fmt.Sprintf(", current_ema20 = %.3f", data.CurrentEMA20))
	}

	if indicators.EnableMACD {
		sb.WriteString(fmt.Sprintf(", current_macd = %.3f", data.CurrentMACD))
	}

	if indicators.EnableRSI {
		sb.WriteString(fmt.Sprintf(", current_rsi7 = %.3f", data.CurrentRSI7))
	}

	sb.WriteString("\n\n")

	// Entry Quality Metrics - helps AI avoid FOMO entries (only RSI check, BOLL shown in timeframe data)
	if indicators.EnableRSI && data.CurrentRSI7 > 0 {
		rsiStatus := "✅ NEUTRAL"
		if data.CurrentRSI7 > 70 {
			rsiStatus = "⚠️ OVERBOUGHT (avoid LONG)"
		} else if data.CurrentRSI7 < 30 {
			rsiStatus = "⚠️ OVERSOLD (avoid SHORT)"
		} else if data.CurrentRSI7 > 60 {
			rsiStatus = "⚠️ HIGH (caution LONG)"
		} else if data.CurrentRSI7 < 40 {
			rsiStatus = "⚠️ LOW (caution SHORT)"
		}
		sb.WriteString(fmt.Sprintf("📊 Entry Quality: RSI7 = %.1f %s\n\n", data.CurrentRSI7, rsiStatus))
	}

	if indicators.EnableOI || indicators.EnableFundingRate {
		sb.WriteString(fmt.Sprintf("Additional data for %s:\n\n", data.Symbol))

		if indicators.EnableOI && data.OpenInterest != nil {
			sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f\n\n",
				data.OpenInterest.Latest, data.OpenInterest.Average))
		}

		if indicators.EnableFundingRate {
			sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))
		}
	}

	if len(data.TimeframeData) > 0 {
		timeframeOrder := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}
		for _, tf := range timeframeOrder {
			if tfData, ok := data.TimeframeData[tf]; ok {
				sb.WriteString(fmt.Sprintf("=== %s Timeframe (oldest → latest) ===\n\n", strings.ToUpper(tf)))
				e.formatTimeframeSeriesData(&sb, tfData, indicators)
			}
		}
	} else {
		// Compatible with old data format
		if data.IntradaySeries != nil {
			klineConfig := indicators.Klines
			sb.WriteString(fmt.Sprintf("Intraday series (%s intervals, oldest → latest):\n\n", klineConfig.PrimaryTimeframe))

			if len(data.IntradaySeries.MidPrices) > 0 {
				sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
			}

			if indicators.EnableEMA && len(data.IntradaySeries.EMA20Values) > 0 {
				sb.WriteString(fmt.Sprintf("EMA indicators (20-period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
			}

			if indicators.EnableMACD && len(data.IntradaySeries.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
			}

			if indicators.EnableRSI {
				if len(data.IntradaySeries.RSI7Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (7-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
				}
				if len(data.IntradaySeries.RSI14Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
				}
			}

			if indicators.EnableVolume && len(data.IntradaySeries.Volume) > 0 {
				sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3m ATR (14-period): %.3f\n\n", data.IntradaySeries.ATR14))
			}
		}

		if data.LongerTermContext != nil && indicators.Klines.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf("Longer-term context (%s timeframe):\n\n", indicators.Klines.LongerTimeframe))

			if indicators.EnableEMA {
				sb.WriteString(fmt.Sprintf("20-Period EMA: %.3f vs. 50-Period EMA: %.3f\n\n",
					data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3-Period ATR: %.3f vs. 14-Period ATR: %.3f\n\n",
					data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))
			}

			if indicators.EnableVolume {
				sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
					data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))
			}

			if indicators.EnableMACD && len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
			}

			if indicators.EnableRSI && len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
			}
		}
	}

	return sb.String()
}

func (e *StrategyEngine) formatTimeframeSeriesData(sb *strings.Builder, data *market.TimeframeSeriesData, indicators store.IndicatorConfig) {
	if len(data.Klines) > 0 {
		sb.WriteString("Recent candles (oldest → latest, compact):\n")
		klines := tailKlines(data.Klines, compactKlineLimit(data.Timeframe))
		for i, k := range klines {
			t := time.Unix(k.Time/1000, 0).UTC()
			timeStr := t.Format("01-02 15:04")
			marker := ""
			if i == len(klines)-1 {
				marker = "  <- current"
			}
			sb.WriteString(fmt.Sprintf("%s O %.4f H %.4f L %.4f C %.4f V %.0f%s\n",
				timeStr, k.Open, k.High, k.Low, k.Close, k.Volume, marker))
		}
		sb.WriteString("\n")
	} else if len(data.MidPrices) > 0 {
		sb.WriteString(fmt.Sprintf("Recent mid prices: %s\n\n", formatRecentFloatSlice(data.MidPrices, 8)))
		if indicators.EnableVolume && len(data.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Recent volume: %s\n\n", formatRecentFloatSlice(data.Volume, 8)))
		}
	}

	if indicators.EnableEMA {
		ema20 := lastFloat(data.EMA20Values)
		ema50 := lastFloat(data.EMA50Values)
		if ema20 > 0 || ema50 > 0 {
			sb.WriteString(fmt.Sprintf("EMA latest: EMA20 %.4f | EMA50 %.4f\n", ema20, ema50))
		}
	}

	if indicators.EnableMACD && len(data.MACDValues) > 0 {
		sb.WriteString(fmt.Sprintf("MACD recent: %s\n", formatRecentFloatSlice(data.MACDValues, 5)))
	}

	if indicators.EnableRSI {
		rsi7 := lastFloat(data.RSI7Values)
		rsi14 := lastFloat(data.RSI14Values)
		if rsi7 > 0 || rsi14 > 0 {
			sb.WriteString(fmt.Sprintf("RSI latest: RSI7 %.1f | RSI14 %.1f\n", rsi7, rsi14))
		}
	}

	if indicators.EnableATR && data.ATR14 > 0 {
		sb.WriteString(fmt.Sprintf("ATR14: %.4f\n", data.ATR14))
	}

	if indicators.EnableBOLL && len(data.BOLLUpper) > 0 {
		upper := lastFloat(data.BOLLUpper)
		middle := lastFloat(data.BOLLMiddle)
		lower := lastFloat(data.BOLLLower)
		sb.WriteString(fmt.Sprintf("BOLL latest: upper %.4f | middle %.4f | lower %.4f\n", upper, middle, lower))
	}

	sb.WriteString("\n")
}

func compactKlineLimit(timeframe string) int {
	switch strings.ToLower(strings.TrimSpace(timeframe)) {
	case "5m":
		return 10
	case "15m":
		return 12
	case "1h", "4h":
		return 8
	default:
		return 8
	}
}

func tailKlines(values []market.KlineBar, limit int) []market.KlineBar {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[len(values)-limit:]
}

func lastFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}

func formatRecentFloatSlice(values []float64, limit int) string {
	if limit > 0 && len(values) > limit {
		values = values[len(values)-limit:]
	}
	return formatFloatSlice(values)
}

func (e *StrategyEngine) formatQuantData(data *QuantData) string {
	if data == nil {
		return ""
	}

	indicators := e.config.Indicators
	if !indicators.EnableQuantOI && !indicators.EnableQuantNetflow {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s Quantitative Data:\n", data.Symbol))

	if len(data.PriceChange) > 0 {
		sb.WriteString("Price Change: ")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}
		parts := []string{}
		for _, tf := range timeframes {
			if v, ok := data.PriceChange[tf]; ok {
				parts = append(parts, fmt.Sprintf("%s: %+.4f%%", tf, v*100))
			}
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteString("\n")
	}

	if indicators.EnableQuantNetflow && data.Netflow != nil {
		sb.WriteString("Fund Flow (Netflow):\n")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}

		if data.Netflow.Institution != nil {
			if len(data.Netflow.Institution.Future) > 0 {
				sb.WriteString("  Institutional Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if len(data.Netflow.Institution.Spot) > 0 {
				sb.WriteString("  Institutional Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}

		if data.Netflow.Personal != nil {
			if len(data.Netflow.Personal.Future) > 0 {
				sb.WriteString("  Retail Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if len(data.Netflow.Personal.Spot) > 0 {
				sb.WriteString("  Retail Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}
	}

	if indicators.EnableQuantOI && len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			if len(oiData.Delta) > 0 {
				sb.WriteString(fmt.Sprintf("Open Interest (%s):\n", exchange))
				for _, tf := range []string{"5m", "15m", "1h", "4h", "12h", "24h"} {
					if d, ok := oiData.Delta[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %+.4f%% (%s)\n", tf, d.OIDeltaPercent, formatFlowValue(d.OIDeltaValue)))
					}
				}
			}
		}
	}

	return sb.String()
}

func formatFlowValue(v float64) string {
	sign := ""
	if v >= 0 {
		sign = "+"
	}
	absV := v
	if absV < 0 {
		absV = -absV
	}
	if absV >= 1e9 {
		return fmt.Sprintf("%s%.2fB", sign, v/1e9)
	} else if absV >= 1e6 {
		return fmt.Sprintf("%s%.2fM", sign, v/1e6)
	} else if absV >= 1e3 {
		return fmt.Sprintf("%s%.2fK", sign, v/1e3)
	}
	return fmt.Sprintf("%s%.2f", sign, v)
}

func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.4f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// ============================================================================
// AI Response Parsing
// ============================================================================

func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) (*FullDecision, error) {
	cotTrace := extractCoTTrace(aiResponse)

	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("decision validation failed: %w", err)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

func isInvalidJSONSafeWaitDecision(decision *FullDecision) bool {
	if decision == nil || len(decision.Decisions) != 1 {
		return false
	}

	d := decision.Decisions[0]
	return strings.EqualFold(d.Symbol, "ALL") &&
		strings.EqualFold(d.Action, "wait") &&
		d.Confidence == 0 &&
		strings.Contains(d.Reasoning, "Model didn't output valid JSON decision")
}

func appendDecisionFormatRetryPrompt(userPrompt string, attempt int) string {
	return userPrompt + fmt.Sprintf(`

# OUTPUT FORMAT RETRY %d
Your previous answer was invalid or truncated because analysis appeared before the JSON.
Decide silently. Do not repeat market analysis.
Return exactly one JSON object and nothing else:
{"decisions":[{"symbol":"ALL","action":"wait","leverage":0,"position_size_usd":0,"stop_loss":0,"take_profit":0,"confidence":60,"risk_usd":0,"reasoning":"No executable setup after scanning candidates"}]}
Do not explain the rules. Do not write analysis. Do not use markdown.`, attempt)
}

func extractCoTTrace(response string) string {
	if match := reReasoningTag.FindStringSubmatch(response); len(match) > 1 {
		logger.Infof("✓ Extracted reasoning chain using <reasoning> tag")
		return strings.TrimSpace(match[1])
	}

	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		logger.Infof("✓ Extracted content before <decision> tag as reasoning chain")
		return strings.TrimSpace(response[:decisionIdx])
	}

	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		logger.Infof("⚠️  Extracted reasoning chain using old format ([ character separator)")
		return strings.TrimSpace(response[:jsonStart])
	}

	return strings.TrimSpace(response)
}

func extractDecisions(response string) ([]Decision, error) {
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)
	s = fixMissingQuotes(s)

	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		logger.Infof("✓ Extracted JSON using <decision> tag")
	} else {
		jsonPart = s
		logger.Infof("⚠️  <decision> tag not found, searching JSON in full text")
	}

	jsonPart = fixMissingQuotes(jsonPart)

	if decisions, ok := parseDecisionJSONCandidate(jsonPart); ok {
		return decisions, nil
	}

	if m := reJSONFence.FindStringSubmatch(jsonPart); len(m) > 1 {
		if decisions, ok := parseDecisionJSONCandidate(m[1]); ok {
			return decisions, nil
		}
	}

	for _, jsonContent := range reJSONDecisionObject.FindAllString(jsonPart, -1) {
		if decisions, ok := parseDecisionJSONCandidate(jsonContent); ok {
			return decisions, nil
		}
	}

	for _, jsonContent := range reJSONArray.FindAllString(jsonPart, -1) {
		if decisions, ok := parseDecisionJSONCandidate(jsonContent); ok {
			return decisions, nil
		}
	}

	// Some free OpenRouter models (notably Gemma-style instruct models) often
	// write a complete natural-language "no trade" conclusion and then start a
	// fenced JSON block that gets truncated, e.g. ```json {"decisions":.
	// In that case the valid signal is in the full response, not in the broken
	// JSON fragment. Inspect both before falling back to the generic invalid-JSON
	// wait marker, so the executor receives a normal wait decision.
	for _, analysisCandidate := range []string{jsonPart, s} {
		if waitDecision, ok := inferExplicitWaitFromAnalysis(analysisCandidate); ok {
			logger.Infof("✓ Model analysis explicitly concluded no trade; converting to valid wait decision")
			return []Decision{waitDecision}, nil
		}
	}

	if waitDecision, ok := inferExplicitWaitFromAnalysis(response); ok {
		logger.Infof("✓ Model analysis explicitly concluded no trade; converting to valid wait decision")
		return []Decision{waitDecision}, nil
	}

	return safeWaitDecision(jsonPart), nil
}

func inferExplicitWaitFromAnalysis(response string) (Decision, bool) {
	lower := strings.ToLower(strings.TrimSpace(response))
	if lower == "" {
		return Decision{}, false
	}

	hasConclusion := strings.Contains(lower, "conclusion:") ||
		strings.Contains(lower, "**conclusion") ||
		strings.Contains(lower, "final conclusion") ||
		strings.Contains(lower, "recommendation:")

	explicitNoTradePhrases := []string{
		"no candidate currently presents",
		"no candidate presents",
		"no candidate meets",
		"no candidate currently meets",
		"no candidate met",
		"no candidate has an executable",
		"no candidate has a clean",
		"no candidate qualifies",
		"no executable setup",
		"no executable trigger",
		"no clear trigger",
		"no confirmed reversal trigger",
		"no high-confidence setup",
		"no high confidence setup",
		"minimum confidence threshold",
		"risk-reward ratio cannot be safely established",
		"cannot be safely established",
		"best action is to wait",
		"recommendation: wait",
	}
	hasExplicitNoTrade := false
	for _, phrase := range explicitNoTradePhrases {
		if strings.Contains(lower, phrase) {
			hasExplicitNoTrade = true
			break
		}
	}

	// Avoid treating a restatement of prompt rules as a market decision. Either
	// require a conclusion section or an unambiguous direct wait recommendation.
	directWait := strings.Contains(lower, "best action is to wait") ||
		strings.Contains(lower, "recommendation: wait")
	if !hasExplicitNoTrade || (!hasConclusion && !directWait) {
		return Decision{}, false
	}

	return Decision{
		Symbol:     "ALL",
		Action:     "wait",
		Confidence: 60,
		Reasoning:  "No executable trigger found in model analysis",
	}, true
}

func isAnalysisDerivedWait(decision Decision) bool {
	return strings.EqualFold(decision.Symbol, "ALL") &&
		strings.EqualFold(decision.Action, "wait") &&
		decision.Reasoning == "No executable trigger found in model analysis"
}

func parseDecisionJSONCandidate(jsonContent string) ([]Decision, bool) {
	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent)

	if strings.HasPrefix(strings.TrimSpace(jsonContent), "{") {
		var wrapped struct {
			Decisions []Decision `json:"decisions"`
		}
		if err := json.Unmarshal([]byte(jsonContent), &wrapped); err != nil {
			logger.Warnf("⚠️  Ignoring unparsable decision JSON object: %v | content: %s", err, jsonContent[:min(160, len(jsonContent))])
			return nil, false
		}
		if wrapped.Decisions != nil {
			return wrapped.Decisions, true
		}

		var single Decision
		if err := json.Unmarshal([]byte(jsonContent), &single); err == nil && single.Symbol != "" && single.Action != "" {
			return []Decision{single}, true
		}

		logger.Warnf("⚠️  Ignoring decision JSON object without decisions array or single decision | content: %s", jsonContent[:min(160, len(jsonContent))])
		return nil, false
	}

	if err := validateJSONFormat(jsonContent); err != nil {
		logger.Warnf("⚠️  Ignoring invalid decision JSON candidate: %v | content: %s", err, jsonContent[:min(160, len(jsonContent))])
		return nil, false
	}

	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		logger.Warnf("⚠️  Ignoring unparsable decision JSON candidate: %v | content: %s", err, jsonContent[:min(160, len(jsonContent))])
		return nil, false
	}

	return decisions, true
}

func safeWaitDecision(contextText string) []Decision {
	logger.Infof("⚠️  [SafeFallback] AI didn't output valid JSON decision, entering safe wait mode")

	cotSummary := strings.TrimSpace(contextText)
	if len(cotSummary) > 240 {
		cotSummary = cotSummary[:240] + "..."
	}
	if cotSummary == "" {
		cotSummary = "empty or invalid model response"
	}

	return []Decision{{
		Symbol:     "ALL",
		Action:     "wait",
		Confidence: 0,
		Reasoning:  fmt.Sprintf("Model didn't output valid JSON decision, entering safe wait; summary: %s", cotSummary),
	}}
}

func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")

	jsonStr = strings.ReplaceAll(jsonStr, "［", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{")
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}")
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":")
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "【", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "　", " ")

	return jsonStr
}

func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	if !reArrayHead.MatchString(trimmed) {
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	if strings.Contains(jsonStr, "~") {
		return fmt.Errorf("JSON cannot contain range symbol ~, all numbers must be precise single values")
	}

	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			return fmt.Errorf("JSON numbers cannot contain thousand separator comma, found: %s", jsonStr[i:min(i+10, len(jsonStr))])
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// ============================================================================
// Decision Validation
// ============================================================================

func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) error {
	for i := range decisions {
		if err := validateDecision(&decisions[i], accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio); err != nil {
			return fmt.Errorf("decision #%d validation failed: %w", i+1, err)
		}
	}
	return nil
}

func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) error {
	validActions := map[string]bool{
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("invalid action: %s", d.Action)
	}

	if d.Action == "open_long" || d.Action == "open_short" {
		maxLeverage := altcoinLeverage
		posRatio := altcoinPosRatio
		maxPositionValue := accountEquity * posRatio
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage
			posRatio = btcEthPosRatio
			maxPositionValue = accountEquity * posRatio
		}

		if d.Leverage <= 0 {
			return fmt.Errorf("leverage must be greater than 0: %d", d.Leverage)
		}
		if d.Leverage > maxLeverage {
			logger.Infof("⚠️  [Leverage Fallback] %s leverage exceeded (%dx > %dx), auto-adjusting to limit %dx",
				d.Symbol, d.Leverage, maxLeverage, maxLeverage)
			d.Leverage = maxLeverage
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("position size must be greater than 0: %.2f", d.PositionSizeUSD)
		}

		const minPositionSizeGeneral = 5.0
		const minPositionSizeBTCETH = 5.0

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			if d.PositionSizeUSD < minPositionSizeBTCETH {
				return fmt.Errorf("%s opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
			}
		} else {
			if d.PositionSizeUSD < minPositionSizeGeneral {
				return fmt.Errorf("opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.PositionSizeUSD, minPositionSizeGeneral)
			}
		}

		tolerance := maxPositionValue * 0.05 // Support 5% overflow for safety
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			logger.Infof("⚠️  [Size Fallback] %s position value exceeded (%.1f > %.1f), auto-adjusting to limit %.1f",
				d.Symbol, d.PositionSizeUSD, maxPositionValue, maxPositionValue)
			d.PositionSizeUSD = maxPositionValue
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("stop loss and take profit must be greater than 0")
		}

		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("for long positions, stop loss price must be less than take profit price")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("for short positions, stop loss price must be greater than take profit price")
			}
		}

		var entryPrice float64
		if d.Action == "open_long" {
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2
		} else {
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		const minParseRiskRewardRatio = 1.5
		const parseRiskRewardEpsilon = 1e-6
		if riskRewardRatio+parseRiskRewardEpsilon < minParseRiskRewardRatio {
			return fmt.Errorf("risk/reward ratio too low (%.4f:1), must be ≥%.4f:1 [risk: %.2f%% reward: %.2f%%] [stop loss: %.2f take profit: %.2f]",
				riskRewardRatio, minParseRiskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// detectLanguage detects language from text content
// Returns LangChinese if text contains Chinese characters, otherwise LangEnglish
func detectLanguage(text string) Language {
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			return LangChinese
		}
	}
	return LangEnglish
}
