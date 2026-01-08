// Package announcements provides services for fetching and classifying company announcements.
package announcements

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

// ASXProvider fetches announcements from ASX sources (Markit API and HTML scraping).
type ASXProvider struct {
	logger     arbor.ILogger
	httpClient *http.Client
}

// NewASXProvider creates a new ASX announcement provider.
func NewASXProvider(logger arbor.ILogger, httpClient *http.Client) *ASXProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &ASXProvider{
		logger:     logger,
		httpClient: httpClient,
	}
}

// Name returns the provider name.
func (p *ASXProvider) Name() string {
	return "ASX"
}

// SupportsExchange returns true for ASX exchange.
func (p *ASXProvider) SupportsExchange(exchange string) bool {
	return strings.EqualFold(exchange, "ASX") || strings.EqualFold(exchange, "AU")
}

// FetchAnnouncements retrieves announcements from ASX sources.
// Tries HTML scraping first (more complete), then Markit API as fallback.
func (p *ASXProvider) FetchAnnouncements(ctx context.Context, ticker common.Ticker, period string, limit int) ([]RawAnnouncement, error) {
	// For Y1 or longer periods, try HTML page which returns full year data
	if period == "Y1" || period == "Y3" || period == "Y5" || period == "Y2" {
		anns, err := p.fetchFromHTML(ctx, ticker.Code, period, limit)
		if err != nil {
			p.logger.Warn().Err(err).Msg("ASX HTML fetch failed, trying Markit API")
		} else if len(anns) > 0 {
			p.logger.Debug().Int("count", len(anns)).Msg("Fetched announcements from ASX HTML")
			return anns, nil
		}
	}

	// Fallback to Markit Digital API
	return p.fetchFromMarkitAPI(ctx, ticker.Code, period, limit)
}

// marketAPIResponse represents the Markit API response structure.
type marketAPIResponse struct {
	Data struct {
		Items []struct {
			Date             string `json:"date"`
			Headline         string `json:"headline"`
			AnnouncementType string `json:"type"`
			URL              string `json:"url"`
			DocumentKey      string `json:"documentKey"`
			FileSize         int    `json:"size"`
			IsPriceSensitive bool   `json:"priceSensitive"`
		} `json:"items"`
	} `json:"data"`
}

// fetchFromMarkitAPI fetches from Markit Digital API (ASX-specific endpoint).
func (p *ASXProvider) fetchFromMarkitAPI(ctx context.Context, code, period string, limit int) ([]RawAnnouncement, error) {
	url := fmt.Sprintf("https://asx.api.markitdigital.com/asx-research/1.0/companies/%s/announcements",
		strings.ToLower(code))

	p.logger.Debug().Str("url", url).Msg("Fetching announcements from Markit API")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp marketAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	cutoffDate := CalculateCutoffDate(period)

	var anns []RawAnnouncement
	for _, item := range apiResp.Data.Items {
		if limit > 0 && len(anns) >= limit {
			break
		}

		date, err := time.Parse(time.RFC3339, item.Date)
		if err != nil {
			date, err = time.Parse("2006-01-02T15:04:05", item.Date)
			if err != nil {
				date = time.Now()
			}
		}

		if !cutoffDate.IsZero() && date.Before(cutoffDate) {
			continue
		}

		pdfURL := item.URL
		if pdfURL == "" && item.DocumentKey != "" {
			pdfURL = fmt.Sprintf("https://www.asx.com.au/asxpdf/%s", item.DocumentKey)
		}

		ann := RawAnnouncement{
			Date:           date,
			Headline:       item.Headline,
			PDFURL:         pdfURL,
			DocumentKey:    item.DocumentKey,
			PriceSensitive: item.IsPriceSensitive,
			Type:           item.AnnouncementType,
		}

		anns = append(anns, ann)
	}

	p.logger.Debug().Int("count", len(anns)).Msg("Fetched announcements from Markit API")
	return anns, nil
}

// fetchFromHTML scrapes announcements from ASX HTML page.
// URL: https://www.asx.com.au/asx/v2/statistics/announcements.do?by=asxCode&asxCode={CODE}&timeframe=Y&year={YEAR}
// This provides 50+ announcements per year vs ~5 from the Markit API.
func (p *ASXProvider) fetchFromHTML(ctx context.Context, code, period string, limit int) ([]RawAnnouncement, error) {
	currentYear := time.Now().Year()
	var allAnns []RawAnnouncement

	// Determine years to fetch based on period
	// For Y1, fetch 2 years to ensure we get 12 months of data
	// (e.g., if today is Jan 2026, we need 2025 + 2026 data)
	yearsToFetch := 2
	switch period {
	case "Y2":
		yearsToFetch = 3
	case "Y3":
		yearsToFetch = 4
	case "Y5":
		yearsToFetch = 6
	}

	for yearOffset := 0; yearOffset < yearsToFetch; yearOffset++ {
		year := currentYear - yearOffset

		// ASX announcement URL
		url := fmt.Sprintf("https://www.asx.com.au/asx/v2/statistics/announcements.do?by=asxCode&asxCode=%s&timeframe=Y&year=%d",
			strings.ToUpper(code), year)

		p.logger.Debug().Str("url", url).Int("year", year).Msg("Fetching announcements from HTML")

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			p.logger.Warn().Err(err).Int("year", year).Msg("Failed to fetch HTML page")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			p.logger.Warn().Int("status", resp.StatusCode).Int("year", year).Msg("Non-OK status from ASX HTML")
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		anns := p.parseAnnouncementsHTML(string(body), code, year)
		allAnns = append(allAnns, anns...)

		p.logger.Info().
			Str("code", code).
			Int("year", year).
			Int("count", len(anns)).
			Msg("Parsed announcements from HTML")
	}

	// Sort by date descending
	sort.Slice(allAnns, func(i, j int) bool {
		return allAnns[i].Date.After(allAnns[j].Date)
	})

	// Apply limit if specified
	if limit > 0 && len(allAnns) > limit {
		allAnns = allAnns[:limit]
	}

	return allAnns, nil
}

