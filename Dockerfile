FROM gcr.io/distroless/base-debian12:latest

COPY opencti_exporter /bin/opencti_exporter

ENTRYPOINT [ "/bin/opencti_exporter" ]
