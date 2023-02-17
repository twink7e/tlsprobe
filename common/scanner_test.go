package common

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"testing"
)

func TestPrometheusDesc(t *testing.T) {
	labels := prometheus.Labels{
		"port": "443",
		"host": "baidu.com",
	}
	descer := prometheus.NewDesc("host_scanner_port", "", nil, labels)
	m, err := prometheus.NewConstMetric(descer, prometheus.GaugeValue, 100)
	if err != nil {
		panic(err)
	}
	fmt.Println(m.Desc())
}
