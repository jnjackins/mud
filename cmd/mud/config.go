package main

import "github.com/jnjackins/mud"

func (c *Session) SetConfig(cfg mud.Config) {
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
