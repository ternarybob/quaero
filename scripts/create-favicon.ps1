# PowerShell script to create favicon.ico file for Quaero
# Creates a teal background with white Q (for Quaero)

# Create a simple 16x16 bitmap in ICO format
$icoBytes = @(
    # ICO Header (6 bytes)
    0x00, 0x00,  # Reserved (must be 0)
    0x01, 0x00,  # Type (1 = ICO)
    0x01, 0x00,  # Number of images (1)

    # Image Directory Entry (16 bytes)
    0x10,        # Width (16 pixels)
    0x10,        # Height (16 pixels)
    0x00,        # Color palette (0 = no palette)
    0x00,        # Reserved (must be 0)
    0x01, 0x00,  # Color planes (1)
    0x20, 0x00,  # Bits per pixel (32 = RGBA)
    0x00, 0x03, 0x00, 0x00,  # Size of image data (768 bytes)
    0x16, 0x00, 0x00, 0x00,  # Offset to image data (22 bytes)

    # Bitmap Info Header (40 bytes)
    0x28, 0x00, 0x00, 0x00,  # Header size (40)
    0x10, 0x00, 0x00, 0x00,  # Width (16)
    0x20, 0x00, 0x00, 0x00,  # Height (32 = 16*2 for ICO format)
    0x01, 0x00,              # Planes (1)
    0x20, 0x00,              # Bits per pixel (32)
    0x00, 0x00, 0x00, 0x00,  # Compression (0 = none)
    0x00, 0x03, 0x00, 0x00,  # Image size (768)
    0x00, 0x00, 0x00, 0x00,  # X pixels per meter (0)
    0x00, 0x00, 0x00, 0x00,  # Y pixels per meter (0)
    0x00, 0x00, 0x00, 0x00,  # Colors used (0)
    0x00, 0x00, 0x00, 0x00   # Important colors (0)
)

# Teal background with white Q pattern (16x16 pixels, BGRA format)
$pixelData = @()

# Define colors (BGRA: Blue, Green, Red, Alpha)
$teal = @(0x99, 0x99, 0x33, 0xFF)    # #339999 (Quaero teal)
$white = @(0xFF, 0xFF, 0xFF, 0xFF)   # #FFFFFF

# Create 16x16 bitmap (stored bottom-up for BMP format)
for ($row = 15; $row -ge 0; $row--) {
    for ($col = 0; $col -lt 16; $col++) {
        # Q pattern (circle with tail)
        $isQ = $false

        # Center point
        $cx = 8
        $cy = 8
        $dx = $col - $cx
        $dy = $row - $cy
        $dist = [Math]::Sqrt($dx * $dx + $dy * $dy)

        # Outer circle (radius ~5)
        if ($dist -ge 3.5 -and $dist -le 5.5) {
            $isQ = $true
        }

        # Inner hollow (radius ~3)
        if ($dist -lt 2.5) {
            $isQ = $false
        }

        # Q tail (diagonal bottom-right)
        if ($col -ge 10 -and $col -le 11 -and $row -ge 3 -and $row -le 5) {
            $isQ = $true
        }
        if ($col -ge 11 -and $col -le 12 -and $row -ge 2 -and $row -le 4) {
            $isQ = $true
        }

        if ($isQ) {
            $pixelData += $white
        } else {
            $pixelData += $teal
        }
    }
}

# Add AND mask (16x16 bits = 32 bytes, all transparent)
$andMask = @(0x00) * 32

# Combine all data
$allBytes = $icoBytes + $pixelData + $andMask

# Write to pages/static directory
$outputPath = "pages\static\favicon.ico"
if (-not (Test-Path "pages\static")) {
    New-Item -Path "pages\static" -ItemType Directory -Force | Out-Null
}
[System.IO.File]::WriteAllBytes($outputPath, $allBytes)

Write-Host "Favicon created at: $outputPath" -ForegroundColor Green
Write-Host "Size: $($allBytes.Length) bytes" -ForegroundColor Cyan
Write-Host "Pattern: Teal background with white 'Q' for Quaero" -ForegroundColor Gray
