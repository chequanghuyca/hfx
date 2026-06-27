package kernel

import (
	"errors"
	"strings"
	"testing"
	"time"

	"nofx/store"
)

func TestExtractDecisionsIgnoresBracketedProse(t *testing.T) {
	response := `We need to output a raw JSON array. Must start with [ and end with ]. Let me analyze the market first.`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected safe fallback, got error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one safe fallback decision, got %d", len(decisions))
	}
	if decisions[0].Symbol != "ALL" || decisions[0].Action != "wait" {
		t.Fatalf("unexpected fallback decision: %#v", decisions[0])
	}
}

func TestExtractDecisionsFindsValidArrayAfterProse(t *testing.T) {
	response := `I checked the candidates. [not json]
[
  {"symbol":"DOGEUSDT","action":"wait","confidence":60,"reasoning":"No clean trigger"}
]`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected valid decision extraction, got error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(decisions))
	}
	if decisions[0].Symbol != "DOGEUSDT" || decisions[0].Action != "wait" {
		t.Fatalf("unexpected decision: %#v", decisions[0])
	}
}

func TestExtractDecisionsFindsWrappedDecisionsObject(t *testing.T) {
	response := `{"decisions":[{"symbol":"AVAXUSDT","action":"wait","confidence":60,"reasoning":"No clean trigger"}]}`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected valid decision extraction, got error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(decisions))
	}
	if decisions[0].Symbol != "AVAXUSDT" || decisions[0].Action != "wait" {
		t.Fatalf("unexpected decision: %#v", decisions[0])
	}
}

func TestExtractDecisionsAcceptsSingleDecisionObject(t *testing.T) {
	response := `{"symbol":"ALL","action":"wait","confidence":60,"reasoning":"No clean trigger"}`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected valid decision extraction, got error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(decisions))
	}
	if decisions[0].Symbol != "ALL" || decisions[0].Action != "wait" {
		t.Fatalf("unexpected decision: %#v", decisions[0])
	}
}

func TestInvalidJSONSafeWaitDetection(t *testing.T) {
	invalidFormatWait := &FullDecision{Decisions: []Decision{{
		Symbol:     "ALL",
		Action:     "wait",
		Confidence: 0,
		Reasoning:  "Model didn't output valid JSON decision, entering safe wait; summary: We need to output only raw JSON",
	}}}
	if !isInvalidJSONSafeWaitDecision(invalidFormatWait) {
		t.Fatal("expected invalid JSON safe-wait to be detected")
	}

	validWait := &FullDecision{Decisions: []Decision{{
		Symbol:     "ALL",
		Action:     "wait",
		Confidence: 60,
		Reasoning:  "No executable setup after scanning candidates",
	}}}
	if isInvalidJSONSafeWaitDecision(validWait) {
		t.Fatal("valid wait decision should not be treated as format failure")
	}
}

func TestBuildTruncatedDecisionRepairPromptUsesCurrentRiskBudget(t *testing.T) {
	prompt := buildTruncatedDecisionRepairPrompt(
		`No candidate has a confirmed trigger. {"decisions":`,
		132,
		store.RiskControlConfig{
			AltcoinMaxLeverage: 3,
			BTCETHMaxLeverage:  3,
			MinConfidence:      68,
			MinRiskRewardRatio: 1.6,
			MaxRiskPerTradePct: 0.006,
		},
	)

	for _, expected := range []string{"risk_usd <= 0.79", "confidence >= 68", `"action":"wait"`} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("repair prompt should contain %q, got: %s", expected, prompt)
		}
	}
}

func TestTruncatedSafeWaitDecisionReturnsExecutableWait(t *testing.T) {
	decision := truncatedSafeWaitDecision(
		"system",
		"user",
		"No candidate has an executable trigger",
		1500*time.Millisecond,
	)

	if len(decision.Decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(decision.Decisions))
	}
	if decision.Decisions[0].Action != "wait" || decision.Decisions[0].Confidence != 60 {
		t.Fatalf("unexpected safe wait decision: %#v", decision.Decisions[0])
	}
	if decision.AIRequestDurationMs != 1500 {
		t.Fatalf("unexpected duration: %d", decision.AIRequestDurationMs)
	}
}

func TestTruncatedPartialNeedsUsefulContextBeforeRepair(t *testing.T) {
	for _, partial := range []string{"", "{", "[", `{"decisions":`, "```json\n"} {
		if isMeaningfulTruncatedPartial(partial) {
			t.Fatalf("partial %q should be too small to repair", partial)
		}
	}

	useful := `**Analysis:** BTC has a 5m EMA reclaim but confidence is below threshold. {"decisions":`
	if !isMeaningfulTruncatedPartial(useful) {
		t.Fatalf("expected market analysis partial to be repairable")
	}
}

