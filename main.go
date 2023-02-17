package main

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"go.uber.org/automaxprocs/maxprocs"
	"net/http"
	"tlsprobe/autodiscover"
	"tlsprobe/common"
	aliyundnsprovider "tlsprobe/dnsprovider/aliyun"
	dnspoddnsprovider "tlsprobe/dnsprovider/dnspod"
	"runtime"
)

var configFilename = pflag.StringP("config", "c", "./config.yml", "config name.")
var logLevel = pflag.StringP("level", "v", "info", "log level like trace/debug/info/warn.")

var health bool

func init() {
	pflag.Parse()
	switch *logLevel {
	case "trace":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

}

func main() {

	// Automatically set GOMAXPROCS to match Linux container CPU quota
	if _, err := maxprocs.Set(maxprocs.Logger(log.Info().Msgf)); err != nil {
		log.Fatal().Msgf("set maxprocs error: %v", err)
	}
	log.Info().Msgf("real GOMAXPROCS %d", runtime.GOMAXPROCS(-1))

	// handle Signal
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	common.SetupSignalHandler(cancelFunc)

	// registry autoDiscover Creator
	autodiscover.RegisterCreator("AliDNS", aliyundnsprovider.Creator)
	autodiscover.RegisterCreator("DNSPod", dnspoddnsprovider.Creator)

	// init config reloader
	reloader := common.NewReloader(ctx, *configFilename)
	if err := reloader.LoadConfig(*configFilename); err != nil {
		panic(err)
	}

	// always is health....
	health = true

	// prometheus handler
	if err := prometheus.Register(common.Exp); err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if health {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(500)
		}
	})
	mux.Handle("/metrics", promhttp.Handler())

	if err := http.ListenAndServe(reloader.Config.ListenAddr, mux); err != nil {
		panic(err)
	}
}
