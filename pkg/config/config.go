package config

import (
	"log"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	GitRef string `required:"true" split_words:"true"`
}

var ZtoperatorConfig Config

func init() {
	err := envconfig.Process("ztoperator", &ZtoperatorConfig)
	if err != nil {
		log.Fatal("failed to load application config: " + err.Error())
	}
}
