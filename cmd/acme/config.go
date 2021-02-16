package main

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type config struct{}

func loadConfig(path string) (config, error) {
	var cfg config

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	err = yaml.Unmarshal(buf, &cfg)
	return cfg, err
}
