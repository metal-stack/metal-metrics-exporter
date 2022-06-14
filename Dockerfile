
FROM golang:1.18-alpine as builder
RUN apk add make binutils
COPY / /work
WORKDIR /work
RUN make metal-metrics-exporter

FROM alpine:3.16
COPY --from=builder /work/bin/metal-metrics-exporter /metal-metrics-exporter
USER root
ENTRYPOINT ["/metal-metrics-exporter"]

EXPOSE 9080
