// -----------------------------------------------------------------------
// Crash Protection - Fatal error handling and crash file generation
// -----------------------------------------------------------------------

package common

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// CrashLogDir is the directory where crash files will be written
// Set during application initialization
var CrashLogDir = "./logs"

// InstallCrashHandler sets up process-level crash protection.
// This should be called at the very start of main() with a deferred recovery.
func InstallCrashHandler(logDir string) {
	if logDir != "" {
		CrashLogDir = logDir
	}

	// Ensure log directory exists
	if err := os.MkdirAll(CrashLogDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "CRASH: Failed to create log directory: %v\n", err)
	}
}

// WriteCrashFile writes a comprehensive crash report to a file.
// This should be called from panic recovery handlers before the process exits.
// Returns the path to the crash file.
func WriteCrashFile(panicVal interface{}, stackTrace string) string {
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf("crash-%s.log", timestamp)
	crashPath := filepath.Join(CrashLogDir, filename)

	// Build crash report
	var report bytes.Buffer

	report.WriteString("=== QUAERO CRASH REPORT ===\n")
	report.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Version: %s\n", GetFullVersion()))
	report.WriteString("\n")

	// Panic value
	report.WriteString("=== PANIC VALUE ===\n")
	report.WriteString(fmt.Sprintf("%v\n", panicVal))
	report.WriteString("\n")

	// Stack trace of the panicking goroutine
	report.WriteString("=== STACK TRACE ===\n")
	report.WriteString(stackTrace)
	report.WriteString("\n")

	// All goroutines stack dump
	report.WriteString("=== ALL GOROUTINES ===\n")
	report.WriteString(GetAllGoroutineStacks())
	report.WriteString("\n")

	// System info
	report.WriteString("=== SYSTEM INFO ===\n")
	report.WriteString(fmt.Sprintf("NumGoroutine: %d\n", runtime.NumGoroutine()))
	report.WriteString(fmt.Sprintf("NumCPU: %d\n", runtime.NumCPU()))
	report.WriteString(fmt.Sprintf("GOOS: %s\n", runtime.GOOS))
	report.WriteString(fmt.Sprintf("GOARCH: %s\n", runtime.GOARCH))

	// Memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	report.WriteString(fmt.Sprintf("Alloc: %d MB\n", memStats.Alloc/1024/1024))
	report.WriteString(fmt.Sprintf("TotalAlloc: %d MB\n", memStats.TotalAlloc/1024/1024))
	report.WriteString(fmt.Sprintf("Sys: %d MB\n", memStats.Sys/1024/1024))
	report.WriteString(fmt.Sprintf("NumGC: %d\n", memStats.NumGC))
	report.WriteString("\n")

	report.WriteString("=== END CRASH REPORT ===\n")

	// Write directly to file using low-level operations
	// This is more reliable than buffered IO in crash scenarios
	file, err := os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		// Last resort: write to stderr
		fmt.Fprintf(os.Stderr, "CRASH: Failed to create crash file: %v\n", err)
		fmt.Fprintf(os.Stderr, "%s", report.String())
		return ""
	}

	_, err = file.Write(report.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "CRASH: Failed to write crash file: %v\n", err)
		fmt.Fprintf(os.Stderr, "%s", report.String())
	}

	// Sync and close
	file.Sync()
	file.Close()

	// Also write to stderr for immediate visibility
	fmt.Fprintf(os.Stderr, "\n!!! FATAL CRASH - Report saved to: %s !!!\n", crashPath)
	fmt.Fprintf(os.Stderr, "Panic: %v\n", panicVal)

	return crashPath
}

// GetAllGoroutineStacks returns stack traces for all goroutines.
// Uses a large buffer to capture all stacks.
func GetAllGoroutineStacks() string {
	// Start with a reasonable buffer size, grow if needed
	buf := make([]byte, 64*1024)
	for {
		n := runtime.Stack(buf, true) // true = all goroutines
		if n < len(buf) {
			return string(buf[:n])
		}
		// Buffer too small, double it
		buf = make([]byte, len(buf)*2)
		if len(buf) > 64*1024*1024 { // Max 64MB
			return string(buf[:runtime.Stack(buf, true)])
		}
	}
}

// GetStackTrace returns the current goroutine's stack trace.
func GetStackTrace() string {
	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false) // false = current goroutine only
	return string(buf[:n])
}

// RecoverWithCrashFile is a helper for deferred panic recovery that writes a crash file.
// Usage: defer common.RecoverWithCrashFile()
func RecoverWithCrashFile() {
	if r := recover(); r != nil {
		stackTrace := GetStackTrace()
		WriteCrashFile(r, stackTrace)
		os.Exit(1)
	}
}
