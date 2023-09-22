FROM scratch
COPY bin/metal-metrics-exporter /metal-metrics-exporter
USER 999
ENTRYPOINT ["/metal-metrics-exporter"]

EXPOSE 9080
