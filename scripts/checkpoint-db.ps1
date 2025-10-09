# Force SQLite WAL checkpoint to commit data to main database file
$dbPath = "C:\development\quaero\bin\data\quaero.db"

Write-Host "Checkpointing database: $dbPath" -ForegroundColor Yellow

# Create a simple Go program to checkpoint the database
$goCode = @"
package main

import (
    "database/sql"
    "fmt"
    "log"
    _ "modernc.org/sqlite"
)

func main() {
    db, err := sql.Open("sqlite", "$($dbPath -replace '\\', '\\')")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Force WAL checkpoint
    _, err = db.Exec("PRAGMA wal_checkpoint(FULL)")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("WAL checkpoint completed successfully")
}
"@

$tempDir = [System.IO.Path]::GetTempPath()
$tempFile = Join-Path $tempDir "checkpoint.go"
$goCode | Set-Content -Path $tempFile

# Run the checkpoint
cd C:\development\quaero
go run $tempFile

Remove-Item $tempFile
Write-Host "Database checkpointed" -ForegroundColor Green
