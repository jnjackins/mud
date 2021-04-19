package main

import (
	"log"
	"os"
	"time"

	"github.com/jnjackins/mud"
)

func (c *client) serve(sess *Session) error {
	log := log.New(os.Stderr, sess.path+": ", log.LstdFlags)

	// Monitor config file for changes
	cfgPath := sess.path + "/config.yaml"
	fi, err := os.Stat(cfgPath)
	if err != nil {
		return err
	}
	mtime := fi.ModTime()
	go func() {
		for range time.Tick(5 * time.Second) {
			fi, err := os.Stat(cfgPath)
			if err != nil {
				log.Println(err)
				continue
			}
			if fi.ModTime().After(mtime) {
				cfg, err := mud.UnmarshalConfig(cfgPath)
				if err != nil {
					log.Println(err)
					continue
				}

				// copy used for mud server
				sess.SetConfig(cfg)

				log.Println("configuration updated")
				mtime = fi.ModTime()
			}
		}
	}()

	return sess.Start()
}
