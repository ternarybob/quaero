// -----------------------------------------------------------------------
// Timing test helpers for worker timing assertions
// Used by market worker tests to verify timing data collection
// -----------------------------------------------------------------------

package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/internal/models"
)

// TimingRecordsResponse represents the response from GET /api/timing
type TimingRecordsResponse struct {
	Records []*models.TimingRecord `json:"records"`
	Total   int                    `json:"total"`
	Limit   int                    `json:"limit"`
	Offset  int                    `json:"offset"`
}

// TimingStatsResponse represents the response from GET /api/timing/stats
type TimingStatsResponse struct {
	ByWorker   map[string]WorkerTimingStats `json:"by_worker"`
	TotalCount int                          `json:"total_count"`
	TimeRange  *TimeRange                   `json:"time_range,omitempty"`
}

// WorkerTimingStats holds aggregated stats for a worker type
type WorkerTimingStats struct {
	Count       int     `json:"count"`
	AvgMs       int64   `json:"avg_ms"`
	MinMs       int64   `json:"min_ms"`
	MaxMs       int64   `json:"max_ms"`
	SuccessRate float64 `json:"success_rate"`
}

// TimeRange represents a time range
type TimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// AssertTimingRecorded checks that at least one timing record exists for the worker type
func AssertTimingRecorded(t *testing.T, helper *HTTPTestHelper, workerType string) {
	t.Helper()
	records := GetTimingRecords(t, helper, workerType)
	require.NotEmpty(t, records, "Expected at least one timing record for worker type %s", workerType)
}

// GetTimingRecords retrieves timing records for a worker type
func GetTimingRecords(t *testing.T, helper *HTTPTestHelper, workerType string) []*models.TimingRecord {
	t.Helper()

	url := fmt.Sprintf("%s/api/timing?worker_type=%s", helper.BaseURL, workerType)
	resp, err := helper.Client.Get(url)
	require.NoError(t, err, "Failed to get timing records")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK from timing endpoint")

	var response TimingRecordsResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err, "Failed to decode timing records response")

	return response.Records
}

// AssertTimingPhases checks that specific phases were recorded in the timing record
func AssertTimingPhases(t *testing.T, record *models.TimingRecord, expectedPhases []string) {
	t.Helper()

	require.NotNil(t, record, "TimingRecord should not be nil")
	require.NotNil(t, record.Phases, "Phases map should not be nil")

	for _, phase := range expectedPhases {
		_, exists := record.Phases[phase]
		require.True(t, exists, "Expected phase %s to be recorded", phase)
	}
}

// GetTimingStats retrieves timing statistics
func GetTimingStats(t *testing.T, helper *HTTPTestHelper) *TimingStatsResponse {
	t.Helper()

	url := fmt.Sprintf("%s/api/timing/stats", helper.BaseURL)
	resp, err := helper.Client.Get(url)
	require.NoError(t, err, "Failed to get timing stats")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK from timing stats endpoint")

	var response TimingStatsResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err, "Failed to decode timing stats response")

	return &response
}

// AssertTimingSuccess checks that a timing record has success status
func AssertTimingSuccess(t *testing.T, record *models.TimingRecord) {
	t.Helper()
	require.NotNil(t, record)
	require.Equal(t, "success", record.Status, "Expected timing record status to be 'success'")
	require.Empty(t, record.Error, "Expected no error in successful timing record")
}

// AssertTimingFailed checks that a timing record has failed status
func AssertTimingFailed(t *testing.T, record *models.TimingRecord) {
	t.Helper()
	require.NotNil(t, record)
	require.Equal(t, "failed", record.Status, "Expected timing record status to be 'failed'")
}

// AssertTimingDuration checks that timing duration is within reasonable bounds
func AssertTimingDuration(t *testing.T, record *models.TimingRecord, minMs, maxMs int64) {
	t.Helper()
	require.NotNil(t, record)
	require.GreaterOrEqual(t, record.TotalMs, minMs, "Timing duration should be >= %dms", minMs)
	require.LessOrEqual(t, record.TotalMs, maxMs, "Timing duration should be <= %dms", maxMs)
}
