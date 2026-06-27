package store

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"strings"
	"time"

	"gorm.io/gorm"
)

// StrategyStore strategy storage
type StrategyStore struct {
	db *gorm.DB
}

// Strategy strategy configuration
type Strategy struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	UserID        string    `gorm:"column:user_id;not null;default:'';index" json:"user_id"`
	Name          string    `gorm:"not null" json:"name"`
	Description   string    `gorm:"default:''" json:"description"`
	IsActive      bool      `gorm:"column:is_active;default:false;index" json:"is_active"`
	IsDefault     bool      `gorm:"column:is_default;default:false" json:"is_default"`
	IsPublic      bool      `gorm:"column:is_public;default:false;index" json:"is_public"`    // whether visible in strategy market
	ConfigVisible bool      `gorm:"column:config_visible;default:true" json:"config_visible"` // whether config details are visible
	Config        string    `gorm:"not null;default:'{}'" json:"config"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Strategy) TableName() string { return "strategies" }

// StrategyConfig strategy configuration details (JSON structure)
type StrategyConfig struct {
	// Strategy type: "ai_trading" (default) or "grid_trading"
	StrategyType string `json:"strategy_type,omitempty"`

	// language setting: "zh" for Chinese, "en" for English
	// This determines the language used for data formatting and prompt generation
	Language string `json:"language,omitempty"`
	// coin source configuration
	CoinSource CoinSourceConfig `json:"coin_source"`
	// quantitative data configuration
	Indicators IndicatorConfig `json:"indicators"`
	// custom prompt (appended at the end)
	CustomPrompt string `json:"custom_prompt,omitempty"`
	// risk control configuration
	RiskControl RiskControlConfig `json:"risk_control"`
	// editable sections of System Prompt
	PromptSections PromptSectionsConfig `json:"prompt_sections,omitempty"`

	// Grid trading configuration (only used when StrategyType == "grid_trading")
	GridConfig *GridStrategyConfig `json:"grid_config,omitempty"`
}

// GridStrategyConfig grid trading specific configuration
type GridStrategyConfig struct {
	// Trading pair (e.g., "BTCUSDT")
	Symbol string `json:"symbol"`
	// Number of grid levels (5-50)
	GridCount int `json:"grid_count"`
	// Total investment in USDT
	TotalInvestment float64 `json:"total_investment"`
	// Leverage (1-20)
	Leverage int `json:"leverage"`
	// Upper price boundary (0 = auto-calculate from ATR)
	UpperPrice float64 `json:"upper_price"`
	// Lower price boundary (0 = auto-calculate from ATR)
	LowerPrice float64 `json:"lower_price"`
	// Use ATR to auto-calculate bounds
	UseATRBounds bool `json:"use_atr_bounds"`
	// ATR multiplier for bound calculation (default 2.0)
	ATRMultiplier float64 `json:"atr_multiplier"`
	// Position distribution: "uniform" | "gaussian" | "pyramid"
	Distribution string `json:"distribution"`
	// Maximum drawdown percentage before emergency exit
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	// Stop loss percentage per position
	StopLossPct float64 `json:"stop_loss_pct"`
	// Daily loss limit percentage
	DailyLossLimitPct float64 `json:"daily_loss_limit_pct"`
	// Use maker-only orders for lower fees
	UseMakerOnly bool `json:"use_maker_only"`
	// Enable automatic grid direction adjustment based on box breakouts
	EnableDirectionAdjust bool `json:"enable_direction_adjust"`
	// Direction bias ratio for long_bias/short_bias modes (default 0.7 = 70%/30%)
	DirectionBiasRatio float64 `json:"direction_bias_ratio"`
}

// PromptSectionsConfig editable sections of System Prompt
type PromptSectionsConfig struct {
	// role definition (title + description)
	RoleDefinition string `json:"role_definition,omitempty"`
	// trading frequency awareness
	TradingFrequency string `json:"trading_frequency,omitempty"`
	// entry standards
	EntryStandards string `json:"entry_standards,omitempty"`
	// decision process
	DecisionProcess string `json:"decision_process,omitempty"`
}

// CoinSourceConfig coin source configuration
type CoinSourceConfig struct {
	// source type: "static" | "ai500" | "oi_top" | "oi_low" | "mixed"
	SourceType string `json:"source_type"`
	// static coin list (used when source_type = "static")
	StaticCoins []string `json:"static_coins,omitempty"`
	// excluded coins list (filtered out from all sources)
	ExcludedCoins []string `json:"excluded_coins,omitempty"`
	// whether to use AI500 coin pool
	UseAI500 bool `json:"use_ai500"`
	// AI500 coin pool maximum count
	AI500Limit int `json:"ai500_limit,omitempty"`
	// whether to use OI Top (持仓增加榜，适合做多)
	UseOITop bool `json:"use_oi_top"`
	// OI Top maximum count
	OITopLimit int `json:"oi_top_limit,omitempty"`
	// whether to use OI Low (持仓减少榜，适合做空)
	UseOILow bool `json:"use_oi_low"`
	// OI Low maximum count
	OILowLimit int `json:"oi_low_limit,omitempty"`
	// whether to use Hyperliquid All coins (all available perp pairs)
	UseHyperAll bool `json:"use_hyper_all"`
	// whether to use Hyperliquid Main coins (top N by 24h volume)
	UseHyperMain bool `json:"use_hyper_main"`
	// Hyperliquid Main maximum count (default 20)
	HyperMainLimit int `json:"hyper_main_limit,omitempty"`
	// Note: API URLs are now built automatically using NofxOSAPIKey from IndicatorConfig
}

// IndicatorConfig indicator configuration
type IndicatorConfig struct {
	// K-line configuration
	Klines KlineConfig `json:"klines"`
	// raw kline data (OHLCV) - always enabled, required for AI analysis
	EnableRawKlines bool `json:"enable_raw_klines"`
	// technical indicator switches
	EnableEMA         bool `json:"enable_ema"`
	EnableMACD        bool `json:"enable_macd"`
	EnableRSI         bool `json:"enable_rsi"`
	EnableATR         bool `json:"enable_atr"`
	EnableBOLL        bool `json:"enable_boll"` // Bollinger Bands
	EnableVolume      bool `json:"enable_volume"`
	EnableOI          bool `json:"enable_oi"`           // open interest
	EnableFundingRate bool `json:"enable_funding_rate"` // funding rate
	// EMA period configuration
	EMAPeriods []int `json:"ema_periods,omitempty"` // default [20, 50]
	// RSI period configuration
	RSIPeriods []int `json:"rsi_periods,omitempty"` // default [7, 14]
	// ATR period configuration
	ATRPeriods []int `json:"atr_periods,omitempty"` // default [14]
	// BOLL period configuration (period, standard deviation multiplier is fixed at 2)
	BOLLPeriods []int `json:"boll_periods,omitempty"` // default [20] - can select multiple timeframes
	// external data sources
	ExternalDataSources []ExternalDataSource `json:"external_data_sources,omitempty"`

	// ========== NofxOS Unified API Configuration ==========
	// Unified API Key for all NofxOS data sources
	NofxOSAPIKey string `json:"nofxos_api_key,omitempty"`

	// quantitative data sources (capital flow, position changes, price changes)
	EnableQuantData    bool `json:"enable_quant_data"`    // whether to enable quantitative data
	EnableQuantOI      bool `json:"enable_quant_oi"`      // whether to show OI data
	EnableQuantNetflow bool `json:"enable_quant_netflow"` // whether to show Netflow data

	// OI ranking data (market-wide open interest increase/decrease rankings)
	EnableOIRanking   bool   `json:"enable_oi_ranking"`             // whether to enable OI ranking data
	OIRankingDuration string `json:"oi_ranking_duration,omitempty"` // duration: 1h, 4h, 24h
	OIRankingLimit    int    `json:"oi_ranking_limit,omitempty"`    // number of entries (default 10)

	// NetFlow ranking data (market-wide fund flow rankings - institution/personal)
	EnableNetFlowRanking   bool   `json:"enable_netflow_ranking"`             // whether to enable NetFlow ranking data
	NetFlowRankingDuration string `json:"netflow_ranking_duration,omitempty"` // duration: 1h, 4h, 24h
	NetFlowRankingLimit    int    `json:"netflow_ranking_limit,omitempty"`    // number of entries (default 10)

	// Price ranking data (market-wide gainers/losers)
	EnablePriceRanking   bool   `json:"enable_price_ranking"`             // whether to enable price ranking data
	PriceRankingDuration string `json:"price_ranking_duration,omitempty"` // durations: "1h" or "1h,4h,24h"
	PriceRankingLimit    int    `json:"price_ranking_limit,omitempty"`    // number of entries per ranking (default 10)
}

// KlineConfig K-line configuration
type KlineConfig struct {
	// primary timeframe: "1m", "3m", "5m", "15m", "1h", "4h"
	PrimaryTimeframe string `json:"primary_timeframe"`
	// primary timeframe K-line count
	PrimaryCount int `json:"primary_count"`
	// longer timeframe
	LongerTimeframe string `json:"longer_timeframe,omitempty"`
	// longer timeframe K-line count
	LongerCount int `json:"longer_count,omitempty"`
	// whether to enable multi-timeframe analysis
	EnableMultiTimeframe bool `json:"enable_multi_timeframe"`
	// selected timeframe list (new: supports multi-timeframe selection)
	SelectedTimeframes []string `json:"selected_timeframes,omitempty"`
}

// ExternalDataSource external data source configuration
type ExternalDataSource struct {
	Name        string            `json:"name"`   // data source name
	Type        string            `json:"type"`   // type: "api" | "webhook"
	URL         string            `json:"url"`    // API URL
	Method      string            `json:"method"` // HTTP method
	Headers     map[string]string `json:"headers,omitempty"`
	DataPath    string            `json:"data_path,omitempty"`    // JSON data path
	RefreshSecs int               `json:"refresh_secs,omitempty"` // refresh interval (seconds)
}

// RiskControlConfig risk control configuration
type RiskControlConfig struct {
	// Max number of coins held simultaneously (CODE ENFORCED)
	MaxPositions int `json:"max_positions"`

	// BTC/ETH exchange leverage for opening positions (AI guided)
	BTCETHMaxLeverage int `json:"btc_eth_max_leverage"`
	// Altcoin exchange leverage for opening positions (AI guided)
	AltcoinMaxLeverage int `json:"altcoin_max_leverage"`

	// BTC/ETH single position max value = equity × this ratio (CODE ENFORCED, default: 5)
	BTCETHMaxPositionValueRatio float64 `json:"btc_eth_max_position_value_ratio"`
	// Altcoin single position max value = equity × this ratio (CODE ENFORCED, default: 1)
	AltcoinMaxPositionValueRatio float64 `json:"altcoin_max_position_value_ratio"`

	// Max margin utilization (e.g. 0.9 = 90%) (CODE ENFORCED)
	MaxMarginUsage float64 `json:"max_margin_usage"`
	// Min position size in USDT (CODE ENFORCED)
	MinPositionSize float64 `json:"min_position_size"`

	// Min take_profit / stop_loss ratio (AI guided)
	MinRiskRewardRatio float64 `json:"min_risk_reward_ratio"`
	// Min AI confidence to open position (AI guided)
	MinConfidence int `json:"min_confidence"`
	// Max loss at stop-loss as a fraction of equity (e.g. 0.008 = 0.8%) (CODE ENFORCED)
	MaxRiskPerTradePct float64 `json:"max_risk_per_trade_pct"`
}

// NewStrategyStore creates a new StrategyStore
func NewStrategyStore(db *gorm.DB) *StrategyStore {
	return &StrategyStore{db: db}
}

func (s *StrategyStore) initTables() error {
	// AutoMigrate will add missing columns without dropping existing data
	return s.db.AutoMigrate(&Strategy{})
}

func (s *StrategyStore) initDefaultData() error {
	enConfig := GetDefaultStrategyConfig("en")
	enConfigJSON, _ := json.Marshal(enConfig)

	var existing Strategy
	if err := s.db.Where("id = ?", "system_default_en").First(&existing).Error; err == nil {
		logger.Infof("🌱 Updating system default strategy to balanced small-account defaults...")
		return s.db.Model(&Strategy{}).
			Where("id = ?", "system_default_en").
			Updates(map[string]interface{}{
				"name":        "Nofx AI Balanced Small Account",
				"description": "Balanced strategy for small USDT futures accounts using Binance market data and strict risk caps",
				"config":      string(enConfigJSON),
			}).Error
	}

	var count int64
	s.db.Model(&Strategy{}).Where("is_default = ?", true).Count(&count)
	if count > 0 {
		return nil
	}

	logger.Infof("🌱 Initializing system default strategy...")

	// Create system default strategy (English)
	defaultStrategy := &Strategy{
		ID:            "system_default_en",
		UserID:        "", // Global
		Name:          "Nofx AI Balanced Small Account",
		Description:   "Balanced strategy for small USDT futures accounts using Binance market data and strict risk caps",
		IsActive:      true,
		IsDefault:     true,
		IsPublic:      true,
		ConfigVisible: true,
		Config:        string(enConfigJSON),
	}

	if err := s.db.Create(defaultStrategy).Error; err != nil {
		return fmt.Errorf("failed to create default strategy: %w", err)
	}

	return nil
}

func defaultLiquidSmallAccountCoins() []string {
	return []string{
		"XRPUSDT", "DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT",
		"SUIUSDT", "LTCUSDT", "NEARUSDT", "UNIUSDT", "AAVEUSDT",
	}
}

// ApplySmallAccountBalancedMode converts overly strict or deprecated-data
// strategies into a practical small-account futures profile at runtime.
// It intentionally keeps the hard safety rails: up to three independent
// positions, low leverage, mandatory SL/TP validation, margin cap, and capped
// risk per trade.
func ApplySmallAccountBalancedMode(config *StrategyConfig) {
	if config == nil {
		return
	}

	if len(config.CoinSource.StaticCoins) == 0 {
		config.CoinSource.StaticCoins = defaultLiquidSmallAccountCoins()
	}
	config.CoinSource.StaticCoins = removeStringItems(config.CoinSource.StaticCoins, "BTCUSDT", "ETHUSDT")
	if len(config.CoinSource.StaticCoins) == 0 {
		config.CoinSource.StaticCoins = defaultLiquidSmallAccountCoins()
	}
	config.CoinSource.StaticCoins = ensureStringItems(
		config.CoinSource.StaticCoins,
		defaultLiquidSmallAccountCoins()...,
	)
	config.CoinSource.ExcludedCoins = ensureStringItems(
		config.CoinSource.ExcludedCoins,
		"BTCUSDT", "ETHUSDT",
	)
	config.CoinSource.SourceType = "mixed"
	config.CoinSource.UseAI500 = true
	config.CoinSource.UseOITop = true
	config.CoinSource.UseOILow = true

	// Keep candidate discovery in mixed mode, but do not make the AI depend on
	// separate NoFx quantitative/ranking payloads. The mixed coin source already
	// has a static safety net when dynamic sources are unavailable.
	config.Indicators.EnableQuantData = false
	config.Indicators.EnableQuantOI = false
	config.Indicators.EnableQuantNetflow = false
	config.Indicators.EnableOIRanking = false
	config.Indicators.EnableNetFlowRanking = false
	config.Indicators.EnablePriceRanking = false
	config.Indicators.Klines.SelectedTimeframes = ensureTimeframes(
		config.Indicators.Klines.SelectedTimeframes,
		"5m", "15m", "1h", "4h",
	)
	if config.Indicators.Klines.PrimaryCount <= 0 || config.Indicators.Klines.PrimaryCount > 24 {
		config.Indicators.Klines.PrimaryCount = 24
	}
	if config.Indicators.Klines.LongerCount <= 0 || config.Indicators.Klines.LongerCount > 8 {
		config.Indicators.Klines.LongerCount = 8
	}
	if config.CoinSource.AI500Limit <= 0 || config.CoinSource.AI500Limit > 8 {
		config.CoinSource.AI500Limit = 8
	}
	if config.CoinSource.OITopLimit <= 0 || config.CoinSource.OITopLimit > 6 {
		config.CoinSource.OITopLimit = 6
	}
	if config.CoinSource.OILowLimit <= 0 || config.CoinSource.OILowLimit > 6 {
		config.CoinSource.OILowLimit = 6
	}

	rc := &config.RiskControl
	rc.MaxPositions = 3
	if rc.BTCETHMaxLeverage <= 0 || rc.BTCETHMaxLeverage > 3 {
		rc.BTCETHMaxLeverage = 3
	}
	if rc.AltcoinMaxLeverage <= 0 || rc.AltcoinMaxLeverage > 3 {
		rc.AltcoinMaxLeverage = 3
	}
	if rc.BTCETHMaxPositionValueRatio <= 0 || rc.BTCETHMaxPositionValueRatio > 0.75 {
		rc.BTCETHMaxPositionValueRatio = 0.75
	}
	if rc.AltcoinMaxPositionValueRatio <= 0 || rc.AltcoinMaxPositionValueRatio > 1.0 {
		rc.AltcoinMaxPositionValueRatio = 1.0
	}
	if rc.MaxMarginUsage <= 0 || rc.MaxMarginUsage > 0.75 {
		rc.MaxMarginUsage = 0.75
	}
	// Binance shows margin, while position_size_usd is notional. Target
	// 90-120 USDT notional, but keep the executable floor lower so risk-capped
	// trades with valid stops are not blocked.
	rc.MinPositionSize = 75
	if rc.MinRiskRewardRatio <= 0 || rc.MinRiskRewardRatio < 1.8 {
		rc.MinRiskRewardRatio = 1.8
	}
	if rc.MinConfidence <= 0 || rc.MinConfidence < 70 {
		rc.MinConfidence = 70
	}
	rc.MaxRiskPerTradePct = 0.009

	config.CustomPrompt = balancedSmallAccountCustomPrompt()
	config.PromptSections = balancedSmallAccountPromptSections()
}

func balancedSmallAccountCustomPrompt() string {
	return `# EXECUTION PRIORITIES
