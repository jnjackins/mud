package main

import (
	"io"
	"os"

	"9fans.net/go/acme"
)

type session struct {
	name string
	fin  io.WriteCloser
	fout io.ReadCloser
	wout *acme.Win
	wcmd *acme.Win
	errc chan error
	cfg  config
}

func loadSessions(paths []string) ([]*session, error) {
	var sessions []*session

	for _, dir := range paths {
		in, err := os.OpenFile(dir+"/in", os.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
		out, err := os.Open(dir + "/out")
		if err != nil {
			return nil, err
		}
		out.Seek(0, io.SeekEnd)
		cfg, err := loadConfig(dir + "/conf")
		if err != nil {
			return nil, err
		}

		sess := &session{
			name: dir,
			fin:  in,
			fout: out,
			cfg:  cfg,
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (s *session) start() error {
	s.errc = make(chan error)
	go s.abilities()
	return <-s.errc
}

func (s *session) abilities() {
	win, err := acme.New()
	if err != nil {
		s.errc <- err
		return
	}
	go func() {
		for c := range commands(win) {
			s.fin.Write([]byte(c))
		}
	}()

	for {
	}
}
