# Quaero Product Strategy

**Tagline:** Rapid corporate archaeology for people who need to get productive fast.

---

## Problem Statement

Organizations accumulate information across multiple disconnected systems (Jira, Confluence, GitHub, SharePoint, Slack, Notion) over 5-10 years. Nothing is deleted, everything is duplicated, search returns hundreds of irrelevant results. New employees and consultants waste 1-2 weeks just finding information before they can be productive.

**The pain:**
- Onboarding takes weeks instead of days
- Consultants bill discovery time instead of delivery time
- Critical knowledge lives in people's heads, not documentation
- Existing enterprise search tools require 6-month implementations and don't solve the context problem

---

## Product Definition

Quaero is a local-first AI knowledge base that ingests documentation from multiple sources, creates a unified searchable index, and provides AI-powered cross-referencing and analysis.

**Core differentiators:**
- **Local-first deployment:** Runs on laptop or internal infrastructure, no cloud dependency
- **Privacy-first architecture:** Dual-mode (offline with llama.cpp, or cloud with Gemini API)
- **Rapid setup:** Hours, not months
- **Cross-source intelligence:** Connects dots across Jira tickets, code repos, wikis, and public information
- **Audit generation:** Automated first-week summaries of projects, backlogs, and technical debt

---

## Target Markets

### Primary: Individual Consultants (Phase 1 - MVP)
**Profile:**
- Technical consultants at large enterprises
- 1-6 month engagements
- Need to be productive in 5-7 days
- Walk into information chaos every engagement
- Budget: $20-50/month (expensable)

**Value proposition:** Cut onboarding from 2 weeks to 2 days. Bill delivery hours, not discovery hours.

### Secondary: Small-Medium Business (Phase 2 - Growth)
**Profile:**
- 10-50 employees
- Distributed information across 3-5 tools
- Too small for enterprise search ($100K+/year)
- High employee turnover or rapid growth
- Budget: $200-500/month

**Value proposition:** New employees productive in week one. Reduce "ask Steve" dependency. Institutional knowledge in a searchable system.

---

## Core Functions

### 1. Multi-Source Ingestion
**Sources:**
- Jira (projects, tickets, comments)
- Confluence (spaces, pages)
- GitHub (repos, PRs, issues, README files)
- Notion (databases, pages)
- SharePoint (documents, lists)
- Local files (markdown, PDF, text)
- Web scraping (public documentation)

**Technical approach:**
- API-based collection where available
- Browser automation for authenticated sources
- File system monitoring for local sources
- Incremental updates (delta sync, not full refresh)
- Markdown storage with metadata (source, timestamp, author, tags)

### 2. Unified Search & Indexing
**Features:**
- Full-text search (SQLite FTS5)
- Metadata filtering (source, date, author, tags)
- Keyword extraction (non-AI: RAKE/TF-IDF)
- Document similarity (embedding-based)
- Cross-reference detection (mentions, links, shared entities)

**Technical stack:**
- SQLite for storage and FTS
- Go for backend processing
- Dual embedding support (local sentence-transformers or Gemini API)

### 3. AI-Powered Analysis
**Capabilities:**
- Natural language queries over document corpus
- Semantic search (beyond keyword matching)
- Document summarization
- Cross-source synthesis ("What's the status of Project X across Jira + GitHub + Slack?")
- Style-based filtering ("Show security concerns" vs "Show business requirements")

**AI integration:**
- API-based (no embedded LLM in codebase)
- Pluggable: Gemini, OpenAI, Anthropic, or local (llama.cpp via HTTP)
- Token optimization: pre-filter with FTS, only send top candidates to LLM

### 4. Audit Generation (Killer Feature)
**"First Week Audit" output:**
- High-priority backlog items (Jira)
- Key architectural decisions (GitHub, Confluence)
- Active vs deprecated systems
- Technical debt indicators
- Critical knowledge gaps
- Who to talk to (frequent contributors, decision makers)
- Onboarding path (documentation gaps, missing context)

**Use cases:**
- Consultant onboarding at new client
- New employee technical ramp-up
- Project health check
- Due diligence for M&A
- Knowledge transfer when key employee leaves

### 5. Human-in-the-Loop Tagging & Context
**Features:**
- Custom document tagging (manual)
- Context groups (collections that span sources)
- Feedback loop (thumbs up/down on AI results)
- Privacy flags (allow_ai_processing, allow_web_enrichment)
- Source prioritization (trust internal docs over public sources)

---

## Technical Architecture

### Stack
- **Language:** Go (performance, single binary deployment)
- **Database:** SQLite (local-first, portable, FTS built-in)
- **AI Integration:** API-based (Gemini, OpenAI, Anthropic, local llama.cpp)
- **Web Interface:** Simple Vue.js or htmx frontend
- **Deployment:** Single binary + SQLite file, or Docker container