- Trade this as a balanced small-account USDT futures strategy for about 130 USDT equity.
- The account may hold up to 3 simultaneous positions, but only when the setups are independent and total margin remains controlled.
- Use mixed candidate discovery: prioritize AI500/OI Top/OI Low coins when available; use the static liquid list only as a safety fallback when dynamic sources are empty or unavailable.
- Avoid opening new BTCUSDT/ETHUSDT trades for now because recent live performance on large caps has been poor. Manage/close existing BTC/ETH positions if present, but prefer altcoin setups for new entries.
- Use Binance price/indicator data as primary for the final decision. Missing NoFx quantitative/ranking payloads are not by themselves a reason to wait.
- Pick clean executable setups; do not require every signal to agree perfectly, but do require a real trigger, nearby invalidation, and fee-adjusted reward.
- Stay directionally neutral: evaluate long and short independently; do not keep trading only one side.
- Overbought 15m conditions reduce long quality and may create short-watch conditions only after rejection/breakdown confirmation. Oversold conditions mirror this for shorts/longs.
- Recent live history shows shorts have underperformed after fees. Do not force shorts; require cleaner rejection/breakdown confirmation for shorts than for neutral waits.
- Never chase obvious exhaustion, never average down, never martingale.
- No mandatory holding period: close on stop, take profit, invalidation, structure break, or risk reduction. However, avoid closing just for tiny noise; fees matter.
- Do not close a position in the first 15 minutes just because the setup now looks less attractive; early exits require explicit stop-loss, take-profit, liquidation, structure break, or invalidation.

