package common

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"tlsprobe/autodiscover"
	"tlsprobe/common/creator"
	"sync"
	"time"
)

var Exp *Exporter = NewExporter()

type Exporter struct {
	HostScanners          map[string]*HostScanner
	TLSCheckers           map[string]*TLSChecker
	AutoDiscover          map[string]autodiscover.AutoDiscover
	MaxCollectConnections uint
	HostScannerRWMutex    *sync.RWMutex
	CheckerRWMutex        *sync.RWMutex
	AutoDiscoverRWMutex   *sync.RWMutex
	waitPool              *WaitPool
}

type UpdateAutoDiscoverType func(ctx context.Context, cfg *autodiscover.Config, creator creator.Creator)
type UpdateUpdateTLSCheckerType func(ctx context.Context, t *TLSChecker, creator creator.Creator)
type UpdateHostScannerConfigType func(ctx context.Context, s *HostScannerConfig, creator creator.Creator)

type RemoveFuncType func(string)

func NewExporter() *Exporter {
	e := &Exporter{
		HostScanners:        make(map[string]*HostScanner),
		TLSCheckers:         make(map[string]*TLSChecker),
		AutoDiscover:        make(map[string]autodiscover.AutoDiscover),
		CheckerRWMutex:      new(sync.RWMutex),
		HostScannerRWMutex:  new(sync.RWMutex),
		AutoDiscoverRWMutex: new(sync.RWMutex),
	}
	// set default connections.
	e.SetMaxConnections(100)
	return e
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
}

func (e *Exporter) SetMaxConnections(limit uint) {
	if e.waitPool != nil {
		return
	}
	e.waitPool = NewWaitPool(limit)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	// hostScanner
	startTime := time.Now()
	e.HostScannerRWMutex.RLock()
	wa := NewWaitPool(e.MaxCollectConnections)
	for _, s := range e.HostScanners {
		log.Info().Msgf("collect hostscanner: %s", s.Config.Key())
		s.CollectPorts(ch)
	}
	e.HostScannerRWMutex.RUnlock()
	// checker
	e.CheckerRWMutex.RLock()
	defer e.CheckerRWMutex.RUnlock()
	for _, t := range e.TLSCheckers {
		wa.Run(t.CollectTLSStatus, ch)
	}
	wa.Wait()
	log.Debug().Msgf("collect total time: %s", time.Since(startTime))
}

func (e *Exporter) RemoveHostScanner(key string) {
	e.HostScannerRWMutex.Lock()
	defer e.HostScannerRWMutex.Unlock()
	hostScanner, exists := e.HostScanners[key]
	if !exists {
		return
	}
	hostScanner.Stop()
	delete(e.HostScanners, key)
	log.Info().Msgf("deleted hostScanner: %s", key)
}

func (e *Exporter) UpdateHostScannerConfig(ctx context.Context, s *HostScannerConfig, creator creator.Creator) {
	e.HostScannerRWMutex.Lock()
	defer e.HostScannerRWMutex.Unlock()
	oldHs, exists := e.HostScanners[s.Key()]
	if exists && oldHs.Config != *s {
		log.Info().Msgf("reload hostScanner: %s", s.Key())
		oldHs.Stop()
	}
	if !exists {
		log.Info().Msgf("added hostScanner: %s", s.Key())
		hostScanner := NewHostScanner(creator, s, e.waitPool, ctx)
		e.HostScanners[s.Key()] = hostScanner
		go hostScanner.Scan()
	}

}

func (e *Exporter) UpdateTLSChecker(ctx context.Context, t *TLSChecker, creator creator.Creator) {
	e.CheckerRWMutex.Lock()
	defer e.CheckerRWMutex.Unlock()
	t.Creator = creator
	e.TLSCheckers[t.Key()] = t
}

func (e *Exporter) RemoveTLSChecker(key string) {
	e.CheckerRWMutex.Lock()
	e.CheckerRWMutex.Unlock()
	log.Info().Msgf("delete tls checker: %s", key)
	delete(e.TLSCheckers, key)
}

func (e *Exporter) UpdateAutoDiscover(ctx context.Context, cfg *autodiscover.Config, creator creator.Creator) {
	e.AutoDiscoverRWMutex.Lock()
	defer e.AutoDiscoverRWMutex.Unlock()
	autoDiscover, exists := e.AutoDiscover[cfg.Name]
	// autoDiscover already exists, and config not changed.
	if exists && autoDiscover.Config().CompareConfig(cfg) {
		return
	}
	// autoDiscover does not exist, or config has been changed
	if autoDiscover != nil {
		autoDiscover.Stop()
	}
	autoDiscover, err := autodiscover.CreateAutoDiscover(ctx, cfg, creator)
	if err != nil {
		log.Error().Msgf("create autoDiscover failed: %s", err)
	}
	e.AutoDiscover[cfg.Name] = autoDiscover
	autoDiscover.Start()
}

func (e *Exporter) RemoveAutoDiscover(name string) {
	e.AutoDiscoverRWMutex.Lock()
	defer e.AutoDiscoverRWMutex.Unlock()
	ad, exists := e.AutoDiscover[name]
	if exists {
		ad.Stop()
		delete(e.AutoDiscover, name)
	}
}
