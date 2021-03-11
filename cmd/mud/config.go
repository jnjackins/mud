package main

import (
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Address string
	Login   struct {
		Name     string
		Password string
	}
	Prompt    pattern
	Abilities map[string]struct {
		Ready []string
		Wait  []string
	}
	Triggers  map[pattern]string
	Vars      map[string]string
	Aliases   map[string]string
	Log       []pattern
	Highlight []pattern
	Timers    []Timer
	Tick      struct {
		Duration time.Duration
		Match    []string
	}
}

func UnmarshalConfig(path string) (Config, error) {
	var cfg Config

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	err = yaml.Unmarshal(buf, &cfg)
	return cfg, err
}

func (c *Session) SetConfig(cfg Config) {
	c.Lock()
	defer c.Unlock()

	c.cfg = cfg

	for k, v := range cfg.Vars {
		if _, exists := c.vars[k]; !exists {
			c.vars[k] = v
		}
	}

	if c.cancelTimers != nil {
		c.cancelTimers()
	}
	c.cancelTimers = c.startTimers()
}
