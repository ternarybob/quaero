package common

import (
	"github.com/ternarybob/banner"
)

// PrintBanner displays the application banner
func PrintBanner(version string) {
	banner.Print("Quaero", version)
}
