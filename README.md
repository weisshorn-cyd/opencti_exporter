# OpenCTI Exporter

A Prometheus exporter for OpenCTI.

This exporter uses [gocti](https://github.com/weisshorn-cyd/gocti) to retrieve some metrics from [OpenCTI](https://github.com/OpenCTI-Platform/opencti).

## Metrics

The exporter check whether the OpenCTI instance is reachable, and retrieves the timestamps of the last created and updated observables. The goal is to identify a possible ingestion issue if there has not been a creation or update of observables for some time. If any issue occurs fetching the last created or updated observable, the metric `opencti_up` will be 0 even if OpenCTI health check is successful.

```
# HELP opencti_last_created_timestamp_seconds Timestamp of the last creation in OpenCTI by entity type.
# TYPE opencti_last_created_timestamp_seconds gauge
opencti_last_created_timestamp_seconds{entity_type="Hostname"} 1.73693012e+09
# HELP opencti_last_updated_timestamp_seconds Timestamp of the last update in OpenCTI by entity type.
# TYPE opencti_last_updated_timestamp_seconds gauge
opencti_last_updated_timestamp_seconds{entity_type="StixFile"} 1.736930175e+09
# HELP opencti_up Wether OpenCTI is up.
# TYPE opencti_up gauge
opencti_up 1
```

## Configuration

| NAME             | VARIABLE          | TYPE       | DEFAULT             | DESCRIPTION                              |
|------------------|-------------------|------------|---------------------|------------------------------------------|
| Port             | PORT              | string     | 10031               | Port to run the HTTP server on           |
| LogLevel         | LOG_LEVEL         | slog.Level | info                | Which log level to log at                |
| OpenctiURL       | OPENCTI_URL       | string     | http://opencti:8080 | OpenCTI URL to connect to                |
| OpenctiToken     | OPENCTI_TOKEN     | string     |                     | OpenCTI token to use                     |
| MetricsSubsystem | METRICS_SUBSYSTEM | string     |                     | The Prometheus subsystem for the metrics |
| MetricsPath      | METRICS_PATH      | string     | /metrics            | The path to access the metrics           |
