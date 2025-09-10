FROM gcr.io/distroless/base-debian12:latest

ARG TARGETPLATFORM

COPY $TARGETPLATFORM/opencti_exporter /bin/opencti_exporter

ENTRYPOINT [ "/bin/opencti_exporter" ]
