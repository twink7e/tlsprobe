package common

import (
	"context"
	"errors"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Reloader struct {
	cw     *ConfigWatcher
	ctx    context.Context
	Config *Config
}

func NewReloader(ctx context.Context, configName string) *Reloader {
	r := Reloader{
		cw:  nil,
		ctx: ctx,
	}
	cw, err := NewConfigWatcher(configName, &r)
	if err != nil {
		panic(err)
	}
	r.cw = cw
	go cw.Run()
	return &r
}

type ConfigWatcher struct {
	configName string
	osWatcher  *fsnotify.Watcher
	stoped     bool
	Reloader   *Reloader
}

func NewConfigWatcher(configName string, reloader *Reloader) (*ConfigWatcher, error) {
	if _, err := os.Stat(configName); errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	watcher.Add(configName)

	return &ConfigWatcher{
		configName,
		watcher,
		false,
		reloader,
	}, nil
}

func (r *ConfigWatcher) Run() {
	defer r.Stop()
	for {
		select {
		case e := <-r.osWatcher.Events:
			r.Reloader.eventHandler(e)
		case err := <-r.osWatcher.Errors:
			log.Info().Msgf("got an error from osWatcher: %v", err)
		}
	}
}

func ignoreSystemFile(fileName string) bool {
	return fileName == "" || fileName == "." || fileName == ".." || fileName == "/"
}

func (r *ConfigWatcher) Stop() {
	if r.stoped {
		return
	}
	r.stoped = true
	err := r.osWatcher.Close()
	if err != nil {
		log.Error().Msgf("close osWatcher failed: %v", err)
	}
}

func (r *Reloader) eventHandler(e fsnotify.Event) {
	log.Info().Msgf("got an event: %s", e)
	if e.Op != fsnotify.Write {
		return
	}
	if ignoreSystemFile(e.Name) {
		return
	}
	fileName, err := filepath.Abs(e.Name)
	if err != nil {
		log.Error().Msgf("get abs fileName(%s) error: %v", fileName, err)
		return
	}
	if err := r.LoadConfig(fileName); err != nil {
		log.Error().Msgf("reload config %s failed: %v", r.cw.configName, err)
	}
}

func (r *Reloader) LoadConfig(filename string) error {
	log.Info().Msgf("starting reload file: %s.", filename)
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	cfg := DefaultConfig()

	// set exporter maxConnections and maxCollectConnections.
	Exp.SetMaxConnections(cfg.MaxConnections)
	Exp.MaxCollectConnections = cfg.MaxCollectConnections
	if err := yaml.Unmarshal(bytes, &cfg); err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	//TODO(@xiaoshuo) should to handle already added collect's config changed and compare config.
	// if collect's config has been changed reload it.

	// hostScanner reload
	hostScannerNamesMap := make(map[string]struct{})
	for _, s := range cfg.HostScannersConfig {
		hostScannerNamesMap[s.Key()] = struct{}{}
		Exp.UpdateHostScannerConfig(r.ctx, &s, r)
	}
	// remove already deleted hostScanner.
	shouldDeleteHostScannersList := make([]string, 0)
	Exp.HostScannerRWMutex.RLock()
	for _, hs := range Exp.HostScanners {
		_, exists := hostScannerNamesMap[hs.Config.Key()]
		if !exists && hs.Creator == r {
			shouldDeleteHostScannersList = append(shouldDeleteHostScannersList, hs.Config.Key())
		}
	}
	Exp.HostScannerRWMutex.RUnlock()
	for _, key := range shouldDeleteHostScannersList {
		Exp.RemoveHostScanner(key)
	}

	// update autoDiscover.
	autoDiscoverNamesMap := make(map[string]struct{})
	for _, a := range cfg.AutoDiscover {
		autoDiscoverNamesMap[a.Name] = struct{}{}
		Exp.UpdateAutoDiscover(r.ctx, &a, r)
	}
	// remove already deleted autoDiscovers.
	shouldDeleteAutoDiscoverList := make([]string, 0)
	Exp.AutoDiscoverRWMutex.RLock()
	for _, a := range Exp.AutoDiscover {
		_, exists := autoDiscoverNamesMap[a.Config().Name]
		if !exists {
			shouldDeleteAutoDiscoverList = append(shouldDeleteAutoDiscoverList, a.Config().Name)
		}
	}
	Exp.AutoDiscoverRWMutex.RUnlock()
	for _, name := range shouldDeleteAutoDiscoverList {
		Exp.RemoveAutoDiscover(name)
	}

	// tls checker reload
	tlsCheckersNamesMap := make(map[string]struct{})
	for i, _ := range cfg.TLSCheckers {
		t := cfg.TLSCheckers[i]
		tlsCheckersNamesMap[t.Key()] = struct{}{}
		Exp.UpdateTLSChecker(r.ctx, &t, r)
	}
	// remove already deleted tlsCheckers.
	Exp.CheckerRWMutex.RLock()
	shouldDeleteTLSCheckersList := make([]string, 0)
	for _, t := range Exp.TLSCheckers {
		_, exists := tlsCheckersNamesMap[t.Key()]
		if !exists && t.Creator == r {
			shouldDeleteTLSCheckersList = append(shouldDeleteTLSCheckersList, t.Key())
		}
	}
	Exp.CheckerRWMutex.RUnlock()
	for _, key := range shouldDeleteTLSCheckersList {
		Exp.RemoveTLSChecker(key)
	}

	r.Config = &cfg
	return nil
}

func (r *Reloader) Describe() string {
	return "Config Reloader"
}
