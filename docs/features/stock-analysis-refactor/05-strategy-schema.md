# ASX Portfolio Intelligence System — Strategy Configuration Schema

## Document Purpose
This document defines the complete strategy configuration schema. The strategy file controls all aspects of stock selection, entry/exit criteria, position sizing, and screening.

---

## Schema Overview

The strategy configuration is a TOML file with the following sections:

1. **Meta** - Strategy identification
2. **Fundamentals** - Business quality filters
3. **Valuation** - Pricing constraints
4. **Technicals** - Entry/exit timing
5. **Announcements** - PR/signal filters
6. **Sectors** - Allocation targets
7. **Position Sizing** - Risk management
8. **Exits** - Exit rules
9. **Screening** - New stock discovery
10. **Comparative** - Peer analysis
11. **Alerts** - Monitoring triggers

---

## Complete Schema Definition

```toml
# =========================================================================
# PORTFOLIO STRATEGY CONFIGURATION
# =========================================================================
# Version: 2.1
# This file defines all parameters for portfolio analysis and screening.
# =========================================================================

# =========================================================================
# SECTION 1: META
# =========================================================================

[strategy]
name = "SMSF Growth-Quality Hybrid"
version = "2.1"
last_updated = "2024-12-28"
description = """
Quality-focused growth strategy for SMSF with 6-24 month holding periods.
Prioritises cash-generative businesses with demonstrated pricing power,
acquired during technical accumulation phases. Avoids narrative-driven 
stocks and high-PR issuers.
"""

# =========================================================================
# SECTION 2: FUNDAMENTALS
# Business quality filters - these define minimum thresholds for business
# quality. Stocks failing these filters are excluded from consideration.
# =========================================================================

[strategy.fundamentals]
description = "Minimum business quality thresholds"

# --- Profitability ---
[strategy.fundamentals.profitability]
# Return on Equity minimum
# Rationale: ROE > 12% indicates capital-efficient business
min_roe_pct = 12.0

# Return on Invested Capital (if available)
# Rationale: ROIC measures true capital efficiency including debt
min_roic_pct = 10.0

# Operating margin minimum
# Rationale: 8% floor ensures business has pricing power
min_operating_margin_pct = 8.0

# Margin trend requirement
# Options: "stable_or_improving", "any"
margin_trend = "stable_or_improving"

# Quarters to look back for margin trend
margin_trend_lookback_quarters = 4


# --- Cash Quality (CRITICAL) ---
# These metrics differentiate paper profits from real cash generation
[strategy.fundamentals.cash_quality]

# Operating Cash Flow to EBITDA
# Rationale: OCF/EBITDA > 0.70 means earnings convert to cash
# Below 0.60 is a red flag for earnings quality
min_ocf_to_ebitda = 0.70

# Free Cash Flow margin (FCF / Revenue)
# Rationale: Business should generate free cash after all spending
min_fcf_margin_pct = 4.0

# CapEx to Operating Cash Flow
# Rationale: CapEx shouldn't consume all operating cash
max_capex_to_ocf = 0.60

# Cash conversion trend requirement
cash_conversion_trend = "stable_or_improving"


# --- Balance Sheet ---
[strategy.fundamentals.balance_sheet]

# Net Debt to EBITDA
# Rationale: Leverage above 2.5x creates refinancing and interest rate risk
max_net_debt_to_ebitda = 2.5

# Debt to Equity
# Rationale: High D/E increases financial risk
max_debt_to_equity = 1.5

# Current ratio minimum
# Rationale: Short-term liquidity floor
min_current_ratio = 1.0

# Interest coverage (EBIT / Interest)
# Rationale: Must be able to service debt comfortably
min_interest_coverage = 4.0


# --- Growth ---
[strategy.fundamentals.growth]

# 3-year revenue CAGR minimum
# Rationale: Business must be growing, not shrinking
min_revenue_growth_3yr_cagr_pct = 5.0

# 1-year revenue growth minimum
# Rationale: Allow flat but not declining
min_revenue_growth_1yr_pct = 0.0

# Preferred revenue growth (for ranking)
preferred_revenue_growth_pct = 10.0


# --- Capital Discipline ---
[strategy.fundamentals.capital_discipline]

# Share count increase maximum (12 months)
# Rationale: Excessive dilution destroys shareholder value
max_dilution_12m_pct = 8.0

# 3-year dilution CAGR maximum
max_dilution_3yr_cagr_pct = 5.0

# Capital raise tolerance
# Options: "accretive_only", "moderate", "any"
# accretive_only = only accept raises for clearly ROI+ investments
capital_raise_tolerance = "accretive_only"


# =========================================================================
# SECTION 3: VALUATION
# Pricing constraints - avoid overpaying even for good businesses
# =========================================================================

[strategy.valuation]
description = "Valuation boundaries"

# --- PBAS-Based Valuation ---
# PBAS (Price-Business Alignment Score) is the primary valuation metric

# Minimum PBAS to consider buying
# Below this, price has run too far ahead of fundamentals
min_pbas_score = 0.45

# Preferred PBAS for entry
# Sweet spot where business momentum exceeds price momentum
preferred_pbas_score = 0.60

# Maximum PBAS to add to position
# Above this, business is already well-reflected in price
max_pbas_score_for_add = 0.75

# --- Traditional Metrics ---
# Used as secondary filters

# P/E vs sector median maximum
# Don't pay more than 50% premium to sector
max_pe_vs_sector_median = 1.5

# Price/Sales maximum (for growth stocks)
max_ps_ratio = 8.0

# EV/EBITDA maximum
max_ev_to_ebitda = 15.0

# --- Cooked Detection ---
# Automatically exclude stocks flagged as overvalued
cooked_auto_exclude = true
max_cooked_score = 1  # Exclude if cooked_score >= 2


# =========================================================================
# SECTION 4: TECHNICALS
# Entry timing and technical requirements
# =========================================================================

[strategy.technicals]
description = "Technical conditions for entry/exit timing"

# --- Trend Requirements ---
[strategy.technicals.trend]

# Must be above 200-day EMA (long-term uptrend)
require_above_200_ema = true

# Prefer above 50-day EMA (medium-term strength)
prefer_above_50_ema = true

# EMA stack preference
# Options: "bullish" (20>50>200), "any"
ema_stack_preference = "bullish"


# --- Regime Requirements ---
[strategy.technicals.regime]

# Regimes allowed for entry
allowed_entry_regimes = [
    "trend_up",
    "accumulation", 
    "breakout",
    "range",
]

# Regimes that prohibit entry
excluded_regimes = [
    "decay",
    "distribution",
]

# Minimum confidence for regime classification
min_regime_confidence = 0.55


# --- Volume Analysis ---
[strategy.technicals.volume]

# Minimum VLI for entry (must show some accumulation)
min_vli_for_entry = 0.30

# Preferred VLI (ideal institutional accumulation)
preferred_vli = 0.50

# Exclude if VLI shows distribution
exclude_if_distributing = true


# --- Entry Setups ---
# Define specific technical setups for entry
[strategy.technicals.entry_setups]

# Preferred setup types (in order of preference)
preferred_setups = [
    "pullback_to_ema20",
    "breakout_confirmation",
    "vcp_breakout",
    "accumulation_breakout",
]

# Setup 1: Pullback to 20 EMA in uptrend
[strategy.technicals.entry_setups.pullback_to_ema20]
description = "Buy on pullback to 20 EMA in established uptrend"
conditions = [
    "regime in (trend_up, breakout)",
    "price within 2% of ema_20",
    "ema_20 > ema_50 > ema_200",
    "rsi_14 between 40 and 60",
    "volume_on_pullback < vol_sma_20",
]
entry_zone = "ema_20 to ema_20 - 0.5*atr_14"
stop_loss = "ema_50 or entry - 1.5*atr_14"
target = "recent_high + 1*atr_14"

# Setup 2: Breakout with volume confirmation
[strategy.technicals.entry_setups.breakout_confirmation]
description = "Buy breakout of consolidation with volume confirmation"
conditions = [
    "price breaks above 20-day Donchian high",
    "volume > 1.5 * vol_sma_20",
    "close in top 25% of day's range",
    "atr_14 not contracting",
]
entry_zone = "on break or first pullback holding above breakout level"
stop_loss = "below breakout level - 0.5*atr_14"
target = "measured move or 2*atr_14"
volume_confirmation = "require 1.5x average"

# Setup 3: Volatility Contraction Pattern (Minervini-style)
[strategy.technicals.entry_setups.vcp_breakout]
description = "Volatility Contraction Pattern breakout"
conditions = [
    "3+ contracting price ranges",
    "volume declining during contraction",
    "price within 15% of 52-week high",
    "ema_stack bullish",
]
entry_zone = "break of VCP high pivot"
stop_loss = "below VCP low pivot"
volume_confirmation = "require 1.5x average on breakout"

# Setup 4: Accumulation range breakout
[strategy.technicals.entry_setups.accumulation_breakout]
description = "Break out of accumulation range"
conditions = [
    "regime = accumulation for >= 15 days",
    "vli > 0.50",
    "price breaks range high with volume > 2x average",
]
entry_zone = "on breakout or pullback to range top"
stop_loss = "below range midpoint"


# --- Relative Strength ---
[strategy.technicals.relative_strength]

# Minimum RS vs benchmark (3-month)
min_rs_vs_xjo_3m = 1.0

# Preferred RS (outperformance)
preferred_rs_vs_xjo = 1.10

# RS rank minimum (percentile)
min_rs_rank_percentile = 50

# Preferred RS rank
preferred_rs_rank_percentile = 70


# --- Chart Patterns (Advanced) ---
[strategy.technicals.patterns]

# Bullish patterns to look for
bullish_patterns = [
    "cup_and_handle",
    "ascending_triangle",
    "bull_flag",
    "double_bottom",
    "inverse_head_and_shoulders",
]

# Bearish patterns to avoid
bearish_patterns_exclude = [
    "head_and_shoulders",
    "descending_triangle",
    "rising_wedge",
]

# Enable pattern detection
pattern_detection_enabled = true

# Minimum confidence for pattern signals
min_pattern_confidence = 0.60


# =========================================================================
# SECTION 5: ANNOUNCEMENTS
# Filter based on announcement quality and PR detection
# =========================================================================

[strategy.announcements]
description = "Filter based on announcement quality"

# --- PR Detection ---
# Maximum PR entropy score (higher = more PR-speak)
max_pr_entropy_score = 0.70

# Minimum average SNI over 6 months
min_avg_sni_6m = 0.30

# Exclude stocks flagged as PR-heavy issuers
pr_heavy_issuer_exclude = true

# --- Announcement Frequency ---
# Maximum announcements per month (excessive = likely PR-driven)
max_announcements_per_month = 8

# --- Required Disclosures ---
# For pre-profit companies, require recent 4C
require_recent_4c_if_pre_profit = true

# Maximum days since material announcement
max_days_since_material_announcement = 90

# --- Negative Triggers ---
# These announcement types trigger automatic review/flag
negative_triggers = [
    "guidance_downgrade",
    "ceo_departure_unplanned",
    "material_contract_loss",
    "audit_qualification",
    "capital_raise_below_market",
]


# =========================================================================
# SECTION 6: SECTORS
# Sector allocation targets and preferences
# =========================================================================

[strategy.sectors]
description = "Sector allocation and thematic preferences"

# --- Target Allocations ---
# Format: { min, target, max } percentages
[strategy.sectors.targets]
infrastructure = { min = 20, target = 30, max = 35 }
technology = { min = 5, target = 15, max = 25 }
healthcare = { min = 5, target = 10, max = 20 }
financials = { min = 5, target = 10, max = 15 }
resources = { min = 0, target = 5, max = 10 }
consumer = { min = 0, target = 10, max = 15 }
defensive = { min = 15, target = 20, max = 30 }

# --- Excluded Sectors ---
excluded_sectors = [
    "speculative_mining",
    "cannabis",
    "crypto",
]

# --- Thematic Tilts ---
# Current macro preferences
[strategy.sectors.thematic_tilts]

# Sectors/themes with positive bias
positive = [
    "government_infrastructure_spend",
    "defence_modernisation",
    "energy_transition",
    "healthcare_innovation",
]

# Sectors/themes to underweight
negative = [
    "commercial_real_estate",
    "discretionary_retail",
    "china_exposed_resources",
]


# =========================================================================
# SECTION 7: POSITION SIZING
# Risk management and position sizing rules
# =========================================================================

[strategy.position_sizing]
description = "Position sizing and risk management"

# --- Position Limits ---
# Maximum single position as % of portfolio
max_single_position_pct = 8.0

# Maximum sector allocation
max_sector_pct = 35.0

# Maximum for highly correlated positions
max_correlated_cluster_pct = 25.0

# Position count range
min_positions = 15
max_positions = 30

# --- Sizing Methodology ---
# Options: "equal_weight", "equal_risk", "conviction_weighted"
sizing_method = "equal_risk"

[strategy.position_sizing.equal_risk]
# Target risk contribution per position
target_risk_per_position_pct = 0.5

# Use stop loss to determine position size
# Size = (Portfolio × TargetRisk) / (Entry - Stop)
stop_loss_determines_size = true

# --- Entry Scaling ---
[strategy.position_sizing.scaling]

# Initial position as % of full position
initial_position_pct = 50

# Add on confirmation
add_on_confirmation = true

# Maximum adds to a position
max_adds = 2

# Conditions to add
add_conditions = [
    "price holds above entry for 5 days",
    "volume confirms direction",
    "no negative announcements",
]


# =========================================================================
# SECTION 8: EXITS
# Exit rules and profit management
# =========================================================================

[strategy.exits]
description = "When and how to exit positions"

# --- Stop Losses ---
[strategy.exits.stop_loss]

# Stop loss method
# Options: "fixed_pct", "atr_based", "support_based"
method = "atr_based"

# ATR multiple for stop placement
atr_multiple = 2.0

# Hard maximum loss regardless of ATR
max_loss_pct = 15.0

# Trailing stop activation (after X% gain)
trailing_stop_activation_pct = 20.0

# Trailing stop ATR multiple
trailing_stop_atr_multiple = 2.5

# --- Profit Taking ---
[strategy.exits.profit_taking]

# First partial exit
partial_exit_1_pct = 25      # Take 25% off
partial_exit_1_trigger_pct = 30  # When up 30%

# Second partial exit
partial_exit_2_pct = 25
partial_exit_2_trigger_pct = 50  # When up 50%

# Remaining 50% runs with trailing stop

# --- Fundamental Exits ---
[strategy.exits.fundamental]

# Exit if PBAS drops below
exit_if_pbas_below = 0.30

# Exit after N quarters of negative OCF
exit_if_ocf_negative_quarters = 2

# Exit if margin compression exceeds
exit_if_margin_compression_pct = 5.0  # 500bps

# --- Time-Based ---
[strategy.exits.time_based]

# Review if position is flat after X weeks
review_if_flat_after_weeks = 12

# Force review after maximum holding period
max_holding_period_months = 36


# =========================================================================
# SECTION 9: SCREENING
# Parameters for finding new stock candidates
# =========================================================================

[strategy.screening]
description = "Parameters for finding new candidates"

# --- Universe ---
# Options: "ASX_200", "ASX_300", "ASX_ALL"
universe = "ASX_300"

# Minimum market cap
min_market_cap_m = 100

# Minimum liquidity
min_avg_daily_turnover = 500000

# --- Screening Stages ---
# Multi-stage filtering for efficiency

[strategy.screening.stage_1_fundamental]
description = "Fundamental quality filter - reduces universe by ~70%"
filters = [
    "roe_pct >= 12",
    "ocf_to_ebitda >= 0.70",
    "net_debt_to_ebitda <= 2.5",
    "revenue_growth_1yr_pct >= 0",
    "dilution_12m_pct <= 8",
]

[strategy.screening.stage_2_valuation]
description = "Valuation filter - reduces remaining by ~50%"
filters = [
    "pbas >= 0.50",
    "cooked = false",
    "pe_vs_sector <= 1.5",
]

[strategy.screening.stage_3_technical]
description = "Technical setup filter - reduces to actionable list"
filters = [
    "above_200_ema = true",
    "regime in (trend_up, accumulation, breakout, range)",
    "vli >= 0.20",
    "rs_vs_xjo_3m >= 0.95",
]

[strategy.screening.stage_4_announcement]
description = "PR and announcement quality filter"
filters = [
    "pr_heavy_issuer = false",
    "avg_sni_6m >= 0.25",
    "no_negative_triggers_90d",
]

# --- Output ---
[strategy.screening.output]
max_candidates = 10
rank_by = ["pbas", "vli", "rs_rank"]
include_setup_details = true


# =========================================================================
# SECTION 10: COMPARATIVE ANALYSIS
# How to compare holdings and candidates
# =========================================================================

[strategy.comparative]
description = "Comparative analysis parameters"

# --- Peer Comparison ---
[strategy.comparative.peer_analysis]

# Compare within sector
compare_within_sector = true

# Metrics to compare
metrics_to_compare = [
    "pbas",
    "roe",
    "ocf_to_ebitda",
    "revenue_growth",
    "rs_vs_xjo",
    "vli",
]

# Flag if in bottom quartile
flag_if_bottom_quartile = true

# --- Portfolio Fit ---
[strategy.comparative.portfolio_fit]

# Assess correlation with existing holdings
assess_correlation = true

# Maximum correlation with existing position
max_correlation_with_existing = 0.75

# Check sector balance impact
assess_sector_balance = true

# Check regime diversity
assess_regime_diversity = true


# =========================================================================
# SECTION 11: ALERTS
# Monitoring and alert triggers
# =========================================================================

[strategy.alerts]
description = "Alert configuration"

# --- Holding Alerts ---
[[strategy.alerts.holding]]
condition = "pbas < 0.40"
severity = "warning"
action = "review"

[[strategy.alerts.holding]]
condition = "pbas < 0.35"
severity = "urgent"
action = "reduce"

[[strategy.alerts.holding]]
condition = "vli < -0.40"
severity = "warning"
action = "watch"

[[strategy.alerts.holding]]
condition = "regime = decay"
severity = "urgent"
action = "exit"

[[strategy.alerts.holding]]
condition = "cooked = true"
severity = "urgent"
action = "reduce"

[[strategy.alerts.holding]]
condition = "stop_loss_breach"
severity = "critical"
action = "exit"

# --- Portfolio Alerts ---
[[strategy.alerts.portfolio]]
condition = "sector_weight > sector_max"
severity = "warning"

[[strategy.alerts.portfolio]]
condition = "correlation_cluster > 0.80"
severity = "warning"

[[strategy.alerts.portfolio]]
condition = "cash_pct < 3"
severity = "info"

# --- Opportunity Alerts ---
[[strategy.alerts.opportunity]]
condition = "holding_at_entry_setup"
severity = "info"

[[strategy.alerts.opportunity]]
condition = "watchlist_breakout"
severity = "info"


# =========================================================================
# SECTION 12: EXECUTION PREFERENCES
# How to execute trades
# =========================================================================

[strategy.execution]

# Prefer limit orders over market
prefer_limit_orders = true

# Maximum spread to tolerate
max_spread_pct = 1.0

# Avoid opening volatility
avoid_first_30_min = true

# Avoid closing games
avoid_last_15_min = true

# Preferred trading window
preferred_execution_window = "10:30-15:45"
```

