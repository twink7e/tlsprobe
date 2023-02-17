package common

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"net"
	"tlsprobe/common/creator"
	"strings"
	"sync"
)

type ScanHostPorts map[uint]*TLSChecker

var EmptyPortsMap = make(map[uint]*TLSChecker)

func ShouldContinue(err error) bool {
	if strings.Index(err.Error(), "timeout") == -1 {
		return false
	} else if strings.Index(err.Error(), "Connection refused") == -1 {
		return false
	}
	return true
}

func IsUnconnectedError(err error) bool {
	if err == nil {
		return false
	}
	if strings.Index(err.Error(), "timeout") != -1 {
		return true
	} else if strings.Index(err.Error(), "connection reset") != -1 {
		return true
	}
	return false
}

func TCPConnect(addr string, dialer *net.Dialer, retryTimes uint) (conn net.Conn, err error) {
	for i := uint(0); i < retryTimes+1; i++ {
		conn, err = dialer.Dial("tcp", addr)
		if err != nil && ShouldContinue(err) {
			if conn != nil {
				conn.Close()
			}
			continue
		}
		break
	}
	return conn, err
}

func NewHostScanner(creator creator.Creator, config *HostScannerConfig, wa *WaitPool, ctx context.Context) *HostScanner {
	c, cf := context.WithCancel(ctx)
	return &HostScanner{
		Creator:    creator,
		Config:     *config,
		Ports:      make(ScanHostPorts),
		wa:         wa,
		ctx:        c,
		cancelFunc: cf,
	}
}

type HostScanner struct {
	Creator    creator.Creator
	Config     HostScannerConfig
	Ports      ScanHostPorts
	wa         *WaitPool
	ctx        context.Context
	cancelFunc context.CancelFunc
	mux        sync.RWMutex
}

func (s *HostScanner) check(port uint, addr string, dialer *net.Dialer) {
	log.Trace().Msgf("starting check addr: %s.", addr)
	rawConn, err := TCPConnect(addr, dialer, s.Config.TLSOptions.ReTryTimes)
	if err != nil && rawConn == nil {
		log.Trace().Msgf("connect to %s failed: %v, skip it.", addr, err)
		return
	}
	checker := NewTLSChecker(s, s.Config.Host, port, s.Config.TLSOptions)
	// TODO@(xiaoshuo) should add metrics when connect is failed,
	// should add metric when cert is expired.

	_, err = checker.CheckWithConn(rawConn)
	if IsUnconnectedError(err) {
		log.Debug().Msgf("hostscanner addr: %s got a unconnect error: %v", checker.Addr(), err)
		return
	}
	s.mux.Lock()
	s.Ports[port] = checker
	s.mux.Unlock()
	log.Debug().Msgf("hostscanner addr: %s check error: %v", checker.Addr(), err)

	if ShouldKeepCheckTLS(err) {
		log.Debug().Msgf("added TLSChecker host: %s", checker.Addr())
		Exp.UpdateTLSChecker(context.Background(), checker, s)
	}
}

func (s *HostScanner) Stop() {
	for _, c := range s.Ports {
		Exp.RemoveTLSChecker(c.Key())
	}
	// help gc
	s.Ports = EmptyPortsMap
	s.cancelFunc()
}

func (s *HostScanner) Scan() {
	for port := uint(10); port < 65536; port++ {
		select {
		case <-s.ctx.Done():
			log.Info().Msgf("%s stopping", s.Config.Key())
			return
		default:
		}
		addr := fmt.Sprintf("%s:%d", s.Config.Host, port)
		dialer := &net.Dialer{
			Timeout: s.Config.TLSOptions.GetTimeout(),
		}
		s.wa.Run(s.check, port, addr, dialer)
	}
}

func (s *HostScanner) GetCreator() creator.Creator {
	return s.Creator
}

func (s *HostScanner) CollectPorts(ch chan<- prometheus.Metric) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	for port, _ := range s.Ports {
		labels := prometheus.Labels{
			"port":   fmt.Sprintf("%d", port),
			"host":   s.Config.Host,
			"domain": s.Config.TLSOptions.Domain,
		}
		descer := prometheus.NewDesc("host_scanner_port", "", nil, labels)
		m, err := prometheus.NewConstMetric(descer, prometheus.GaugeValue, 1)
		if err != nil {
			log.Warn().Msgf("scanner exec NewConstMetric failed, host: %s, error: %v.", s.Config.Host, err)
			continue
		}
		ch <- m
	}
}

func (s *HostScanner) Describe() string {
	return fmt.Sprintf("HostScanner: Host: %v.", s.Config.Host)
}
