package githublogs

import (
	"strings"
	"testing"
)

func TestParseLogURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantJobID int64
		wantErr   bool
	}{
		{
			name:      "Valid URL",
			url:       "https://github.com/ternarybob/quaero/actions/runs/19523113218/job/55890316589",
			wantOwner: "ternarybob",
			wantRepo:  "quaero",
			wantJobID: 55890316589,
			wantErr:   false,
		},
		{
			name:    "Invalid URL format",
			url:     "https://github.com/ternarybob/quaero/issues/1",
			wantErr: true,
		},
		{
			name:    "Invalid Job ID",
			url:     "https://github.com/ternarybob/quaero/actions/runs/123/job/abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, jobID, err := ParseLogURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if owner != tt.wantOwner {
					t.Errorf("ParseLogURL() owner = %v, want %v", owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("ParseLogURL() repo = %v, want %v", repo, tt.wantRepo)
				}
				if jobID != tt.wantJobID {
					t.Errorf("ParseLogURL() jobID = %v, want %v", jobID, tt.wantJobID)
				}
			}
		})
	}
}

func TestSanitizeLogForAI(t *testing.T) {
	rawLog := `2023-10-27T10:00:00.1234567Z Step 1: Setting up
2023-10-27T10:00:01.1234567Z Step 2: Installing dependencies
2023-10-27T10:00:02.1234567Z Step 3: Running tests
2023-10-27T10:00:03.1234567Z Error: Test failed
2023-10-27T10:00:04.1234567Z Stack trace line 1
2023-10-27T10:00:05.1234567Z Stack trace line 2
2023-10-27T10:00:06.1234567Z Step 4: Cleanup
` + strings.Repeat("2023-10-27T10:00:07.0000000Z Normal log line\n", 100) + `
2023-10-27T10:00:08.1234567Z Final summary line 1
2023-10-27T10:00:09.1234567Z Final summary line 2`

	sanitized := SanitizeLogForAI(rawLog)

	// Checks:
	// 1. Timestamps removed
	if strings.Contains(sanitized, "2023-10-27T") {
		t.Errorf("SanitizeLogForAI() did not remove timestamps")
	}

	// 2. Error context preserved
	if !strings.Contains(sanitized, "Error: Test failed") {
		t.Errorf("SanitizeLogForAI() did not preserve error line")
	}
	if !strings.Contains(sanitized, "Stack trace line 1") {
		t.Errorf("SanitizeLogForAI() did not preserve context after error")
	}
	if !strings.Contains(sanitized, "Step 2: Installing dependencies") {
		t.Errorf("SanitizeLogForAI() did not preserve context before error")
	}

	// 3. Tail preserved
	if !strings.Contains(sanitized, "Final summary line 2") {
		t.Errorf("SanitizeLogForAI() did not preserve tail")
	}

	// 4. Middle skipped (we added 100 lines, so some should be skipped)
	if !strings.Contains(sanitized, "...") {
		t.Errorf("SanitizeLogForAI() did not skip middle lines")
	}
}