---

## Schema Validation

```go
package config

import (
    "errors"
    "fmt"
)

// ValidateStrategy validates a loaded strategy configuration
func ValidateStrategy(s *Strategy) error {
    var errs []error
    
    // Fundamentals validation
    if s.Fundamentals.Profitability.MinROEPct < 0 {
        errs = append(errs, errors.New("min_roe_pct cannot be negative"))
    }
    
    if s.Fundamentals.CashQuality.MinOCFToEBITDA < 0 || 
       s.Fundamentals.CashQuality.MinOCFToEBITDA > 2 {
        errs = append(errs, errors.New("min_ocf_to_ebitda must be between 0 and 2"))
    }
    
    // Valuation validation
    if s.Valuation.MinPBASScore < 0 || s.Valuation.MinPBASScore > 1 {
        errs = append(errs, errors.New("min_pbas_score must be between 0 and 1"))
    }
    
    if s.Valuation.MinPBASScore > s.Valuation.PreferredPBASScore {
        errs = append(errs, errors.New("min_pbas_score cannot exceed preferred_pbas_score"))
    }
    
    // Technicals validation
    for _, regime := range s.Technicals.Regime.AllowedEntryRegimes {
        if !isValidRegime(regime) {
            errs = append(errs, fmt.Errorf("invalid regime: %s", regime))
        }
    }
    
    // Position sizing validation
    if s.PositionSizing.MaxSinglePositionPct > 15 {
        errs = append(errs, errors.New("max_single_position_pct should not exceed 15%"))
    }
    
    totalSectorMax := sumSectorMaxes(s.Sectors.Targets)
    if totalSectorMax < 100 {
        errs = append(errs, errors.New("sector max allocations should sum to at least 100%"))
    }
    
    // Exits validation
    if s.Exits.StopLoss.MaxLossPct > 25 {
        errs = append(errs, errors.New("max_loss_pct should not exceed 25%"))
    }
    
    if len(errs) > 0 {
        return combineErrors(errs)
    }
    
    return nil
}

func isValidRegime(regime string) bool {
    valid := map[string]bool{
        "trend_up":       true,
        "trend_down":     true,
        "breakout":       true,
        "accumulation":   true,
        "distribution":   true,
        "range":          true,
        "decay":          true,
        "undefined":      true,
    }
    return valid[regime]
}
```

---

## Loading Strategy

```go
package config

import (
    "os"
    
    "github.com/BurntSushi/toml"
)

// LoadStrategy loads a strategy from a TOML file
func LoadStrategy(path string) (*Strategy, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read strategy file: %w", err)
    }
    
    var strategy Strategy
    if err := toml.Unmarshal(data, &strategy); err != nil {
        return nil, fmt.Errorf("failed to parse strategy: %w", err)
    }
    
    if err := ValidateStrategy(&strategy); err != nil {
        return nil, fmt.Errorf("invalid strategy: %w", err)
    }
    
    return &strategy, nil
}
```

---

## Next Document
Proceed to `06-announcement-processing.md` for announcement classification and SNI calculation.