# POSITION AND MARGIN BUDGET
- Maximum 3 simultaneous positions.
- Use 3x leverage. Binance displays margin; decision ` + "`position_size_usd`" + ` is notional.
- Preferred new-trade margin is 30-40 USDT, equal to about 90-120 USDT notional at 3x.
- With 130 USDT equity, prefer 30-35 USDT margin per position when planning for 3 positions. Use 40 USDT margin only for the cleanest setup or when fewer than 2 positions are open.
- Risk-capped probes from 75-90 USDT notional are allowed when the stop is wider but the trigger, RR, and fees are still acceptable.
- Total projected margin usage should normally stay below 75% of equity. Do not open a third position if it pushes projected margin above 75%.
- Avoid correlated duplicate exposure: do not open multiple positions that depend on the same market move unless each setup has its own trigger and stop.

# RISK BUDGET
- Maximum loss at stop: about 0.9% of current equity per trade, and preferably 0.5-0.7% when there are already open positions.
- For 130 USDT equity, 0.9% risk is about 1.17 USDT. At 90-120 USDT notional, this implies the stop usually needs to be within about 1.0-1.3%.
- If a valid stop is wider than about 1.3%, wait or use a smaller probe only if exchange minimums and RR are still valid. Do not force 90-120 USDT notional into a wide-stop setup.
- Starter/probe entries are allowed at confidence 70-72 only when the 5m trigger is clear and the stop is nearby; use about 0.45-0.60% equity risk.
- Require at least 1.8:1 real reward/risk at the current market price.
- Expected take profit should be large enough to beat taker fees and noise; avoid trades where the likely gross profit is less than about 0.45 USDT on a 90 USDT notional position.
- Confidence must be at least 70 for an opening trade.

