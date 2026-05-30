package main

import (
	"context"
	"log"
	"os"
	"time"

	client "kiro_waf/internal/client"
	"kiro_waf/internal/client/xdp"
)

func init() {
	// Register XDP startup hook to break import cycle between client and xdp packages.
	client.XDPStartupFunc = startXDPComponents
}

func main() {
	os.Exit(client.Run())
}

// startXDPComponents initializes and starts XDP-related components:
// GeoIP loader with periodic refresh and botnet controller monitoring.
func startXDPComponents(ctx context.Context, geoipCSVPath string) {
	// Start GeoIP loader
	geoipLoader := xdp.NewGeoIPLoader()
	if geoipCSVPath != "" {
		if err := geoipLoader.LoadFromCSV(geoipCSVPath); err != nil {
			log.Printf("WARNING: GeoIP CSV load failed: %v", err)
		}
	}
	if err := geoipLoader.LoadBlockedCountriesFromEnv(); err != nil {
		log.Printf("WARNING: GeoIP blocked countries load failed: %v", err)
	}
	geoipLoader.StartPeriodicRefresh(ctx, 24*time.Hour)

	// Start botnet controller
	botnetCtrl := xdp.NewBotnetController(xdp.BotnetControllerConfig{
		Threshold:       5000,
		CooldownSeconds: 30,
		PollInterval:    1 * time.Second,
	})
	botnetCtrl.Start(ctx)
}
