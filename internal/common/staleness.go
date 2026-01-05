// Package common provides shared utilities across the application.
package common

import (
	"fmt"
	"time"

	"github.com/ternarybob/quaero/internal/eodhd"
)

// StalenessResult contains the result of a staleness check.
type StalenessResult struct {
	// IsStale indicates whether the cached data is stale and needs refresh.
	IsStale bool
	// NextCheckTime is when to check again if data is not currently stale.
	// This is useful for scheduling the next check.
	NextCheckTime time.Time
	// Reason provides a human-readable explanation for the staleness decision.
	Reason string
}

// CheckTickerStaleness determines if cached ticker data is stale based on
// exchange trading schedules and data availability delays.
//
// Parameters:
//   - docDate: The date of the EOD data we have cached (typically from document metadata)
//   - now: Current time (in UTC)
//   - metadata: Exchange metadata with timezone, close time, holidays, etc.
//
// Returns a StalenessResult indicating whether the data is stale.
func CheckTickerStaleness(docDate time.Time, now time.Time, metadata *eodhd.ExchangeMetadata) StalenessResult {
	// If no metadata, assume always stale (fallback behavior)
	if metadata == nil {
		return StalenessResult{
			IsStale: true,
			Reason:  "no exchange metadata available, assuming stale",
		}
	}

	// Normalize times to UTC
	now = now.UTC()
	docDate = docDate.UTC()

	// Get the last trading day before now
	lastTradingDay := GetLastTradingDay(now, metadata.WorkingDays, metadata.Holidays)

	// Calculate when new data would be available
	dataAvailableTime, err := GetDataAvailableTime(
		lastTradingDay,
		metadata.CloseTime,
		metadata.Timezone,
		metadata.DataDelayMinutes,
	)
	if err != nil {
		// If we can't calculate, assume stale to be safe
		return StalenessResult{
			IsStale: true,
			Reason:  fmt.Sprintf("failed to calculate data availability: %v", err),
		}
	}

	// Normalize doc date to just the date (no time component)
	docDateOnly := time.Date(docDate.Year(), docDate.Month(), docDate.Day(), 0, 0, 0, 0, time.UTC)
	lastTradingDayOnly := time.Date(lastTradingDay.Year(), lastTradingDay.Month(), lastTradingDay.Day(), 0, 0, 0, 0, time.UTC)

	// Data is stale if:
	// 1. doc_date < last_trading_day AND
	// 2. now > data_available_time
	if docDateOnly.Before(lastTradingDayOnly) && now.After(dataAvailableTime) {
		return StalenessResult{
			IsStale: true,
			Reason: fmt.Sprintf(
				"cached data from %s is older than last trading day %s, new data available since %s",
				docDateOnly.Format("2006-01-02"),
				lastTradingDayOnly.Format("2006-01-02"),
				dataAvailableTime.Format("2006-01-02 15:04 MST"),
			),
		}
	}

	// Not stale - calculate next check time
	// If we haven't reached data available time yet, check then
	if now.Before(dataAvailableTime) {
		return StalenessResult{
			IsStale:       false,
			NextCheckTime: dataAvailableTime,
			Reason: fmt.Sprintf(
				"data not yet available for %s, check again at %s",
				lastTradingDayOnly.Format("2006-01-02"),
				dataAvailableTime.Format("2006-01-02 15:04 MST"),
			),
		}
	}

	// Data is fresh - next check after next trading day's data is available
	nextTradingDay := GetNextTradingDay(now, metadata.WorkingDays, metadata.Holidays)
	nextDataAvailableTime, _ := GetDataAvailableTime(
		nextTradingDay,
		metadata.CloseTime,
		metadata.Timezone,
		metadata.DataDelayMinutes,
	)

	return StalenessResult{
		IsStale:       false,
		NextCheckTime: nextDataAvailableTime,
		Reason: fmt.Sprintf(
			"data is fresh (doc: %s, last trading: %s), next check at %s",
			docDateOnly.Format("2006-01-02"),
			lastTradingDayOnly.Format("2006-01-02"),
			nextDataAvailableTime.Format("2006-01-02 15:04 MST"),
		),
	}
}