### Security & Privacy
- **Offline mode:** All processing local, no external calls
- **Cloud mode:** API calls to chosen LLM provider
- **Data isolation:** Private documents never mixed with public enrichment
- **Governance layer (future):** PII extraction, data classification, audit logs

### Scalability Model
- **Individual:** 1-10K documents, laptop deployment
- **Small team:** 10-100K documents, shared server
- **Medium business:** 100K-1M documents, dedicated infrastructure

---

## Go-to-Market Strategy

### Phase 1: MVP & Validation (Months 1-3)
**Goals:**
- Build core ingestion (Jira, GitHub, Confluence)
- Implement FTS + basic AI query
- Create "First Week Audit" feature
- Test with 5-10 consultants on real engagements

**Steps:**
1. Recruit beta testers from Everest Engineering consultants
2. Provide sanitized data ingestion (no client PII exposure)
3. Iterate on audit output format based on feedback
4. Measure time-to-productivity improvement
5. Document case studies ("Cut onboarding from 10 days to 3")

**Revenue:** None (validation phase)

### Phase 2: Individual Launch (Months 4-6)
**Goals:**
- Polish UX for self-service setup
- Build payment/licensing system
- Launch on Product Hunt, Hacker News, dev communities
- Target: 50 paying individual users

**Steps:**
1. Create demo video (consultant onboarding scenario)
2. Write technical blog posts (architecture, privacy approach)
3. Open-source core, commercial license for enterprise features
4. Pricing: $29/month or $290/year individual license
5. Build community: Discord/Slack for users

**Revenue target:** $1,500/month (50 users × $30)

### Phase 3: SMB Expansion (Months 7-12)
**Goals:**
- Team features (shared instance, multi-user)
- Enterprise connectors (SAML, SSO)
- Target: 10 small business customers

**Steps:**
1. Outbound to SMBs with 10-50 employees
2. Partner with consultancies (Everest, others) to offer as onboarding tool
3. Case study-driven sales ("Company X saved 80 hours/quarter")
4. Pricing: $199/month (5-10 users), $499/month (10-50 users)
5. Annual contracts with implementation support

**Revenue target:** $5,000/month (10 customers × $500 average)

---

## Success Metrics

### Product Metrics
- **Time to first insight:** < 30 minutes from installation to useful query
- **Onboarding acceleration:** 50% reduction in time-to-productivity
- **Query success rate:** 80% of queries return actionable results
- **Daily active usage:** 3-5 queries per user per day

### Business Metrics
- **Customer acquisition cost (CAC):** < $500 (content marketing, word of mouth)
- **Lifetime value (LTV):** $2,000+ (2+ years retention)
- **Churn:** < 10% monthly for individuals, < 5% for teams
- **NPS:** 50+ (strong word-of-mouth growth)

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Enterprise tools add AI search** | High - incumbents have distribution | Focus on ease of setup, local-first privacy, consultant niche |
| **Token costs too high for profitability** | Medium - margins compressed | Pre-filter with FTS, cache results, offer bring-your-own-API-key |
| **Users can't set up connectors** | High - adoption blocked | One-click templates for common setups, video tutorials, professional services |
| **Privacy concerns in beta testing** | High - reputational damage | Strict sanitization rules, opt-in only, clear data handling policies |
| **Can't validate consultant value prop** | Medium - wrong target market | Pivot to SMB if consultant traction is low, or focus on specific vertical |

---

## Funding Requirement

**Runway need:** 6 months to reach Phase 2 revenue

**Burn rate:**
- Personal salary: $8K/month (minimum viable)
- Infrastructure: $500/month (dev tools, hosting, APIs)
- Marketing: $1K/month (content, ads)
- **Total:** $9.5K/month × 6 months = **$57K**

**Potential structures:**
1. **Retainer from Everest:** Quaero development + consultant tool license
2. **Equity investment:** 10-15% for $50-75K
3. **Pre-sales:** 10 annual licenses × $2,500 = $25K upfront
4. **Grants:** SBIR, open-source foundation support

---

## Next Steps (Immediate)

1. **Week 1:** Meeting with Craig (Everest MD) - validate consultant use case, discuss retainer
2. **Week 2:** Recruit 3-5 beta testers from Everest consultants
3. **Week 3:** Build minimum viable audit feature (Jira + GitHub ingestion → summary output)
4. **Week 4:** First real-world test on active engagement
5. **Month 2:** Iterate based on feedback, add Confluence connector
6. **Month 3:** Validate time-to-productivity metrics, document case studies

**Decision point at Month 3:** Go/no-go on Phase 2 launch based on beta feedback and metrics.