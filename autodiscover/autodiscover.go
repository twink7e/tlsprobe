package autodiscover

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"tlsprobe/common/creator"
)

var registeredAutoDiscover map[string]AutoDiscover = make(map[string]AutoDiscover)
var registeredCreator map[string]Creator = make(map[string]Creator)

type Creator func(context.Context, *Config, creator.Creator) (AutoDiscover, error)

var ErrorNotFoundAutoDiscoverCreator error = errors.New("not Found AutoDiscover Creator")

type AutoDiscover interface {
	Start() error
	Stop() error
	Config() *Config
	Key() string
	creator.GetCreator
}

type ConfigOptions map[string]string

type Config struct {
	Name    string        `yaml:"name"`
	Type    string        `yaml:"type"`
	Options ConfigOptions `yaml:"options"`
}

func (a *Config) Key() string {
	return "AutoDiscover: " + a.Name
}

func (a *Config) CompareConfig(b *Config) bool {
	if a == nil && b == nil {
		return true
	}
	if a != b {
		return false
	}
	if a.Options == nil && b.Options == nil {
		return true
	} else if a.Options != nil && b.Options != nil && reflect.DeepEqual(a.Options, b.Options) {
		return true
	}
	return false
}

func RegisterCreator(typeName string, creator Creator) {
	registeredCreator[typeName] = creator
}

func CreateAutoDiscover(ctx context.Context, cfg *Config, creator creator.Creator) (AutoDiscover, error) {
	c, exists := registeredCreator[cfg.Type]
	if !exists {
		return nil, fmt.Errorf("CreateAutoDiscover type: %s :%w", cfg.Type, ErrorNotFoundAutoDiscoverCreator)
	}
	return c(ctx, cfg, creator)
}
