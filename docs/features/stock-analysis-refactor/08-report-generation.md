# ASX Portfolio Intelligence System — Report Generation

## Document Purpose
This document specifies how the daily portfolio report is assembled from component data. It defines the report structure, section content, and formatting rules.

---

## Report Assembly Flow

```
ASSEMBLE_DAILY_REPORT(portfolio_state, rollup, assessments, signals):

  report = new DailyReport()
  
  # Metadata
  report.generated_at = current_timestamp()
  report.report_date = format_date(today())
  
  # Build sections in order
  report.executive_summary = build_executive_summary(portfolio_state, rollup)
  report.market_context = build_market_context()
  report.actions_required = build_actions_section(assessments)
  report.holdings_summary = build_holdings_by_block(portfolio_state, signals, assessments)
  report.portfolio_health = build_health_checks(rollup)
  report.upcoming_catalysts = build_catalyst_calendar(signals)
  report.screening_results = build_screening_section() IF enabled
  report.appendix = build_appendix()
  
  # Render to formats
  report.markdown_content = render_markdown(report)
  report.html_content = render_html(report) IF email_format = html
  
  RETURN report
```

---

## Report Sections

### 1. Executive Summary

**Content**:
- Portfolio total value
- Daily change ($ and %)
- YTD return
- Performance vs benchmark (XJO)
- Number of positions
- Current market regime
- Top 3 priority items

```
BUILD_EXECUTIVE_SUMMARY(state, rollup):

  summary = {
    portfolio_value: rollup.performance.total_value,
    daily_change: calculate_daily_change(state),
    daily_change_pct: calculate_daily_change_pct(state),
    ytd_return: rollup.performance.return_ytd_pct,
    vs_benchmark: rollup.performance.vs_xjo_ytd_pct,
    positions: count(state.holdings),
    market_regime: get_market_regime(),
    priority_items: extract_priority_items(rollup.action_summary, limit=3)
  }
  
  RETURN summary
```

**Format Template**:
```
PORTFOLIO SNAPSHOT
──────────────────────────────────────────────────────────────────
  Total Value:     ${total_value}      Cost Basis:    ${total_cost}
  Unrealised P&L:  +${pnl} (+{pnl_pct}%)
  
  vs XJO (YTD):    {vs_xjo}% {outperformance|underperformance}

  Holdings: {count} │ SMSF: {smsf_count} │ Trader: {trader_count}
```

---

### 2. Market Context

**Content**:
- Index level and trend position
- Position vs key EMAs
- Market regime classification
- Breadth indicator
- Key upcoming events

```
BUILD_MARKET_CONTEXT():

  xjo_data = fetch_index_data("XJO")
  
  context = {
    index_level: xjo_data.current,
    vs_50_ema: compare_to_ema(xjo_data, 50),
    vs_200_ema: compare_to_ema(xjo_data, 200),
    regime: classify_market_regime(xjo_data),
    breadth: calculate_breadth_pct_above_200ema(),
    key_events: get_upcoming_macro_events()
  }
  
  RETURN context
```

---

### 3. Actions Required

**Structure**:
- 🔴 URGENT (This Week): Holdings requiring immediate action
- 🟡 WATCH (Monitor Closely): Holdings approaching triggers

