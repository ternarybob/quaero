test\results\api\market_workers_20260106-133722\TestWorkerAnnouncementsSingle\output_1.md

Technical Correction: Announcement Analysis Worker Implementation
To: Development Team / AI Analysis Agent

Subject: Critical Failure in Signal vs. Noise Analysis (Ticker: EXR)

1. The Issue
The current application output for EXR is suffering from "Narrative Capture." Instead of performing a critical, data-driven audit of management’s performance, the worker is reproducing a promotional executive summary that contradicts its own technical data.

Specific Contradictions in Current Output:

The Summary says: "High-quality announcement profile... 82% accuracy... reliable signals."

The Data says: Conviction Score is 2/10, Communication Style is LEAKY/INSIDER RISK, and the Leak Score is 35.3%.

The Failure: The Executive Summary is essentially "hallucinating" a positive narrative that the underlying metrics have already debunked.

2. Required Logic Changes
You are directed to update the analysis logic to prioritize Operational Reality over Management Narrative. Apply the following logic gates:

A. Tone & Summary Reset
Constraint: If the ConvictionScore is < 4 or LeakScore > 0.2, the Executive Summary MUST NOT use words like "high-quality," "transparent," or "reliable."

Requirement: The summary must lead with the Risk Flags. If a "Leaky" style is detected, the summary must focus on the danger of information leakage and pre-announcement drift rather than "proactive engagement."

B. Critical Signal vs. Noise Filter
Management Bluff Detection: The worker is currently giving credit for "82% accuracy." This is a shallow metric. You must implement the Management Bluff logic: If Price Sensitive = True but Volume Ratio < 0.8, categorize this as a failure of management to identify truly material news.

Sentiment Noise Detection: If "Non-price-sensitive" updates (like Webinars) trigger High Signals, the summary must identify this as Retail Speculation/Hype, not "proactive engagement."

C. Data Extraction Requirements (Missing Information)
The current report lacks the technical "Operational Reality" data. The worker must be updated to fetch or flag the following gaps:

Target vs. Result: Did the "23m Net Gas Pay" announcement meet the pre-drill geological target? (Currently, the worker just sees "High Signal" without knowing if the result was a technical disappointment).

Cash Burn Correlation: Align the frequency of "High Signal" announcements with the quarterly cash burn. Is the company "buying" market sentiment with high-cost exploration news?

3. Revised Output Instruction (The "Critical" Prompt)
When generating the final report, use this directive:

"Act as a short-seller or a critical forensic auditor. Your goal is to remove the emotion and false statements from the company narrative. If the data shows a 35% leak score and a 2/10 conviction rating, your summary must investigate why management is failing to control information and why the market is 'selling the fact' on major milestones. Focus on the divergence between what management says (The Noise) and what the price/volume data proves (The Signal)."

4. Implementation Checklist
[ ] Ensure SignalSummary metrics (LeakScore, NoiseRatio) dictate the vocabulary of the ExecutiveSummary.

[ ] Implement the MANAGEMENT_BLUFF classification strictly (Sensitive: Yes + Low Volume = Bluff).

[ ] Add a "Strategic Divergence" section that highlights whenever a "Price Sensitive" news item results in a negative price move (indicating the market viewed the 'success' as a failure).

[ ] Flag all instances of Pre-Drift > 5% as "High Probability Information Leakage" in the lead paragraph.