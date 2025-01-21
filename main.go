package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	metalgo "github.com/metal-stack/metal-go"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	client metalgo.Client
)

func main() {
	envOrDefault := func(key, fallback string) string {
		if val := os.Getenv(key); val != "" {
			return val
		}
		return fallback
	}

	var (
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

		url              = os.Getenv("METAL_API_URL")
		hmac             = os.Getenv("METAL_API_HMAC")
		fetchIntervalEnv = envOrDefault("FETCH_INTERVAL", "90s") // time to sleep after every metrics fetch
		updateTimeoutEnv = envOrDefault("UPDATE_TIMEOUT", "60s") // maximum time for metal-api to respond to all our requests until context gets cancelled

		err error
	)

	client, err = metalgo.NewDriver(url, "", hmac)
	if err != nil {
		log.Error("error creating client", "error", err)
		os.Exit(1)
	}

	fetchInterval, err := time.ParseDuration(fetchIntervalEnv)
	if err != nil {
		log.Error("error parsing fetch interval", "error", err)
		os.Exit(1)
	}

	updateTimeout, err := time.ParseDuration(updateTimeoutEnv)
	if err != nil {
		log.Error("error parsing update timeout", "error", err)
		os.Exit(1)
	}

	go func() {
		var (
			initialUpdateSuccess = false
			failCount            = 0
		)

		for {
			log.Info("updating metrics...")
			start := time.Now()

			err = update(updateTimeout)
			if err != nil {
				if !initialUpdateSuccess {
					log.Error("error during initial update", "error", err)
					os.Exit(1)
				}

				log.Error("error during update", "error", err, "took", time.Since(start).String(), "fail-count", failCount)
				failCount++

				if failCount >= 3 {
					log.Error("failed three time or more, dying...")
					os.Exit(1)
				}
			} else {
				initialUpdateSuccess = true
				failCount = 0
				log.Info("metrics updated successfully", "took", time.Since(start).String())
			}

			log.Info("next fetch in " + fetchInterval.String())
			time.Sleep(fetchInterval)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())

	log.Info("beginning to serve on port :9080")

	server := &http.Server{
		Addr:              ":9080",
		ReadHeaderTimeout: 1 * time.Minute,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Error("error serving", "error", err)
	}
}
