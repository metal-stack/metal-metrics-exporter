GO111MODULE := on
DOCKER_TAG := $(or ${GIT_TAG_NAME}, latest)

.PHONY: all
all: test metal-metrics-exporter

.PHONY: metal-metrics-exporter
metal-metrics-exporter:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/metal-metrics-exporter *.go
	strip bin/metal-metrics-exporter

.PHONY: test
test:
	go test -cover ./...

.PHONY: clean
clean:
	rm -f bin/*
