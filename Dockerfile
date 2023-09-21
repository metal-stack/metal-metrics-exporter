
FROM golang:1.21 as builder
COPY / /work
WORKDIR /work
RUN make metal-metrics-exporter

FROM scratch
COPY --from=builder /work/bin/metal-metrics-exporter /metal-metrics-exporter
USER 999
ENTRYPOINT ["/metal-metrics-exporter"]

EXPOSE 9080
