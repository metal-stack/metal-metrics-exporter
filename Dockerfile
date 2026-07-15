FROM scratch
COPY bin/metal-metrics-exporter /metal-metrics-exporter
COPY /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 999
ENTRYPOINT ["/metal-metrics-exporter"]

EXPOSE 9080