# RESPONSE
- Output only a raw JSON object with a decisions array. No prose, no markdown, no XML tags.`
}

func balancedSmallAccountPromptSections() PromptSectionsConfig {
	return PromptSectionsConfig{
		RoleDefinition: `# Balanced Small-Account Futures Trader

Trade a small USDT futures account pragmatically. Your job is not to wait for a perfect institutional-grade setup; your job is to take clear, controlled opportunities with very small size, mandatory stop loss, and realistic take profit.`,
		TradingFrequency: `# Trading Discipline

- Target 1-3 reasonable attempts per active day, not forced entries every cycle.
- Maximum 3 open positions at once.
- Prefer 1 strong position over 3 mediocre positions; add second/third positions only when setups are independent and projected margin remains below the limit.
- Do not average down, martingale, or open correlated duplicate trades.
- If the market is flat/choppy with no visible edge, wait; otherwise select the cleanest candidate instead of requiring every external signal to agree.
- Exit on stop, take profit, invalidation, structure break, or risk reduction. Avoid closing profitable positions for tiny noise when fees would consume the edge.
- Do not close a position in the first 15 minutes just because the setup now looks less attractive; early exits require explicit stop-loss, take-profit, liquidation, structure break, or invalidation.`,
		EntryStandards: `# Entry Standards