**Per-Holding Detail Block**:
```
BUILD_ACTION_DETAIL(assessment, signals, holding):

  detail = {
    # Header info
    ticker: holding.ticker,
    name: holding.name,
    action: assessment.decision.action,
    urgency: assessment.decision.urgency,
    
    # Position info
    position: "{units} units @ ${avg_price} avg",
    current_price: signals.price.current,
    pnl: calculate_pnl(holding, signals.price.current),
    pnl_pct: calculate_pnl_pct(holding, signals.price.current),
    weight: calculate_weight_pct(holding, portfolio_value),
    
    # Technical assessment box
    technical_box: {
      pbas_score: signals.pbas.score,
      pbas_interpretation: signals.pbas.interpretation,
      pbas_components: {
        business_momentum: signals.pbas.business_momentum,
        price_momentum: signals.pbas.price_momentum
      },
      vli_score: signals.vli.score,
      vli_label: signals.vli.label,
      regime: signals.regime.classification,
      regime_confidence: signals.regime.confidence,
      cooked: signals.cooked
    },
    
    # Fundamental assessment box
    fundamental_box: {
      cash_conversion: signals.quality.cash_conversion,
      margin_trend: signals.quality.margin_trend,
      balance_sheet_risk: signals.quality.balance_sheet_risk
    },
    
    # Announcement analysis box
    announcement_box: {
      total_30d: signals.announcements.count_30d,
      high_signal: signals.announcements.high_signal_count_30d,
      noise: signals.announcements.noise_count,
      most_recent: signals.announcements.most_recent_material,
      most_recent_sni: signals.announcements.most_recent_material_sni,
      pr_heavy: signals.announcements.pr_heavy_issuer
    },
    
    # Decision box
    decision_box: {
      action: assessment.decision.action,
      confidence: assessment.decision.confidence,
      urgency: assessment.decision.urgency,
      primary_reasoning: assessment.reasoning.primary,
      evidence: assessment.reasoning.evidence,  # Exactly 3 items
      risk_flags: assessment.risk_flags
    },
    
    # Execution guidance
    execution: {
      instruction: format_execution_instruction(assessment),
      stop_loss: assessment.entry_exit.stop_loss,
      stop_loss_pct: assessment.entry_exit.stop_loss_pct,
      invalidation: assessment.entry_exit.invalidation
    },
    
    # Justified gain analysis
    justified_gain_box: {
      your_return: holding.return_pct,
      justified_return: assessment.justified_gain.justified_12m_pct,
      divergence: assessment.justified_gain.divergence_pct,
      interpretation: assessment.justified_gain.verdict
    }
  }
  
  RETURN detail
```

---

### 4. Holdings Summary by Block

**Block Groupings**:
- Infrastructure Block
- Growth Block  
- Defensive Block
- Thematic ETF Block
- Blue Chip Block

```
BUILD_HOLDINGS_BY_BLOCK(portfolio_state, signals, assessments):

  blocks = []
  
  # Group holdings by sector/type
  grouped = group_holdings_by_block(portfolio_state.holdings)
  
  FOR each block_name, holdings IN grouped:
    
    block = {
      name: block_name,
      weight_pct: sum(calculate_weight(h) FOR h IN holdings),
      target_pct: get_sector_target(block_name),
      holdings_table: [],
      block_assessment: "",
      block_risks: []
    }
    
    FOR each holding IN holdings:
      signal = signals[holding.ticker]
      assessment = assessments[holding.ticker]
      
      row = {
        ticker: holding.ticker,
        price: signal.price.current,
        pnl_pct: calculate_pnl_pct(holding, signal.price.current),
        pbas: signal.pbas.score,
        regime: signal.regime.classification,
        vli: format_vli_short(signal.vli),
        action: assessment.decision.action
      }
      block.holdings_table.append(row)
    
    block.block_assessment = generate_block_narrative(holdings, signals)
    block.block_risks = identify_block_level_risks(holdings, signals)
    
    blocks.append(block)
  
  RETURN blocks
```

**Table Format**:
```
┌────────┬─────────┬────────┬───────┬──────────────┬───────────┬────────────┐
│ Ticker │ Current │ P&L %  │ PBAS  │ Regime       │ VLI       │ Action     │
├────────┼─────────┼────────┼───────┼──────────────┼───────────┼────────────┤
│ {data rows}                                                               │
└────────┴─────────┴────────┴───────┴──────────────┴───────────┴────────────┘
```

---

### 5. Portfolio Health

