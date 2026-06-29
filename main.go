package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	apiv2client "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/metal-metrics-exporter/pkg/collector"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
		token            = os.Getenv("METAL_API_TOKEN")
		fetchIntervalEnv = envOrDefault("FETCH_INTERVAL", "90s") // time to sleep after every metrics fetch
		updateTimeoutEnv = envOrDefault("UPDATE_TIMEOUT", "60s") // maximum time for metal-api to respond to all our requests until context gets cancelled

		err error
	)
	client, err := apiv2client.New(&apiv2client.DialConfig{
		BaseURL:      url,
		Token:        token,
		TokenRenewal: &apiv2client.TokenRenewal{
			// FIXME
		},
	})
	if err != nil {
		log.Error("error creating client", "error", err)
		os.Exit(1)
	}
	client.Ping(context.Background(), &apiv2client.PingConfig{
		ComponentType: apiv2.ComponentType_COMPONENT_TYPE_METAL_METRICS_EXPORTER,
	})

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

	c := collector.New(client, updateTimeout)

	prometheus.MustRegister(c)

	// initialize metrics before starting to serve them to prevent empty values
	log.Info("initializing metrics...")

	err = c.Update()
	if err != nil {
		log.Error("error during initial update", "error", err)
		os.Exit(1)
	}

	go func() {
		var (
			failCount = 0
		)

		for {
			log.Info("next fetch in " + fetchInterval.String())
			time.Sleep(fetchInterval)

			log.Info("updating metrics...")
			start := time.Now()

			err = c.Update()
			if err != nil {
				log.Error("error during update", "error", err, "took", time.Since(start).String(), "fail-count", failCount)
				failCount++

				if failCount >= 3 {
					log.Error("failed three time or more, dying...")
					os.Exit(1)
				}
			} else {
				failCount = 0
				log.Info("metrics updated successfully", "took", time.Since(start).String())
			}
		}
	}()

	http.Handle("/metrics", &handleWithLog{
		log:     log,
		handler: promhttp.Handler(),
	})

	log.Info("beginning to serve on port :9080")

	server := &http.Server{
		Addr:              ":9080",
		ReadHeaderTimeout: 1 * time.Minute,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Error("error serving", "error", err)
	}
}

type handleWithLog struct {
	log     *slog.Logger
	handler http.Handler
}

func (h *handleWithLog) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	h.log.Info("serving metrics request", "method", rq.Method, "url", rq.URL.String(), "user-agent", rq.UserAgent(), "remote-addr", rq.RemoteAddr)
	h.handler.ServeHTTP(rw, rq)
}
