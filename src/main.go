package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
)

var version = "dev"

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("docker client error: %v", err)
	}

	unifiClient := NewUnifiClient(cfg.UnifiURL, cfg.APIToken, cfg.InsecureTLS)
	siteID, err := unifiClient.getSiteIDByName(ctx, cfg.SiteName)
	if err != nil {
		log.Fatalf("site lookup error: %v", err)
	}

	log.Printf("unifi-external-dns %s started...", version)

	runSync := func() {
		dockerRecords, err := loadDockerRecords(ctx, dockerClient, cfg.DefaultTTL)
		if err != nil {
			log.Printf("docker records error: %v", err)
			return
		}

		yamlRecords, err := loadYAMLRecords(cfg.DNSYAMLPath, cfg.DefaultTTL)
		if err != nil {
			log.Printf("yaml records error: %v", err)
			return
		}

		if err := syncOnce(ctx, cfg, dockerRecords, yamlRecords, unifiClient, siteID); err != nil {
			log.Printf("sync error: %v", err)
		}
	}

	runSync()

	ticker := time.NewTicker(cfg.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("shutdown")
			return
		case <-ticker.C:
			runSync()
		}
	}
}