func TestTemporaryAIProviderErrorDetectsRateLimit(t *testing.T) {
	err := errors.New(`still failed after 4 retries: API returned error (status 429): provider is temporarily rate-limited`)
	if !isTemporaryAIProviderError(err) {
		t.Fatal("expected 429/rate-limit provider error to be treated as temporary")
	}

	if isTemporaryAIProviderError(errors.New("invalid request: bad strategy config")) {
		t.Fatal("non-provider temporary errors should not be swallowed")
	}
}

func TestExtractDecisionsConvertsGemmaNoTradeAnalysisToWait(t *testing.T) {
	response := `**Analysis:**

1. Market Overview: Most candidates are overbought on the 15m timeframe.
2. Candidate Evaluation: XRP and DOGE have no clean short trigger yet.
3. Directional Neutrality: Shorting without confirmation is premature.
4. **Conclusion:** No candidate currently presents a high-confidence setup with a valid risk-reward ratio.

{"decisions":`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected deterministic wait extraction, got error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Action != "wait" || decisions[0].Confidence != 60 {
		t.Fatalf("unexpected inferred decision: %#v", decisions)
	}
	if !isAnalysisDerivedWait(decisions[0]) {
		t.Fatalf("expected analysis-derived wait marker, got %#v", decisions[0])
	}
}

func TestExtractDecisionsConvertsTruncatedFencedGemmaAnalysisToWait(t *testing.T) {
	response := `**Analysis:**

1. **Market Overview**: Most altcoins are currently in a sharp short-term decline, with RSI7 values deeply in the oversold territory.
2. **Candidate Evaluation**: Shorts are high-risk due to mean-reversion bounce risk. Longs require confirmed reclaim or higher-low structure.
3. **Directional Neutrality**: I evaluated both sides. Shorts are penalized due to oversold conditions; longs lack confirmation.
4. **Conclusion**: No candidate currently presents a high-confidence setup with a valid Risk-Reward ratio.

` + "```json\n" + `{"decisions":`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected deterministic wait extraction, got error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(decisions))
	}
	if decisions[0].Action != "wait" || decisions[0].Confidence != 60 {
		t.Fatalf("unexpected inferred decision: %#v", decisions[0])
	}
	if !isAnalysisDerivedWait(decisions[0]) {
		t.Fatalf("expected analysis-derived wait marker, got %#v", decisions[0])
	}
}

func TestExtractDecisionsConvertsGPTNoTradeThresholdAnalysisToWait(t *testing.T) {
	response := `**Analysis:**

1. **Market Overview:** Most candidate coins are currently in a state of extreme oversold conditions on the 15m timeframe.
2. **Candidate Evaluation:** While they are oversold, there is no clear 5m or 15m bullish trigger to justify a LONG entry.
3. **Directional Neutrality:** Shorts are penalized due to oversold conditions. Longs are penalized because there is no confirmed reversal trigger.
4. **Conclusion:** No candidate meets the minimum confidence threshold (68) for an entry. The risk-reward ratio cannot be safely established without a clear trigger.

` + "```json\n" + `{"decisions":`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected deterministic wait extraction, got error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(decisions))
	}
	if decisions[0].Action != "wait" || decisions[0].Confidence != 60 {
		t.Fatalf("unexpected inferred decision: %#v", decisions[0])
	}
	if !isAnalysisDerivedWait(decisions[0]) {
		t.Fatalf("expected analysis-derived wait marker, got %#v", decisions[0])
	}
}

func TestInferExplicitWaitDoesNotTreatFormatInstructionAsDecision(t *testing.T) {
	response := `If no candidate has an executable trigger, return a wait decision.
We need to analyze all candidates before deciding.`

	if decision, ok := inferExplicitWaitFromAnalysis(response); ok {
		t.Fatalf("format instruction must not become a market decision: %#v", decision)
	}
}

func TestAnalysisDerivedWaitDecisionStoresNormalizedRawJSON(t *testing.T) {
	wait, ok := inferExplicitWaitFromAnalysis(
		`Conclusion: No candidate currently presents a high-confidence setup.`,
	)
	if !ok {
		t.Fatal("expected explicit wait inference")
	}

	full := analysisDerivedWaitDecision("system", "user", "long analysis", wait, time.Second)
	if strings.Contains(full.RawResponse, "long analysis") {
		t.Fatalf("raw response should be normalized JSON, got %s", full.RawResponse)
	}
	if !strings.Contains(full.RawResponse, `"action":"wait"`) {
		t.Fatalf("expected normalized wait JSON, got %s", full.RawResponse)
	}
	if full.CoTTrace != "long analysis" {
		t.Fatalf("expected original analysis in CoT trace, got %q", full.CoTTrace)
	}
}
