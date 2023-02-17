package common

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"net"
	"tlsprobe/common/creator"
	"strings"
	"time"
)

func ShouldKeepCheckTLS(err error) bool {
	if err == nil {
		return true
	}
	if e := errors.Unwrap(err); e.Error() == "EOF" {
		return false
	}
	if strings.Index(err.Error(), "first record does not look like a TLS handshake") != -1 {
		return false
	} else if strings.Index(err.Error(), "context deadline exceeded") != -1 {
		return false
	}
	return true

}

type TLSCheckOptions struct {
	Domain             string `yaml:"domain"`
	Timeout            uint   `yaml:"timeout"`
	InsecureSkipVerify bool   `yaml:"skipVerify"`
	ReTryTimes         uint   `yaml:"reTryTimes"`
}

func (o *TLSCheckOptions) GetTimeout() time.Duration {
	return time.Duration(o.Timeout) * time.Millisecond
}

func (o *TLSCheckOptions) ToTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: o.InsecureSkipVerify,
		ServerName:         o.Domain,
	}
}

type TLSChecker struct {
	creator.Creator
	Host            string `yaml:"host"`
	Port            uint   `yaml:"port"`
	TLSCheckOptions `yaml:"TLSCheckOptions"`
}

func NewTLSChecker(creator creator.Creator, host string, port uint, options TLSCheckOptions) *TLSChecker {
	c := &TLSChecker{
		Creator:         creator,
		Host:            host,
		Port:            port,
		TLSCheckOptions: options,
	}
	c.SetDefaultOption()
	return c
}

func (t *TLSChecker) GetCreator() creator.Creator {
	return t.Creator
}

func (t *TLSChecker) Key() string {
	return fmt.Sprintf("TLSChecker addr: %s, domain: %s", t.Addr(), t.TLSCheckOptions.Domain)
}

func (t *TLSChecker) SetDefaultOption() {
	if t.Domain == "" {
		t.Domain = t.Host
	}
	if t.Timeout == 0 {
		t.Timeout = 3000
	}
}

func (t *TLSChecker) Addr() string {
	return fmt.Sprintf("%s:%d", t.Host, t.Port)
}

func (t *TLSChecker) Check() (*tls.ConnectionState, error) {
	conn, err := net.DialTimeout("tcp", t.Addr(), t.GetTimeout())
	if err != nil {
		return nil, fmt.Errorf("tls check error: %w", err)
	}
	return t.CheckWithConn(conn)
}

func (t *TLSChecker) CheckWithConn(rawConn net.Conn) (*tls.ConnectionState, error) {
	var cancel context.CancelFunc
	ctx, cancel := context.WithTimeout(context.Background(), t.GetTimeout())
	defer cancel()
	cfg := t.ToTLSConfig()
	conn := tls.Client(rawConn, cfg)
	err := conn.HandshakeContext(ctx)
	defer conn.Close()
	if err != nil {
		return nil, fmt.Errorf("tlsChecker WithConn error: %w", err)
	}

	stat := conn.ConnectionState()
	cert := stat.PeerCertificates[0]
	log.Trace().Msgf("Subnet: %s, DNSNames: %s, NetBefore: %s, NetAfter: %s.\n", cert.Subject, cert.DNSNames, cert.NotBefore, cert.NotAfter)
	return &stat, nil
}

func (t *TLSChecker) collectTLSExpireTime(ch chan<- prometheus.Metric, stat *tls.ConnectionState) {
	labels := prometheus.Labels{
		"port":   fmt.Sprintf("%d", t.Port),
		"host":   t.Host,
		"domain": t.TLSCheckOptions.Domain,
	}
	cert := stat.PeerCertificates[0]
	dNotAfter := prometheus.NewDesc("tls_checker_not_after", "", nil, labels)
	dNotBefore := prometheus.NewDesc("tls_checker_not_before", "", nil, labels)
	mNotAfter, err := prometheus.NewConstMetric(dNotAfter, prometheus.GaugeValue, float64(cert.NotAfter.Unix()))
	mNotBefore, err := prometheus.NewConstMetric(dNotBefore, prometheus.GaugeValue, float64(cert.NotBefore.Unix()))
	if err != nil {
		log.Warn().Msgf("tls checker %s:%d exec prometheus.NewConstMetric failed, error: %v", t.Host, t.Port, err)
		return
	}
	ch <- mNotAfter
	ch <- mNotBefore
}

func (t *TLSChecker) CollectTLSStatus(ch chan<- prometheus.Metric) {
	stat, err := t.Check()
	log.Debug().Msgf("host: %v, port: %d, err: %v", t.Host, t.Port, err)
	var value float64 = 0
	labels := prometheus.Labels{
		"port":   fmt.Sprintf("%d", t.Port),
		"host":   t.Host,
		"error":  "",
		"domain": t.TLSCheckOptions.Domain,
	}
	if err != nil {
		labels["error"] = err.Error()
	} else {
		value = 1
		if len(stat.PeerCertificates) > 0 {
			cert := stat.PeerCertificates[0]
			labels["CertDNSNames"] = fmt.Sprintf("%s", cert.DNSNames)
			labels["NotBefore"] = cert.NotBefore.String()
			labels["NotAfter"] = cert.NotAfter.String()
			//labels["NotBefore"] = strconv.FormatInt(cert.NotBefore.Unix(), 10)
			//labels["NotAfter"] = strconv.FormatInt(cert.NotAfter.Unix(), 10)
		}
		t.collectTLSExpireTime(ch, stat)

	}
	descer := prometheus.NewDesc("tls_checker", "", nil, labels)
	m, err := prometheus.NewConstMetric(descer, prometheus.GaugeValue, value)
	if err != nil {
		log.Warn().Msgf("tls checker %s:%d exec prometheus.NewConstMetric failed, error: %v", t.Host, t.Port, err)
		return
	}
	ch <- m
}
