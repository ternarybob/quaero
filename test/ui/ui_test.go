package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var screenshotCounter int

func takeScreenshot(ctx context.Context, t *testing.T, name string) {
	screenshotCounter++
	runDir := os.Getenv("TEST_RUN_DIR")
	if runDir == "" {
		runDir = filepath.Join("..", "results")
	}

	filename := fmt.Sprintf("%02d_%s.png", screenshotCounter, name)
	screenshotPath := filepath.Join(runDir, filename)

	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err == nil {
		os.MkdirAll(filepath.Dir(screenshotPath), 0755)
		if err := os.WriteFile(screenshotPath, buf, 0644); err == nil {
			t.Logf("📸 Screenshot %d: %s", screenshotCounter, filename)
		}
	}
}

func startVideoRecording(ctx context.Context, t *testing.T) (func(), error) {
	runDir := os.Getenv("TEST_RUN_DIR")
	if runDir == "" {
		runDir = filepath.Join("..", "results")
	}

	videoPath := filepath.Join(runDir, "test_recording.webm")
	os.MkdirAll(filepath.Dir(videoPath), 0755)

	frameCount := 0
	maxFrames := 300 // 30 seconds at 10fps

	// Start screencast
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return page.StartScreencast().
				WithFormat("png").
				WithQuality(80).
				WithEveryNthFrame(6). // ~10fps at 60fps base
				Do(ctx)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start screencast: %w", err)
	}

	t.Log("🎥 Video recording started")

	// Cleanup function
	stopRecording := func() {
		chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				return page.StopScreencast().Do(ctx)
			}),
		)
		t.Logf("🎥 Video recording stopped (%d frames captured)", frameCount)
	}

	// Listen for screencast frames
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if frameCount >= maxFrames {
			return
		}

		if _, ok := ev.(*page.EventScreencastFrame); ok {
			frameCount++
		}
	})

	return stopRecording, nil
}
