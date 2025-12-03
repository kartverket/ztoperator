package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	GitRef             string `split_words:"true" default:"main"`
	TokenProxyImageTag string `split_words:"true" default:"latest"`
}

var (
	cfg     Config
	IsLocal bool
)

func Load() error {
	return envconfig.Process("ztoperator", &cfg)
}

func Get() Config {
	return cfg
}