// IsWorkingDay checks if a given date is a working day for the exchange.
// It accounts for both weekends (based on workingDays) and holidays.
func IsWorkingDay(t time.Time, workingDays []time.Weekday, holidays []time.Time) bool {
	// Check if it's a working weekday
	dayOfWeek := t.Weekday()
	isWorkDay := false
	for _, wd := range workingDays {
		if wd == dayOfWeek {
			isWorkDay = true
			break
		}
	}
	if !isWorkDay {
		return false
	}

	// Check if it's a holiday
	tDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	for _, h := range holidays {
		hDate := time.Date(h.Year(), h.Month(), h.Day(), 0, 0, 0, 0, time.UTC)
		if tDate.Equal(hDate) {
			return false
		}
	}

	return true
}

// GetLastTradingDay returns the most recent trading day on or before the given time.
// It walks backwards from the given time until finding a working day.
func GetLastTradingDay(t time.Time, workingDays []time.Weekday, holidays []time.Time) time.Time {
	// Start from the current date
	current := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)

	// Walk backwards up to 10 days to find a trading day
	// (handles long holiday periods like Christmas/New Year)
	for i := 0; i < 10; i++ {
		if IsWorkingDay(current, workingDays, holidays) {
			return current
		}
		current = current.AddDate(0, 0, -1)
	}

	// Fallback: return the original date if no trading day found
	// This shouldn't happen with normal exchange schedules
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// GetNextTradingDay returns the next trading day after the given time.
// It walks forward from the given time until finding a working day.
func GetNextTradingDay(t time.Time, workingDays []time.Weekday, holidays []time.Time) time.Time {
	// Start from the next day
	current := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)

	// Walk forward up to 10 days to find a trading day
	for i := 0; i < 10; i++ {
		if IsWorkingDay(current, workingDays, holidays) {
			return current
		}
		current = current.AddDate(0, 0, 1)
	}

	// Fallback: return next day if no trading day found
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
}

// GetDataAvailableTime calculates when new EOD data would be available for a given trading day.
// It combines the trading day with the exchange's close time, converts to the exchange timezone,
// adds the data delay, and returns the result in UTC.
func GetDataAvailableTime(tradingDay time.Time, closeTime string, timezone string, delayMinutes int) (time.Time, error) {
	// Load the exchange timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %s: %w", timezone, err)
	}

	// Parse close time
	hour, min := 16, 0 // Default 4:00 PM
	if closeTime != "" {
		_, err := fmt.Sscanf(closeTime, "%d:%d", &hour, &min)
		if err != nil {
			// Try just hours
			fmt.Sscanf(closeTime, "%d", &hour)
		}
	}

	// Create close time in exchange timezone
	closeDateTime := time.Date(
		tradingDay.Year(), tradingDay.Month(), tradingDay.Day(),
		hour, min, 0, 0,
		loc,
	)

	// Add delay and convert to UTC
	dataAvailableTime := closeDateTime.Add(time.Duration(delayMinutes) * time.Minute).UTC()

	return dataAvailableTime, nil
}

// DefaultExchangeMetadata returns sensible default metadata for unknown exchanges.
// This is used as a fallback when exchange details cannot be fetched.
func DefaultExchangeMetadata(exchangeCode string) *eodhd.ExchangeMetadata {
	// Get defaults from the eodhd package maps
	timezone := "UTC"
	if tz, ok := eodhd.DefaultExchangeTimezones[exchangeCode]; ok {
		timezone = tz
	}

	closeTime := "16:00"
	if ct, ok := eodhd.DefaultCloseTime[exchangeCode]; ok {
		closeTime = ct
	}

	return &eodhd.ExchangeMetadata{
		Code:             exchangeCode,
		Name:             exchangeCode + " Exchange",
		Timezone:         timezone,
		CloseTime:        closeTime,
		DataDelayMinutes: eodhd.GetDataDelay(exchangeCode),
		WorkingDays:      eodhd.DefaultWorkingDays(),
		Holidays:         []time.Time{}, // No holidays known
		LastFetched:      time.Now().UTC(),
	}
}
