GO111MODULE := on
DOCKER_TAG := $(or ${GIT_TAG_NAME}, latest)

all: metal-metrics-exporter

.PHONY: metal-metrics-exporter
metal-metrics-exporter:
	go build -o bin/metal-metrics-exporter *.go
	strip bin/metal-metrics-exporter

.PHONY: clean
clean:
	rm -f bin/*
