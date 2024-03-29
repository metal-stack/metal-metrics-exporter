package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	metalgo "github.com/metal-stack/metal-go"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	klog "k8s.io/klog/v2"
)

func main() {

	url := os.Getenv("METAL_API_URL")
	hmac := os.Getenv("METAL_API_HMAC")

	client, err := metalgo.NewDriver(url, "", hmac)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	metalCollector := newMetalCollector(client)
	prometheus.MustRegister(metalCollector)

	http.Handle("/metrics", promhttp.Handler())
	klog.Info("Beginning to serve on port :9080")
	server := &http.Server{
		Addr:              ":9080",
		ReadHeaderTimeout: 1 * time.Minute,
	}
	klog.Fatal(server.ListenAndServe())
}