// parseAnnouncementsHTML parses HTML table rows to extract announcement data.
// HTML structure:
// <tr>
//
//	<td>09/12/2025<br><span class="dates-time">12:24 pm</span></td>
//	<td class="pricesens"><!-- img if price sensitive --></td>
//	<td><a href="/asx/v2/statistics/displayAnnouncement.do?display=pdf&idsId=03038930">Headline</a></td>
//
// </tr>
func (p *ASXProvider) parseAnnouncementsHTML(html, code string, year int) []RawAnnouncement {
	var anns []RawAnnouncement

	// Extract tbody content
	tbodyPattern := regexp.MustCompile(`(?s)<tbody>(.*?)</tbody>`)
	tbodyMatch := tbodyPattern.FindStringSubmatch(html)
	if len(tbodyMatch) < 2 {
		p.logger.Debug().Msg("No tbody found in HTML")
		return anns
	}
	tbody := tbodyMatch[1]

	// Match individual rows - pattern: date | price sensitive | headline with link
	rowPattern := regexp.MustCompile(`(?s)<tr>\s*<td>\s*(\d{2}/\d{2}/\d{4})<br>\s*<span class="dates-time">([^<]+)</span>\s*</td>\s*<td[^>]*>\s*(.*?)\s*</td>\s*<td>\s*(.*?)\s*</td>\s*</tr>`)

	rows := rowPattern.FindAllStringSubmatch(tbody, -1)

	for _, row := range rows {
		if len(row) < 5 {
			continue
		}

		dateStr := row[1]  // e.g., "09/12/2025"
		timeStr := row[2]  // e.g., "12:24 pm"
		priceCol := row[3] // Contains img tag if price sensitive
		headlineCol := row[4]

		// Parse date and time
		dateTime, err := time.Parse("02/01/2006 3:04 pm", dateStr+" "+strings.TrimSpace(timeStr))
		if err != nil {
			dateTime, err = time.Parse("02/01/2006", dateStr)
			if err != nil {
				continue
			}
		}

		// Check price sensitive
		priceSensitive := strings.Contains(priceCol, "icon-price-sensitive")

		// Extract headline and PDF URL
		headlinePattern := regexp.MustCompile(`href="([^"]+)"[^>]*>([^<]+)`)
		headlineMatch := headlinePattern.FindStringSubmatch(headlineCol)

		var headline, pdfPath, idsId string
		if len(headlineMatch) >= 3 {
			pdfPath = headlineMatch[1]
			headline = strings.TrimSpace(headlineMatch[2])
		}

		// Extract idsId from URL for document key
		idsIdPattern := regexp.MustCompile(`idsId=(\d+)`)
		idsIdMatch := idsIdPattern.FindStringSubmatch(pdfPath)
		if len(idsIdMatch) >= 2 {
			idsId = idsIdMatch[1]
		}

		// Build full PDF URL
		pdfURL := ""
		if pdfPath != "" {
			pdfURL = "https://www.asx.com.au" + pdfPath
		}

		// Extract page count for announcement type hint
		pagePattern := regexp.MustCompile(`<span class="page">(\d+)`)
		pageMatch := pagePattern.FindStringSubmatch(headlineCol)
		pageCount := 0
		if len(pageMatch) >= 2 {
			pageCount, _ = strconv.Atoi(pageMatch[1])
		}

		// Infer type from headline
		annType := inferAnnouncementType(headline, pageCount)

		anns = append(anns, RawAnnouncement{
			Date:           dateTime,
			Headline:       headline,
			PDFURL:         pdfURL,
			DocumentKey:    idsId,
			PriceSensitive: priceSensitive,
			Type:           annType,
		})
	}

	return anns
}

// stripHTML removes HTML tags from a string.
func stripHTML(s string) string {
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	s = tagPattern.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return strings.TrimSpace(s)
}

// inferAnnouncementType guesses announcement type from headline.
func inferAnnouncementType(headline string, pageCount int) string {
	headlineUpper := strings.ToUpper(headline)

	typePatterns := []struct {
		keywords []string
		annType  string
	}{
		{[]string{"TRADING HALT", "SUSPENSION"}, "Trading Halt"},
		{[]string{"QUARTERLY", "ACTIVITIES REPORT", "4C"}, "Quarterly Report"},
		{[]string{"HALF YEAR", "HALF-YEAR", "1H", "H1"}, "Half Year Report"},
		{[]string{"ANNUAL REPORT", "FULL YEAR", "FY"}, "Annual Report"},
		{[]string{"DIVIDEND"}, "Dividend"},
		{[]string{"AGM", "GENERAL MEETING"}, "Meeting"},
		{[]string{"APPENDIX"}, "Appendix"},
		{[]string{"DIRECTOR"}, "Director Related"},
		{[]string{"SUBSTANTIAL", "HOLDER"}, "Substantial Holder"},
	}

	for _, tp := range typePatterns {
		for _, kw := range tp.keywords {
			if strings.Contains(headlineUpper, kw) {
				return tp.annType
			}
		}
	}

	if pageCount > 20 {
		return "Report"
	}
	return "Announcement"
}
