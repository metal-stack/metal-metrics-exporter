
FROM golang:1.18 as builder
COPY / /work
WORKDIR /work
RUN make metal-metrics-exporter

FROM gcr.io/distroless/static-debian11:nonroot
COPY --from=builder /work/bin/metal-metrics-exporter /metal-metrics-exporter
USER 999
ENTRYPOINT ["/metal-metrics-exporter"]

EXPOSE 9080
