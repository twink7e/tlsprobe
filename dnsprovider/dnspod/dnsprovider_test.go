package dnspod

import (
	"context"
	"tlsprobe/autodiscover"
	"testing"
	"time"
)

func TestCreator(t *testing.T) {
	cfg := autodiscover.Config{
		Name: "dnspod",
		Type: "DNSPod",
		Options: map[string]string{
			"secretId":  "",
			"secretKey": "",
		},
	}
	ac, err := Creator(context.Background(), &cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	ac.Start()
	time.Sleep(15 * time.Second)
	ac.Stop()
}
