I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

The cookie injection flow exists in `enhanced_crawler_executor_auth.go` with the `injectAuthCookies` method already implementing comprehensive logging using the 🔐 emoji prefix. However, critical diagnostic gaps prevent understanding why cookies aren't being used:

**Existing Logging (Good):**
- Auth credential loading and validation
- Cookie count and preparation details
- Individual cookie injection attempts with success/fail counts

**Missing Diagnostics (Critical Gaps):**
1. **No verification after injection** - Cookies are set but never read back to confirm they persisted
2. **No domain comparison** - Target URL domain vs cookie domains not logged for mismatch detection
3. **No network monitoring** - `renderPageWithChromeDp` doesn't log what cookies are actually sent with requests
4. **No ChromeDP network domain enablement** - Required for cookie operations but not explicitly enabled

**Key Files:**
- `internal/jobs/processor/enhanced_crawler_executor_auth.go` - Cookie injection logic (300 lines)
- `internal/jobs/processor/enhanced_crawler_executor.go` - Main execution flow with `renderPageWithChromeDp` method (808 lines)

**ChromeDP API Pattern (from web search):**
```
network.Enable() → network.SetCookie() → network.GetCookies() → chromedp.Navigate()
```

Current implementation skips `network.Enable()` and `network.GetCookies()` verification steps.

### Approach

## Implementation Strategy

Add comprehensive diagnostic logging throughout the cookie injection and page rendering flow without changing functional behavior. Use ChromeDP's network API to verify cookies after injection and monitor what cookies are sent with requests.

**Three-Phase Approach:**

1. **Phase 1: Pre-injection diagnostics** - Log target URL domain parsing and cookie domain comparison before injection
2. **Phase 2: Post-injection verification** - Enable network domain, inject cookies, then read them back to verify persistence
3. **Phase 3: Request-time monitoring** - Add logging in `renderPageWithChromeDp` to show cookies being sent with navigation requests

**Key Technical Decisions:**

- Use `network.Enable()` before cookie operations (ChromeDP best practice)
- Use `network.GetCookies().WithURLs([]string{targetURL}).Do(ctx)` to verify cookies applicable to target URL
- Log detailed cookie attributes (name, domain, path, secure, httpOnly, sameSite) at each stage
- Compare injected cookies vs verified cookies to detect mismatches
- Add network request logging in `renderPageWithChromeDp` to show actual cookie headers

**Breaking Changes:** None - all changes are additive logging only

### Reasoning

I explored the repository structure and read the two main files handling cookie injection (`enhanced_crawler_executor_auth.go` and `enhanced_crawler_executor.go`). I searched for existing ChromeDP network API usage patterns in the codebase and found similar cookie injection in `html_scraper.go`. I performed a web search to understand the correct ChromeDP API pattern for cookie verification using `network.GetCookies()`. I reviewed the documentation in `docs/update-chrome-extension-generic-crawl/` and `docs/validate-and-update-chrome-extension/` to understand the complete auth capture and crawl workflow.

## Mermaid Diagram

sequenceDiagram
    participant Executor as EnhancedCrawlerExecutor
    participant Auth as injectAuthCookies
    participant ChromeDP as ChromeDP Browser
    participant Network as Network API
    participant Render as renderPageWithChromeDp

    Note over Executor,Render: Phase 1: Pre-Injection Diagnostics
    Executor->>Auth: injectAuthCookies(ctx, browserCtx, parentJobID, targetURL)
    Auth->>Auth: Parse targetURL domain
    Auth->>Auth: 🔐 LOG: Target domain extracted
    Auth->>Auth: Load auth credentials from storage
    Auth->>Auth: Unmarshal cookies from JSON
    loop For each cookie
        Auth->>Auth: 🔐 LOG: Compare cookie domain vs target domain
        Auth->>Auth: 🔐 LOG: Domain match/mismatch detected
    end

    Note over Auth,Network: Phase 2: Injection & Verification
    Auth->>ChromeDP: network.Enable()
    ChromeDP-->>Auth: Network domain enabled
    Auth->>Auth: 🔐 LOG: Network domain enabled
    
    loop For each cookie
        Auth->>Network: network.SetCookie(name, value, domain, path, ...)
        Network-->>Auth: Cookie set (success/fail)
        Auth->>Auth: 🔐 LOG: Cookie injection result
    end
    
    Auth->>Network: network.GetCookies().WithURLs([targetURL])
    Network-->>Auth: Return applicable cookies
    Auth->>Auth: 🔐 LOG: Verified cookie count
    
    loop For each verified cookie
        Auth->>Auth: 🔐 LOG: Cookie details (name, domain, path, flags)
    end
    
    Auth->>Auth: Compare injected vs verified cookies
    alt Mismatch detected
        Auth->>Auth: 🔐 ERROR: Cookie injection/verification mismatch
    else All cookies verified
        Auth->>Auth: 🔐 SUCCESS: All cookies verified
    end
    
    Auth-->>Executor: Return (cookies injected)

    Note over Executor,Render: Phase 3: Request-Time Monitoring
    Executor->>Render: renderPageWithChromeDp(ctx, browserCtx, url)
    Render->>Network: network.GetCookies().WithURLs([url])
    Network-->>Render: Return cookies for URL
    Render->>Render: LOG: Cookies before navigation (count, details)
    
    alt No cookies found
        Render->>Render: WARNING: Navigating without authentication
    end
    
    Render->>ChromeDP: chromedp.Navigate(url)
    Note over ChromeDP: Browser sends HTTP request
    ChromeDP->>Render: EventRequestWillBeSent (with Cookie header)
    Render->>Render: LOG: Actual cookies sent in request
    
    ChromeDP-->>Render: Navigation complete
    
    Render->>Network: network.GetCookies().WithURLs([url])
    Network-->>Render: Return cookies after navigation
    Render->>Render: LOG: Cookies after navigation
    
    alt Cookie count changed
        Render->>Render: WARNING: Cookies cleared during navigation
    end
    
    Render-->>Executor: Return (htmlContent, statusCode)

