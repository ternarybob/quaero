package models

import (
	"testing"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces/jobtypes"
)

func TestCrawlJob_GetStatusReport(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		job            *CrawlJob
		childStats     *jobtypes.JobChildStats
		expectedReport *jobtypes.JobStatusReport
	}{
		{
			name: "parent job with no children",
			job: &CrawlJob{
				ID:        "parent-1",
				ParentID:  "",
				Status:    JobStatusRunning,
				CreatedAt: now,
			},
			childStats: nil,
			expectedReport: &jobtypes.JobStatusReport{
				Status:            "running",
				ChildCount:        0,
				CompletedChildren: 0,
				FailedChildren:    0,
				RunningChildren:   0,
				ProgressText:      "No child jobs spawned yet",
				Errors:            []string{},
				Warnings:          []string{},
			},
		},
		{
			name: "parent job with children - all completed",
			job: &CrawlJob{
				ID:        "parent-2",
				ParentID:  "",
				Status:    JobStatusCompleted,
				CreatedAt: now,
			},
			childStats: &jobtypes.JobChildStats{
				ChildCount:        10,
				CompletedChildren: 10,
				FailedChildren:    0,
			},
			expectedReport: &jobtypes.JobStatusReport{
				Status:            "completed",
				ChildCount:        10,
				CompletedChildren: 10,
				FailedChildren:    0,
				RunningChildren:   0,
				ProgressText:      "10 completed (Total: 10)",
				Errors:            []string{},
				Warnings:          []string{},
			},
		},
		{
			name: "parent job with children - mixed status",
			job: &CrawlJob{
				ID:        "parent-3",
				ParentID:  "",
				Status:    JobStatusRunning,
				CreatedAt: now,
			},
			childStats: &jobtypes.JobChildStats{
				ChildCount:        44,
				CompletedChildren: 11,
				FailedChildren:    2,
				RunningChildren:   31,
			},
			expectedReport: &jobtypes.JobStatusReport{
				Status:            "running",
				ChildCount:        44,
				CompletedChildren: 11,
				FailedChildren:    2,
				RunningChildren:   31,
				ProgressText:      "31 running, 11 completed, 2 failed (Total: 44)",
				Errors:            []string{},
				Warnings:          []string{},
			},
		},
		{
			name: "parent job with children - all failed",
			job: &CrawlJob{
				ID:        "parent-4",
				ParentID:  "",
				Status:    JobStatusFailed,
				CreatedAt: now,
			},
			childStats: &jobtypes.JobChildStats{
				ChildCount:        5,
				CompletedChildren: 0,
				FailedChildren:    5,
			},
			expectedReport: &jobtypes.JobStatusReport{
				Status:            "failed",
				ChildCount:        5,
				CompletedChildren: 0,
				FailedChildren:    5,
				RunningChildren:   0,
				ProgressText:      "5 failed (Total: 5)",
				Errors:            []string{},
				Warnings:          []string{},
			},
		},
		{
			name: "child job with progress",
			job: &CrawlJob{
				ID:        "child-1",
				ParentID:  "parent-123",
				Status:    JobStatusRunning,
				CreatedAt: now,
				Progress: CrawlProgress{
					TotalURLs:     25,
					CompletedURLs: 15,
					FailedURLs:    3,
					PendingURLs:   7,
				},
			},
			childStats: nil,
			expectedReport: &jobtypes.JobStatusReport{
				Status:            "running",
				ChildCount:        0,
				CompletedChildren: 0,
				FailedChildren:    0,
				RunningChildren:   0,
				ProgressText:      "25 URLs (15 completed, 3 failed, 7 running)",
				Errors:            []string{},
				Warnings:          []string{},
			},
		},
		{
			name: "child job without progress",
			job: &CrawlJob{
				ID:        "child-2",
				ParentID:  "parent-123",
				Status:    JobStatusRunning,
				CreatedAt: now,
				Progress:  CrawlProgress{}, // Empty progress
			},
			childStats: nil,
			expectedReport: &jobtypes.JobStatusReport{
				Status:            "running",
				ChildCount:        0,
				CompletedChildren: 0,
				FailedChildren:    0,
				RunningChildren:   0,
				ProgressText:      "Status: running",
				Errors:            []string{},
				Warnings:          []string{},
			},
		},
		{
			name: "job with error",
			job: &CrawlJob{
				ID:        "job-with-error",
				ParentID:  "",
				Status:    JobStatusFailed,
				CreatedAt: now,
				Error:     "HTTP 404: Not Found",
			},
			childStats: nil,
			expectedReport: &jobtypes.JobStatusReport{
				Status:            "failed",
				ChildCount:        0,
				CompletedChildren: 0,
				FailedChildren:    0,
				RunningChildren:   0,
				ProgressText:      "No child jobs spawned yet",
				Errors:            []string{"HTTP 404: Not Found"},
				Warnings:          []string{},
			},
		},
		{
			name: "job without error",
			job: &CrawlJob{
				ID:        "job-no-error",
				ParentID:  "",
				Status:    JobStatusCompleted,
				CreatedAt: now,
				Error:     "",
			},
			childStats: nil,
			expectedReport: &jobtypes.JobStatusReport{
				Status:            "completed",
				ChildCount:        0,
				CompletedChildren: 0,
				FailedChildren:    0,
				RunningChildren:   0,
				ProgressText:      "No child jobs spawned yet",
				Errors:            []string{},
				Warnings:          []string{},
			},
		},
		{
			name: "edge case - negative running children calculation",
			job: &CrawlJob{
				ID:        "parent-edge",
				ParentID:  "",
				Status:    JobStatusRunning,
				CreatedAt: now,
			},
			childStats: &jobtypes.JobChildStats{
				ChildCount:        10,
				CompletedChildren: 7,
				FailedChildren:    5, // Completed + Failed > ChildCount (edge case)
			},
			expectedReport: &jobtypes.JobStatusReport{
				Status:            "running",
				ChildCount:        10,
				CompletedChildren: 7,
				FailedChildren:    5,
				RunningChildren:   0, // Should be clamped to 0, not negative
				ProgressText:      "7 completed, 5 failed (Total: 10)",
				Errors:            []string{},
				Warnings:          []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := tt.job.GetStatusReport(tt.childStats)

			// Assert Status
			if report.Status != tt.expectedReport.Status {
				t.Errorf("Status: got %v, want %v", report.Status, tt.expectedReport.Status)
			}

			// Assert ChildCount
			if report.ChildCount != tt.expectedReport.ChildCount {
				t.Errorf("ChildCount: got %v, want %v", report.ChildCount, tt.expectedReport.ChildCount)
			}

			// Assert CompletedChildren
			if report.CompletedChildren != tt.expectedReport.CompletedChildren {
				t.Errorf("CompletedChildren: got %v, want %v", report.CompletedChildren, tt.expectedReport.CompletedChildren)
			}

			// Assert FailedChildren
			if report.FailedChildren != tt.expectedReport.FailedChildren {
				t.Errorf("FailedChildren: got %v, want %v", report.FailedChildren, tt.expectedReport.FailedChildren)
			}

			// Assert RunningChildren
			if report.RunningChildren != tt.expectedReport.RunningChildren {
				t.Errorf("RunningChildren: got %v, want %v", report.RunningChildren, tt.expectedReport.RunningChildren)
			}

			// Assert ProgressText
			if report.ProgressText != tt.expectedReport.ProgressText {
				t.Errorf("ProgressText: got %v, want %v", report.ProgressText, tt.expectedReport.ProgressText)
			}

			// Assert Errors
			if len(report.Errors) != len(tt.expectedReport.Errors) {
				t.Errorf("Errors length: got %v, want %v", len(report.Errors), len(tt.expectedReport.Errors))
			} else {
				for i := range report.Errors {
					if report.Errors[i] != tt.expectedReport.Errors[i] {
						t.Errorf("Errors[%d]: got %v, want %v", i, report.Errors[i], tt.expectedReport.Errors[i])
					}
				}
			}

			// Assert Warnings
			if len(report.Warnings) != len(tt.expectedReport.Warnings) {
				t.Errorf("Warnings length: got %v, want %v", len(report.Warnings), len(tt.expectedReport.Warnings))
			} else {
				for i := range report.Warnings {
					if report.Warnings[i] != tt.expectedReport.Warnings[i] {
						t.Errorf("Warnings[%d]: got %v, want %v", i, report.Warnings[i], tt.expectedReport.Warnings[i])
					}
				}
			}
		})
	}
}
