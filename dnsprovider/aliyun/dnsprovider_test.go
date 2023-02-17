package aliyun

import (
	"context"
	"testing"
	"time"
	"tlsprobe/autodiscover"
)

func TestCreator(t *testing.T) {
	cfg := autodiscover.Config{
		Name: "aliyun-test1",
		Type: "aliyun",
		Options: map[string]string{
			"accessKeyId":     "",
			"accessKeySecret": "",
		},
	}
	ac, err := Creator(context.Background(), &cfg, nil)
	if err != nil {
		panic(err)
	}
	ac.Start()
	time.Sleep(15 * time.Second)
	ac.Stop()
}
