package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	GitRef string `split_words:"true" default:"main"`
}

var cfg Config

func Load() error {
	return envconfig.Process("ztoperator", &cfg)
}

func Get() Config {
	return cfg
}
