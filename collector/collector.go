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
	customProperties = "id entity_type observable_value created_at updated_at creators { id name }"
	userProperties   = "id name"
)

type creator struct {
	id   string
	name string
}

// Verify if the OpenCTICollector implements prometheus.Collector.
var _ prometheus.Collector = (*OpenCTICollector)(nil)

//nolint:containedctx // Needed to share the context.
type OpenCTICollector struct {
	ctx                  context.Context
	opencti              *gocti.OpenCTIAPIClient
	entityTypes          []string
	creators             []creator
	up                   *prometheus.Desc
	lastCreatedTimestamp *prometheus.Desc
	lastUpdatedTimestamp *prometheus.Desc
	logger               *slog.Logger
}

func NewOpenCTICollector(
	ctx context.Context,
	opencti *gocti.OpenCTIAPIClient,
	subsystem string,
	entityTypes []string,
	creatorNames []string,
	logger *slog.Logger,
) (*OpenCTICollector, error) {
	var creators []creator

	if len(creatorNames) > 0 {
		resolved, err := resolveCreators(ctx, opencti, creatorNames, logger)
		if err != nil {
			return nil, fmt.Errorf("resolving creators: %w", err)
		}

		creators = resolved

		logger.DebugContext(ctx, "Creators resolved", "count", len(creators))
	}

	// Add empty entity type and creator to always collect global last created / updated timestamp.
	entityTypes = append([]string{""}, entityTypes...)
	creators = append([]creator{{id: "", name: ""}}, creators...)

	logger.InfoContext(ctx, "Collector initialized", "entitiy_types", entityTypes, "creators", creators)

	return &OpenCTICollector{
		ctx:         ctx,
		opencti:     opencti,
		entityTypes: entityTypes,
		creators:    creators,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "up"),
			"Wether OpenCTI is up.", nil, nil,
		),
		lastCreatedTimestamp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "last_created_timestamp_seconds"),
			"Timestamp of the last creation in OpenCTI by entity type and creator.", []string{"entity_type", "creator"}, nil,
		),
		lastUpdatedTimestamp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "last_updated_timestamp_seconds"),
			"Timestamp of the last update in OpenCTI by entity type and creator.", []string{"entity_type", "creator"}, nil,
		),
		logger: logger,
	}, nil
}

// resolveCreators queries OpenCTI users to match them with the creators.
func resolveCreators(
	ctx context.Context,
	opencti *gocti.OpenCTIAPIClient,
	creatorNames []string,
	logger *slog.Logger,
) ([]creator, error) {
	users, err := opencti.ListUsers(ctx, userProperties, true, nil)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	logger.DebugContext(ctx, "Retrieved users from OpenCTI", "count", len(users))

	nameSet := make(map[string]struct{}, len(creatorNames))

	for _, name := range creatorNames {
		nameSet[name] = struct{}{}
	}

	result := make([]creator, 0, len(creatorNames))

	for _, user := range users {
		if _, ok := nameSet[user.Name]; ok {
			result = append(result, creator{id: user.ID, name: user.Name})
		}
	}

	return result, nil
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

	for _, entityType := range c.entityTypes {
		for _, cr := range c.creators {
			c.collectTimestamp(ch, entityType, cr.id, cr.name, "created_at", c.lastCreatedTimestamp)
			c.collectTimestamp(ch, entityType, cr.id, cr.name, "updated_at", c.lastUpdatedTimestamp)
		}
	}

	return 1.0
}

func (c *OpenCTICollector) buildListOptions(entityType, creatorID, creatorName string) []list.Option {
	opts := []list.Option{
		list.WithFirst(1),
	}

	if entityType != "" {
		c.logger.DebugContext(c.ctx, "Filtering by entity type", "type", entityType)
		opts = append(opts, list.WithTypes([]string{entityType}))
	}

	if creatorID+creatorName != "" {
		c.logger.DebugContext(c.ctx, "Filtering by creator", "id", creatorID, "name", creatorName)

		creatorFilter := list.FilterGroup{
			Mode: list.FilterModeAnd,
			Filters: []list.Filter{
				{
					Key:      []string{"creator_id"},
					Values:   []any{creatorID},
					Operator: list.FilterOperatorEq,
					Mode:     list.FilterModeAnd,
				},
			},
		}

		opts = append(opts, list.WithFilters(creatorFilter))
	}

	return opts
}

func (c *OpenCTICollector) collectTimestamp(
	ch chan<- prometheus.Metric,
	entityType string,
	creatorID, creatorName string,
	orderBy string,
	desc *prometheus.Desc,
) {
	opts := c.buildListOptions(entityType, creatorID, creatorName)
	opts = append(opts, list.WithOrderBy(orderBy), list.WithOrderMode(list.OrderModeDesc))

	observables, err := c.opencti.ListStixCyberObservables(
		c.ctx, customProperties, false, nil, opts...,
	)
	if err != nil {
		c.logger.ErrorContext(c.ctx, fmt.Sprintf("Retrieving last %s StixCyberObservables", orderBy),
			"entity_type", entityType, "creator", creatorName, "error", err)

		return
	}

	if len(observables) == 0 {
		c.logger.ErrorContext(c.ctx, fmt.Sprintf("No last %s StixCyberObservable retrieved", orderBy))

		return
	}

	c.logger.DebugContext(c.ctx, fmt.Sprintf("Last %s StixCyberObservable", orderBy),
		"entity_type", entityType, "creator", creatorName,
		"object", fmt.Sprintf("%+v", observables[0]))

	var timestamp float64

	switch orderBy {
	case "created_at":
		timestamp = float64(observables[0].CreatedAt.Unix())
	default:
		timestamp = float64(observables[0].UpdatedAt.Unix())
	}

	ch <- prometheus.MustNewConstMetric(
		desc,
		prometheus.GaugeValue,
		timestamp,
		entityType,
		creatorName,
	)
}