Use mixed candidate discovery: prioritize coins that come from AI500/OI Top/OI Low when available, and use the static liquid list only as a safety net when dynamic sources are empty or unavailable. Use Binance market data as the primary source for the final trade decision.

Directional balance:
- For every candidate, compare LONG edge, SHORT edge, and WAIT. Do not assume the market should only be traded in one direction.
- Balance means no directional bias, not forced equal long/short frequency.
- Recent live history shows shorts have underperformed and fees have hurt very short holds. Do not force shorts; require cleaner rejection/breakdown confirmation for shorts than for neutral waits.
- If many candidates are overbought on 15m (RSI7 > 70 or near upper Bollinger), avoid chasing longs; consider shorts only after a confirmed rejection, failed breakout, breakdown, or lower-high structure with valid stop/RR.
- If many candidates are oversold on 15m (RSI7 < 30 or near lower Bollinger), avoid chasing shorts; consider longs only after reclaim, failed breakdown, bounce, or higher-low structure with valid stop/RR.
- Avoid new BTC/ETH entries unless the strategy is explicitly changed; recent live results on large caps are poor. Existing BTC/ETH positions may still be managed/closed normally.

Open only when these four gates are acceptable:
1. Direction: 1h and 4h are regime filters, not automatic vetoes. A mixed higher-timeframe regime is allowed when 15m location and 5m trigger are clean.
2. Trigger/location: 15m defines the setup; 5m confirms entry with a breakout close, EMA20 reclaim, pullback bounce, failed breakdown, or rejection near EMA/Bollinger/structure. Do not chase obvious exhaustion.
3. Risk: stop loss is on the correct side of structure/current price, take profit gives at least 1.8R, confidence is at least 70, and new-trade notional is normally 90-120 USDT unless explicitly reduced by risk to 75-90 USDT.
4. Portfolio: no more than 3 open positions, no correlated duplicates, and projected total margin normally below 75% equity.

