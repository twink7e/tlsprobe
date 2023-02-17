package common

import (
	"crypto/tls"
	"fmt"
	"testing"
)

func TestTLSCheck(t *testing.T) {
	addr := "354g1920y53kp58o.aliyunddos1002.com:80"
	cfg := &tls.Config{
		ServerName:         "www.yunpian.cn",
		InsecureSkipVerify: true,
	}
	conn, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Println(conn.ConnectionState().PeerCertificates[0].Subject, conn.ConnectionState().PeerCertificates[0].NotAfter)
}

func TestShouldKeepCheckTLS(t *testing.T) {
	addr := "www.yunpian.com:80"
	_, err := tls.Dial("tcp", addr, &tls.Config{})
	fmt.Println(ShouldKeepCheckTLS(err))
	if ShouldKeepCheckTLS(err) {
		t.Fatal(err)
	}
}