## Proposed File Changes

### internal\jobs\processor\enhanced_crawler_executor_auth.go(MODIFY)

**Add Pre-Injection Domain Diagnostics (after line 169):**

After unmarshaling cookies and before converting to ChromeDP format, add detailed logging to compare target URL domain with cookie domains:

- Parse `targetURL` to extract domain/host
- Log target URL domain with 🔐 prefix
- For each extension cookie, log a comparison showing:
  - Cookie name
  - Cookie domain (original from extension)
  - Whether cookie domain matches target URL domain
  - Whether cookie domain is a parent domain (e.g., `.example.com` matches `subdomain.example.com`)
  - Any domain mismatches that might prevent cookie from being sent

This helps diagnose domain mismatch issues before injection.

**Add Network Domain Enablement (before line 241):**

Before the `chromedp.Run` block that injects cookies, add a separate `chromedp.Run` call to enable the network domain:

- Call `network.Enable()` using `chromedp.Run(browserCtx, network.Enable())`
- Log success/failure with 🔐 prefix
- This is required for `network.GetCookies()` to work properly

**Add Post-Injection Cookie Verification (after line 281):**

After the cookie injection `chromedp.Run` block completes successfully, add a new `chromedp.Run` block to verify cookies were actually set:

- Use `network.GetCookies().WithURLs([]string{targetURL}).Do(ctx)` to read back cookies applicable to the target URL
- Log the count of cookies returned by GetCookies
- For each returned cookie, log detailed attributes:
  - Name, value (truncated to first 20 chars for security)
  - Domain, path
  - Secure, httpOnly, sameSite flags
  - Expiration timestamp if set
- Compare injected cookie count vs verified cookie count
- If counts don't match, log a WARNING with 🔐 prefix indicating potential cookie injection failure
- If specific cookies are missing, log which ones failed to persist

This verification step is critical for diagnosing whether cookies are actually being set in the browser.

**Add Cookie Mismatch Detection:**

After verification, compare the list of injected cookies with the list of verified cookies:

- Create a map of injected cookie names for quick lookup
- For each verified cookie, check if it was in the injected list
- For each injected cookie, check if it appears in the verified list
- Log any discrepancies with 🔐 ERROR prefix:
  - Cookies that were injected but not verified (failed to persist)
  - Cookies that were verified but not injected (unexpected)
  - Cookies with different values between injection and verification

This helps identify which specific cookies are failing.

**Enhanced Error Logging:**

Update the existing error logging at line 283-286 to include more context:

- Log the specific ChromeDP error message
- Log the target URL that was being processed
- Log the number of cookies that were attempted
- Log the browser context state (cancelled, deadline exceeded, etc.)

### internal\jobs\processor\enhanced_crawler_executor.go(MODIFY)

References: 

- internal\jobs\processor\enhanced_crawler_executor_auth.go(MODIFY)

**Add Cookie Monitoring in renderPageWithChromeDp (line 543):**

Enhance the `renderPageWithChromeDp` method to log what cookies are actually being sent with the navigation request:

**Before Navigation (after line 552):**

Before the `chromedp.Run` block that navigates to the URL, add a separate `chromedp.Run` call to read and log cookies:

- Parse the target URL to extract domain
- Log the target URL domain being navigated to
- Use `network.GetCookies().WithURLs([]string{url}).Do(ctx)` to get cookies that will be sent
- Log the count of cookies applicable to this URL
- For each cookie, log:
  - Cookie name
  - Domain and path
  - Whether it's secure/httpOnly
  - Whether it matches the target URL domain
- If no cookies are found, log a WARNING indicating navigation will proceed without authentication

This shows what cookies ChromeDP thinks are applicable BEFORE navigation.

**During Navigation (within chromedp.Run at line 555):**

Add network request monitoring to the existing `chromedp.Run` block:

- Before `chromedp.Navigate(url)`, add `network.Enable()` if not already enabled
- Add a `chromedp.ActionFunc` that sets up a network request listener using `network.EventRequestWillBeSent`
- In the listener, log:
  - Request URL
  - Request headers (specifically the Cookie header)
  - Whether cookies are present in the request
- This shows what cookies are ACTUALLY sent with the HTTP request

**After Navigation (after line 561):**

After the navigation completes, add logging to verify cookies persisted:

- Use `network.GetCookies().WithURLs([]string{url}).Do(ctx)` again to check if cookies still exist
- Log if cookie count changed during navigation
- Log a WARNING if cookies were present before navigation but missing after
- This detects if cookies are being cleared during navigation

**Error Context Enhancement:**

Update error logging at lines 563-569 to include cookie-related context:

- Log whether cookies were present before navigation
- Log the cookie count that was expected
- Log any network errors related to cookie handling
- This helps correlate navigation failures with cookie issues

**Add Cookie Domain Comparison Logging:**

In the pre-navigation cookie check, add detailed domain matching logic:

- For each cookie, compare its domain with the target URL's domain
- Log whether the cookie domain is an exact match, parent domain, or mismatch
- Log any cookies that won't be sent due to domain mismatch
- Log any cookies that won't be sent due to secure flag mismatch (http vs https)
- This helps diagnose why cookies might not be sent even if they're set