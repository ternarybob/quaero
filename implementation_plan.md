# Refactor Announcement Analysis Logic

Refactor the announcement analysis logic to prioritize "Operational Reality" over "Management Narrative", implementing strict signal/noise filtering and tone resets for leaky/low-conviction stocks.

## User Review Required
> [!IMPORTANT]
> The "Management Bluff" logic will be strictly enforced: `PriceSensitive=True` but `VolumeRatio < 0.8` will now be classified as `MANAGEMENT_BLUFF`. This may significantly increase the number of "Bluff" classifications for small-cap stocks with low liquidity.

## Proposed Changes

### Queue Workers

#### [MODIFY] [signal_analysis_classifier.go](file:///c:/development/quaero/internal/queue/workers/signal_analysis_classifier.go)
-   Update `ClassifyAnnouncement` to implement strict `MANAGEMENT_BLUFF` logic.
-   Update `SENTIMENT_NOISE` logic if applicable for "Non-price-sensitive" high signals.

#### [MODIFY] [market_announcements_worker.go](file:///c:/development/quaero/internal/queue/workers/market_announcements_worker.go)
-   Update `AnnouncementSummaryData` struct to include `ConvictionScore`, `LeakScore`, and `CommunicationStyle`.
-   Update `createSummaryDocument` to populate these new fields from the calculated `SignalSummary`.
-   Update `buildSummaryPrompt` to inject the "Revised Output Instruction" and the conditional "Tone Reset" constraints (if `ConvictionScore < 4` or `LeakScore > 0.2`).
-   Add "Strategic Divergence" section prompt instruction.
-   Add "High Probability Information Leakage" flag instruction for `PreDrift > 5%`.

### Templates

#### [MODIFY] [announcement-analysis-report.toml](file:///c:/development/quaero/internal/templates/announcement-analysis-report.toml)
-   Update the prompt to include the "Critical" directive ("Act as a short-seller...") for the multi-stock report as well.

## Verification Plan

### Automated Tests
-   Run unit tests for signal analysis logic:
    ```bash
    go test ./internal/queue/workers/... -v
    ```
-   Run the specific API integration test:
    ```bash
    go test ./test/api/market_workers -run TestWorkerAnnouncementsSingle
    ```

### Manual Verification
-   Inspect the generated markdown output from the test (located in `test/results/api/...`) to verify:
    -   The Executive Summary tone has changed (if inputs trigger the condition).
    -   The "Strategic Divergence" section is present if applicable.
    -   Risk flags are properly highlighted.
