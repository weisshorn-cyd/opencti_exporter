package collector

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weisshorn-cyd/gocti"
	"github.com/weisshorn-cyd/gocti/list"
)

const (
	namespace        = "opencti"
	customProperties = "id entity_type observable_value created_at updated_at"
)

// Verify if the OpenCTICollector implements prometheus.Collector.
var _ prometheus.Collector = (*OpenCTICollector)(nil)

//nolint:containedctx // Needed to share the context.
type OpenCTICollector struct {
	ctx                  context.Context
	opencti              *gocti.OpenCTIAPIClient
	up                   *prometheus.Desc
	lastCreatedTimestamp *prometheus.Desc
	lastUpdatedTimestamp *prometheus.Desc
	logger               *slog.Logger
}

func NewOpenCTICollector(
	ctx context.Context,
	opencti *gocti.OpenCTIAPIClient,
	subsystem string,
	logger *slog.Logger,
) *OpenCTICollector {
	return &OpenCTICollector{
		ctx:     ctx,
		opencti: opencti,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "up"),
			"Wether OpenCTI is up.", nil, nil,
		),
		lastCreatedTimestamp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "last_created_timestamp_seconds"),
			"Timestamp of the last creation in OpenCTI by entity type.", []string{"entity_type"}, nil,
		),
		lastUpdatedTimestamp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "last_updated_timestamp_seconds"),
			"Timestamp of the last update in OpenCTI by entity type.", []string{"entity_type"}, nil,
		),
		logger: logger,
	}
}

// Collect implements prometheus.Collector.
func (c *OpenCTICollector) Collect(ch chan<- prometheus.Metric) {
	up := c.scrape(ch)
	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, up)
}

// Describe implements Prometheus.Collector.
func (c *OpenCTICollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up

	ch <- c.lastCreatedTimestamp

	ch <- c.lastUpdatedTimestamp
}

// scrape collects metrics from OpenCTI and return an up metric value.
func (c *OpenCTICollector) scrape(ch chan<- prometheus.Metric) float64 {
	if err := c.opencti.HealthCheck(c.ctx); err != nil {
		c.logger.ErrorContext(c.ctx, "Health check failed", "error", err)

		return 0.0
	}

	c.logger.DebugContext(c.ctx, "Health check successful")
	// Retrieve last created observable.
	observablesCreated, err := c.opencti.ListStixCyberObservables(c.ctx, customProperties, false, nil,
		list.WithFirst(1),
		list.WithOrderBy("created_at"),
		list.WithOrderMode(list.OrderModeDesc),
	)
	if err != nil {
		c.logger.ErrorContext(c.ctx, "Retrieving last created StixCyberObservables", "error", err)

		return 0.0
	}

	if len(observablesCreated) == 0 {
		c.logger.ErrorContext(c.ctx, "No last created StixCyberObservable retrieved")

		return 0.0
	}

	c.logger.DebugContext(c.ctx, "Last StixCyberObservable created", "object", fmt.Sprintf("%+v", observablesCreated[0]))

	// Retrieve last updated observable.
	observablesUpdated, err := c.opencti.ListStixCyberObservables(c.ctx, customProperties, false, nil,
		list.WithFirst(1),
		list.WithOrderBy("updated_at"),
		list.WithOrderMode(list.OrderModeDesc),
	)
	if err != nil {
		c.logger.ErrorContext(c.ctx, "Retrieving last updated StixCyberObservables", "error", err)

		return 0.0
	}

	if len(observablesUpdated) == 0 {
		c.logger.ErrorContext(c.ctx, "No last updated StixCyberObservable retrieved")

		return 0.0
	}

	c.logger.DebugContext(c.ctx, "Last StixCyberObservable updated", "object", fmt.Sprintf("%+v", observablesUpdated[0]))

	ch <- prometheus.MustNewConstMetric(
		c.lastCreatedTimestamp,
		prometheus.GaugeValue,
		float64(observablesCreated[0].UpdatedAt.Unix()),
		observablesCreated[0].EntityType,
	)

	ch <- prometheus.MustNewConstMetric(
		c.lastUpdatedTimestamp,
		prometheus.GaugeValue,
		float64(observablesUpdated[0].UpdatedAt.Unix()),
		observablesUpdated[0].EntityType,
	)

	return 1.0
}