```
BUILD_HEALTH_CHECKS(rollup):

  checks = {
    concentration: [],
    quality_distribution: {},
    regime_distribution: {}
  }
  
  # Concentration checks
  checks.concentration.append({
    name: "Largest position",
    status: IF max_position_pct < 8 THEN "pass" ELSE "warning",
    detail: "{ticker} at {pct}% (limit: 8%)"
  })
  
  checks.concentration.append({
    name: "Top 5 positions",
    status: IF top5_pct < 40 THEN "pass" ELSE "warning",
    detail: "{pct}% (limit: 40%)"
  })
  
  checks.concentration.append({
    name: "Sector limits",
    status: check_all_sector_limits(rollup.allocation.by_sector),
    detail: format_sector_status()
  })
  
  # Quality distribution
  checks.quality_distribution = {
    underpriced: count WHERE pbas > 0.65,
    fair: count WHERE pbas BETWEEN 0.50 AND 0.65,
    watch: count WHERE pbas < 0.50
  }
  
  # Regime distribution
  checks.regime_distribution = rollup.allocation.by_regime
  
  RETURN checks
```

---

### 6. Upcoming Catalysts

```
BUILD_CATALYST_CALENDAR(signals):

  catalysts = []
  
  FOR each ticker, signal IN signals:
    
    # Earnings dates
    IF has_expected_results_date(signal):
      catalysts.append({
        date: signal.expected_results_date,
        ticker: ticker,
        event: determine_results_type(signal),
        impact: assess_impact_significance(signal)
      })
    
    # 4C dates
    IF has_expected_4c_date(signal):
      catalysts.append({
        date: signal.expected_4c_date,
        ticker: ticker,
        event: "4C Quarterly",
        impact: "Cash flow / burn rate"
      })
  
  # Add macro events
  macro_events = get_macro_calendar()
  catalysts.extend(macro_events)
  
  # Sort by date, limit to 10
  catalysts = sort_by_date(catalysts)[:10]
  
  RETURN catalysts
```

---

### 7. Screening Results (Optional)

```
BUILD_SCREENING_SECTION():

  IF NOT config.screening_enabled:
    RETURN null
  
  candidates = run_screening_pipeline(strategy)
  
  results = []
  FOR each candidate IN candidates[:3]:  # Top 3 only
    
    result = {
      ticker: candidate.ticker,
      name: candidate.name,
      sector: candidate.sector,
      
      why_flagged: format_screening_reasons(candidate),
      technical_setup: describe_entry_setup(candidate),
      strategy_match: list_matching_criteria(candidate)
    }
    results.append(result)
  
  RETURN results
```

---

### 8. Appendix

**Content**:
- Signal definitions reference
- Strategy version and last updated
- Data sources list
- Processing time

---

## Formatting Standards

### Number Formatting Rules
```
FORMATTING:
  prices:        $X.XX (2 decimals)
  percentages:   X.X% (1 decimal)  
  large_numbers: $X.XXM or $X.XXB
  ratios:        X.XX (2 decimals)
  scores:        0.XX (2 decimals)
```

### Visual Elements
```
INDICATORS:
  🔴  Urgent action required
  🟡  Watch closely
  ✓   Check passed
  ⚠   Warning
  
SEPARATORS:
  Major: ━━━━━━━━━━━━━━━━━━━━━━━━━
  Minor: ──────────────────────────
  
BOXES:
  Use box-drawing characters for data tables
  Use indentation for nested content
```

### Length Guidelines

| Section | Target Lines |
|---------|-------------|
| Executive Summary | 10-15 |
| Market Context | 5-10 |
| Action (per holding) | 30-50 |
| Holdings Block | 10-20 |
| Portfolio Health | 15-20 |
| Catalysts | 10-15 |

---

## Output Storage

```
STORE_REPORT(report):

  tag_document = {
    tag: "daily-report",
    content: {
      markdown: report.markdown_content,
      html: report.html_content,
      structured: report  # Full object for API access
    },
    format: "json",
    metadata: {
      generated_at: report.generated_at,
      processing_time: report.processing_time_seconds,
      holdings_count: report.holdings_assessed
    }
  }
  
  store.set(tag_document)
```

---

## Next Document
Proceed to `09-validation-qa.md` for validation and quality assurance.
