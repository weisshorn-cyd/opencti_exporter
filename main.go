package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"github.com/sourcegraph/conc/pool"
	"github.com/weisshorn-cyd/gocti"

	"github.com/weisshorn-cyd/opencti_exporter/collector"

	_ "go.uber.org/automaxprocs"
)

const (
	exporterName = "opencti_exporter"

	serverShutdownTimeout = 30 * time.Second
	readTimeout           = 5 * time.Second
	writeTiemout          = 10 * time.Second
	idleTimeout           = 30 * time.Second
)

type envConfig struct {
	Port     string     `envconfig:"PORT"      default:"10031" desc:"Port to run the HTTP server on"`
	LogLevel slog.Level `envconfig:"LOG_LEVEL" default:"info"  desc:"Which log level to log at"`

	OpenctiURL   string `envconfig:"OPENCTI_URL"   default:"http://opencti:8080" desc:"OpenCTI URL to connect to"`
	OpenctiToken string `envconfig:"OPENCTI_TOKEN" required:"true"               desc:"OpenCTI token to use"`

	MetricsSubsystem string `envconfig:"METRICS_SUBSYSTEM" default:""         desc:"The Prometheus subsystem for the metrics"`
	MetricsPath      string `envconfig:"METRICS_PATH"      default:"/metrics" desc:"The path to access the metrics"`
}

func init() {
	prometheus.MustRegister(versioncollector.NewCollector(exporterName))
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		slog.Default().Error("Processing env var", "error", err)
		os.Exit(1)
	}

	if err := run(env); err != nil {
		slog.Default().Error("Running "+exporterName, "error", err)
		os.Exit(1)
	}
}

func run(env envConfig) error {
	promslogConfig := &promslog.Config{}
	toolkitFlags := kingpinflag.AddFlags(kingpin.CommandLine, ":"+env.Port)
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print(exporterName))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if err := promslogConfig.Level.Set(env.LogLevel.String()); err != nil {
		return fmt.Errorf("setting log level: %w", err)
	}

	logger := promslog.New(promslogConfig)

	logger.Info("Starting "+exporterName, "version", version.Info())
	logger.Info("Build context", "build_context", version.BuildContext())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctxPool := pool.New().
		WithContext(ctx).
		WithFirstError().
		WithCancelOnError()

	opencti, err := gocti.NewOpenCTIAPIClient(
		env.OpenctiURL, env.OpenctiToken,
		gocti.WithLogLevel(env.LogLevel),
	)
	if err != nil {
		return fmt.Errorf("creating OpenCTI client: %w", err)
	}

	oc := collector.NewOpenCTICollector(ctx, opencti, env.MetricsSubsystem, logger.With("url", env.OpenctiURL))

	prometheus.MustRegister(oc)

	logger.DebugContext(ctx, "OpenCTI collector initialized")

	http.Handle(env.MetricsPath, promhttp.Handler())

	if env.MetricsPath != "/" {
		landingConfig := web.LandingConfig{
			Name:        "OpenCTI Exporter",
			Description: "Prometheus Exporter for OpenCTI",
			Version:     version.Info(),
			Links: []web.LandingLinks{
				{
					Address: env.MetricsPath,
					Text:    "Metrics",
				},
			},
		}

		landingPage, err := web.NewLandingPage(landingConfig)
		if err != nil {
			return fmt.Errorf("creating landing page: %w", err)
		}

		http.Handle("/", landingPage)
	}

	srv := &http.Server{
		Addr:         fmt.Sprint(":", env.Port),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTiemout,
		IdleTimeout:  idleTimeout,
	}

	ctxPool.Go(func(_ context.Context) error {
		if err := web.ListenAndServe(srv, toolkitFlags, logger); err != nil {
			return fmt.Errorf("HTTP server crashed: %w", err)
		}

		return nil
	})

	ctxPool.Go(func(ctx context.Context) error {
		<-ctx.Done()

		logger.DebugContext(ctx, "Context is finished")

		ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil { //nolint:contextcheck,lll // this is a bug https://github.com/kkHAIKE/contextcheck/issues/2
			return fmt.Errorf("shutting down http server: %w", err)
		}

		return nil
	})

	if err = ctxPool.Wait(); err != nil {
		return fmt.Errorf("error in goroutine pool: %w", err)
	}

	return nil
}
