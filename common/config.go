package common

import (
	"fmt"
	"tlsprobe/autodiscover"
)

type HostScannerConfig struct {
	Host       string          `yaml:"host"`
	TLSOptions TLSCheckOptions `yaml:"TLSOptions"`
}

func (h *HostScannerConfig) Key() string {
	return fmt.Sprintf("HostScanner Host: %s, domain: %s", h.Host, h.TLSOptions.Domain)
}

type Config struct {
	AutoDiscover          []autodiscover.Config `yaml:"autoDiscover"`
	HostScannersConfig    []HostScannerConfig   `yaml:"hostScannersConfig"`
	TLSCheckers           []TLSChecker          `yaml:"TLSCheckers"`
	MaxConnections        uint                  `yaml:"maxConnections"`
	MaxCollectConnections uint                  `yaml:"maxCollectConnections"`
	ListenAddr            string                `yaml:"listenAddr"`
}
