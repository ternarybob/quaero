package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/storage/sqlite"
)

func main() {
	fmt.Println("=== Cookie Authentication Test ===")
	fmt.Println()

	// 1. Setup logger
	logger := arbor.NewLogger()

	// 2. Connect to database
	config := &common.SQLiteConfig{
		Path: "../../bin/data/quaero.db",
	}
	db, err := sqlite.NewSQLiteDB(logger, config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	authStorage := sqlite.NewAuthStorage(db, logger)

	// 2. Load auth credentials
	ctx := context.Background()
	auths, err := authStorage.ListCredentials(ctx)
	if err != nil {
		log.Fatalf("Failed to list credentials: %v", err)
	}

	if len(auths) == 0 {
		log.Fatal("No auth credentials found in database")
	}

	fmt.Printf("Found %d auth credential(s)\n", len(auths))
	authCreds := auths[0]
	fmt.Printf("Using auth: %s (site: %s)\n", authCreds.Name, authCreds.SiteDomain)
	fmt.Println()

	// 3. Parse cookies
	var cookies []*interfaces.AtlassianExtensionCookie
	if err := json.Unmarshal(authCreds.Cookies, &cookies); err != nil {
		log.Fatalf("Failed to unmarshal cookies: %v", err)
	}

	fmt.Printf("Loaded %d cookies from database\n", len(cookies))
	for i, c := range cookies {
		fmt.Printf("  [%d] %s (domain=%s, path=%s, expires=%s)\n",
			i+1, c.Name, c.Domain, c.Path, time.Unix(c.Expires, 0).Format("2006-01-02"))
	}
	fmt.Println()

	// 4. Test OLD method (BROKEN) - all cookies set on base URL
	fmt.Println("--- Test 1: OLD Method (setting all cookies on base URL) ---")
	testOldMethod(authCreds, cookies)
	fmt.Println()

	// 5. Test NEW method (FIXED) - cookies grouped by domain
	fmt.Println("--- Test 2: NEW Method (cookies grouped by domain) ---")
	testNewMethod(authCreds, cookies)
}

func testOldMethod(authCreds *models.AuthCredentials, cookies []*interfaces.AtlassianExtensionCookie) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 1 {
				fmt.Printf("  REDIRECT: %s -> %s\n", via[len(via)-1].URL.String(), req.URL.String())
			}
			return nil
		},
	}

	baseURL, _ := url.Parse(authCreds.BaseURL)

	// Convert to http.Cookie
	httpCookies := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		httpCookies = append(httpCookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  time.Unix(c.Expires, 0),
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}

	// OLD METHOD: Set ALL cookies on base URL
	client.Jar.SetCookies(baseURL, httpCookies)

	// Check what cookies are actually set
	retrievedCookies := client.Jar.Cookies(baseURL)
	fmt.Printf("  Cookies set on jar for %s: %d\n", baseURL.String(), len(retrievedCookies))
	for _, c := range retrievedCookies {
		fmt.Printf("    - %s=%s (domain=%s)\n", c.Name, c.Value[:min(20, len(c.Value))], c.Domain)
	}

	// Make request
	testRequest(client, authCreds.BaseURL)
}

func testNewMethod(authCreds *models.AuthCredentials, cookies []*interfaces.AtlassianExtensionCookie) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 1 {
				fmt.Printf("  REDIRECT: %s -> %s\n", via[len(via)-1].URL.String(), req.URL.String())
			}
			return nil
		},
	}

	baseURL, _ := url.Parse(authCreds.BaseURL)

	// NEW METHOD: Group cookies by domain
	cookiesByDomain := make(map[string][]*http.Cookie)
	for _, c := range cookies {
		// Fix expiration: treat expired cookies as session cookies
		var expires time.Time
		if c.Expires > 0 {
			expires = time.Unix(c.Expires, 0)
			// If cookie expired more than a day ago, treat as session cookie
			if expires.Before(time.Now().Add(-24 * time.Hour)) {
				fmt.Printf("    Cookie %s expired on %s, treating as session cookie\n", c.Name, expires.Format("2006-01-02"))
				expires = time.Time{} // Zero value = session cookie
			}
		}

		httpCookie := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  expires,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}

		// Use cookie's domain, removing leading dot if present
		domain := strings.TrimPrefix(c.Domain, ".")
		if domain == "" {
			domain = baseURL.Host
		}

		cookiesByDomain[domain] = append(cookiesByDomain[domain], httpCookie)
	}

	// Set cookies for each domain
	fmt.Printf("  Setting cookies for %d domains:\n", len(cookiesByDomain))
	for domain, domainCookies := range cookiesByDomain {
		domainURL, _ := url.Parse(fmt.Sprintf("https://%s/", domain))
		client.Jar.SetCookies(domainURL, domainCookies)
		fmt.Printf("    - %s: %d cookies\n", domain, len(domainCookies))
	}

	// Check what cookies are actually set
	retrievedCookies := client.Jar.Cookies(baseURL)
	fmt.Printf("  Cookies available for %s: %d\n", baseURL.String(), len(retrievedCookies))
	for _, c := range retrievedCookies {
		fmt.Printf("    - %s=%s...\n", c.Name, c.Value[:min(20, len(c.Value))])
	}

	// Make request
	testRequest(client, authCreds.BaseURL)
}

func testRequest(client *http.Client, targetURL string) {
	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("User-Agent", "Quaero/1.0 (Cookie Test)")

	fmt.Printf("  Making request to: %s\n", targetURL)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Response: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Printf("  Final URL: %s\n", resp.Request.URL.String())
	fmt.Printf("  Body length: %d bytes\n", len(body))

	// Check if we got redirected to login
	if strings.Contains(resp.Request.URL.String(), "id.atlassian.com/login") {
		fmt.Printf("  ❌ FAILED: Redirected to login page (authentication failed)\n")
	} else {
		fmt.Printf("  ✅ SUCCESS: Stayed on authenticated page\n")
		// Print first 200 chars of title
		titleStart := strings.Index(string(body), "<title>")
		titleEnd := strings.Index(string(body), "</title>")
		if titleStart != -1 && titleEnd != -1 {
			title := string(body[titleStart+7 : titleEnd])
			fmt.Printf("  Page title: %s\n", title)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