Funding/OI can improve or reduce confidence, but missing external quantitative data is not a blocker. Prefer dynamic-sourced candidates when quality is similar, but never force a trade solely because a coin appeared in a ranking.`,
		DecisionProcess: `# Decision Process

1. Manage open risk first. Close or reduce positions on stop, invalidation, structure break, or when the remaining upside no longer justifies risk and fees.
2. Count open positions and projected margin before opening anything. Maximum 3 positions; projected total margin should normally stay below 75% equity.
3. Score each candidate separately for LONG, SHORT, and WAIT using trend, 5m/15m trigger, RSI/Bollinger location, volatility, nearby invalidation, fee-adjusted target, and correlation with existing exposure.
4. Penalize long setups when 15m RSI7 > 70 unless there is a fresh reclaim/pullback continuation with room to target. Penalize short setups when 15m RSI7 < 30 unless there is a clean continuation breakdown.
5. If at least one candidate has a clean trigger and executable risk, choose the best setup. Multiple opens are allowed only when each setup independently meets the gates and total margin remains controlled.
6. Use 3x max leverage. Size from stop distance and a maximum 0.9% equity loss at stop; ` + "`position_size_usd`" + ` is notional, not Binance margin. Preferred new-trade notional is 90-120 USDT (about 30-40 USDT margin at 3x). Allow 75-90 USDT only when risk sizing requires it. Use 90-105 USDT when planning for three positions; use 105-120 USDT only for stronger single/second positions with tight stops.
7. Return wait when no candidate has an executable trigger, the stop/target cannot be placed safely, the likely profit is too small after taker fees, or another position would overuse margin.`,
	}
}

func ensureTimeframes(existing []string, required ...string) []string {
	seen := make(map[string]bool, len(existing)+len(required))
	result := make([]string, 0, len(existing)+len(required))
	for _, timeframe := range append(existing, required...) {
		timeframe = strings.TrimSpace(timeframe)
		if timeframe == "" || seen[timeframe] {
			continue
		}
		seen[timeframe] = true
		result = append(result, timeframe)
	}
	return result
}

func ensureStringItems(existing []string, required ...string) []string {
	seen := make(map[string]bool, len(existing)+len(required))
	result := make([]string, 0, len(existing)+len(required))
	for _, value := range append(required, existing...) {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func removeStringItems(existing []string, blocked ...string) []string {
	blockedSet := make(map[string]bool, len(blocked))
	for _, value := range blocked {
		blockedSet[strings.ToUpper(strings.TrimSpace(value))] = true
	}

	result := make([]string, 0, len(existing))
	seen := make(map[string]bool, len(existing))
	for _, value := range existing {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" || blockedSet[value] || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

// GetDefaultStrategyConfig returns the default strategy configuration for the given language
func GetDefaultStrategyConfig(lang string) StrategyConfig {
	// Normalize language to "zh" or "en"
	normalizedLang := "en"
	if lang == "zh" {
		normalizedLang = "zh"
	}

	config := StrategyConfig{
		Language: normalizedLang,
		CoinSource: CoinSourceConfig{
			SourceType:  "mixed",
			UseAI500:    true,
			AI500Limit:  12,
			UseOITop:    true,
			OITopLimit:  8,
			UseOILow:    true,
			OILowLimit:  8,
			StaticCoins: defaultLiquidSmallAccountCoins(),
		},
		Indicators: IndicatorConfig{
			Klines: KlineConfig{
				PrimaryTimeframe:     "15m",
				PrimaryCount:         24,
				LongerTimeframe:      "4h",
				LongerCount:          8,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"5m", "15m", "1h", "4h"},
			},
			EnableRawKlines:   true, // Required - raw OHLCV data for AI analysis
			EnableEMA:         true, // EMA20/50 for trend direction
			EnableMACD:        true, // MACD for momentum confirmation
			EnableRSI:         true, // RSI7/14 for overbought/oversold
			EnableATR:         true, // ATR for volatility & SL/TP placement
			EnableBOLL:        true, // Bollinger Bands for range & mean reversion
			EnableVolume:      true,
			EnableOI:          true,
			EnableFundingRate: true,
			EMAPeriods:        []int{20, 50},
			RSIPeriods:        []int{7, 14},
			ATRPeriods:        []int{14},
			BOLLPeriods:       []int{20},
			// NofxOS unified API key
			NofxOSAPIKey: "cm_568c67eae410d912c54c",
			// Quant data from the bundled public NoFx key is currently unavailable.
			// Keep these off by default so the AI does not wait for missing data.
			EnableQuantData:    false,
			EnableQuantOI:      false,
			EnableQuantNetflow: false,
			// OI ranking data
			EnableOIRanking:   false,
			OIRankingDuration: "1h",
			OIRankingLimit:    10,
			// NetFlow ranking data
			EnableNetFlowRanking:   false,
			NetFlowRankingDuration: "1h",
			NetFlowRankingLimit:    10,
			// Price ranking data
			EnablePriceRanking:   false,
			PriceRankingDuration: "1h,4h,24h",
			PriceRankingLimit:    10,
		},
		RiskControl: RiskControlConfig{
			MaxPositions:                 3,
			BTCETHMaxLeverage:            3,
			AltcoinMaxLeverage:           3,
			BTCETHMaxPositionValueRatio:  0.75,
			AltcoinMaxPositionValueRatio: 1.0,
			MaxMarginUsage:               0.75,
			MinPositionSize:              75,
			MinRiskRewardRatio:           1.8,
			MinConfidence:                70,
			MaxRiskPerTradePct:           0.009,
		},
	}

	config.CustomPrompt = balancedSmallAccountCustomPrompt()
	config.PromptSections = balancedSmallAccountPromptSections()

	return config
}

// Create create a strategy
func (s *StrategyStore) Create(strategy *Strategy) error {
	return s.db.Create(strategy).Error
}

// Update update a strategy
func (s *StrategyStore) Update(strategy *Strategy) error {
	return s.db.Model(&Strategy{}).
		Where("id = ? AND user_id = ?", strategy.ID, strategy.UserID).
		Updates(map[string]interface{}{
			"name":           strategy.Name,
			"description":    strategy.Description,
			"config":         strategy.Config,
			"is_public":      strategy.IsPublic,
			"config_visible": strategy.ConfigVisible,
			"updated_at":     time.Now().UTC(),
		}).Error
}

// Delete delete a strategy
func (s *StrategyStore) Delete(userID, id string) error {
	// do not allow deleting system default strategy
	var st Strategy
	if err := s.db.Where("id = ?", id).First(&st).Error; err == nil && st.IsDefault {
		return fmt.Errorf("cannot delete system default strategy")
	}

	return s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&Strategy{}).Error
}

// List get user's strategy list
func (s *StrategyStore) List(userID string) ([]*Strategy, error) {
	var strategies []*Strategy
	err := s.db.Where("user_id = ? OR is_default = ?", userID, true).
		Order("is_default DESC, created_at DESC").
		Find(&strategies).Error
	if err != nil {
		return nil, err
	}
	return strategies, nil
}

// ListPublic get all public strategies for the strategy market
func (s *StrategyStore) ListPublic() ([]*Strategy, error) {
	var strategies []*Strategy
	err := s.db.Where("is_public = ?", true).
		Order("created_at DESC").
		Find(&strategies).Error
	if err != nil {
		return nil, err
	}
	return strategies, nil
}

// Get get a single strategy
func (s *StrategyStore) Get(userID, id string) (*Strategy, error) {
	var st Strategy
	err := s.db.Where("id = ? AND (user_id = ? OR is_default = ?)", id, userID, true).
		First(&st).Error
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// GetActive get user's currently active strategy
func (s *StrategyStore) GetActive(userID string) (*Strategy, error) {
	var st Strategy
	err := s.db.Where("user_id = ? AND is_active = ?", userID, true).First(&st).Error
	if err == gorm.ErrRecordNotFound {
		// no active strategy, return system default strategy
		return s.GetDefault()
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// GetDefault get system default strategy
func (s *StrategyStore) GetDefault() (*Strategy, error) {
	var st Strategy
	// Try to get default English strategy first
	err := s.db.Where("is_default = ? AND id = ?", true, "system_default_en").First(&st).Error
	if err == nil {
		return &st, nil
	}
	// Fallback to any default strategy
	err = s.db.Where("is_default = ?", true).First(&st).Error
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// GetActiveOrDefault get user's active strategy or system default
func (s *StrategyStore) GetActiveOrDefault(userID string) (*Strategy, error) {
	var st Strategy
	// 1. Try active strategy
	err := s.db.Where("user_id = ? AND is_active = ?", userID, true).First(&st).Error
	if err == nil {
		return &st, nil
	}

	// 2. Try any strategy of the user
	err = s.db.Where("user_id = ?", userID).First(&st).Error
	if err == nil {
		return &st, nil
	}

	// 3. Fallback to system default
	return s.GetDefault()
}

// SetActive set active strategy (will first deactivate other strategies)
func (s *StrategyStore) SetActive(userID, strategyID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// first deactivate all strategies for the user
		if err := tx.Model(&Strategy{}).Where("user_id = ?", userID).
			Update("is_active", false).Error; err != nil {
			return err
		}

		// activate specified strategy
		return tx.Model(&Strategy{}).
			Where("id = ? AND (user_id = ? OR is_default = ?)", strategyID, userID, true).
			Update("is_active", true).Error
	})
}

// Duplicate duplicate a strategy (used to create custom strategy based on default strategy)
func (s *StrategyStore) Duplicate(userID, sourceID, newID, newName string) error {
	// get source strategy
	source, err := s.Get(userID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get source strategy: %w", err)
	}

	// create new strategy
	newStrategy := &Strategy{
		ID:          newID,
		UserID:      userID,
		Name:        newName,
		Description: "Created based on [" + source.Name + "]",
		IsActive:    false,
		IsDefault:   false,
		Config:      source.Config,
	}

	return s.Create(newStrategy)
}

// ParseConfig parse strategy configuration JSON
func (s *Strategy) ParseConfig() (*StrategyConfig, error) {
	var config StrategyConfig
	if err := json.Unmarshal([]byte(s.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to parse strategy configuration: %w", err)
	}
	return &config, nil
}

// SetConfig set strategy configuration
func (s *Strategy) SetConfig(config *StrategyConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize strategy configuration: %w", err)
	}
	s.Config = string(data)
	return nil
}
