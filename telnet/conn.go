package telnet

import (
	"bytes"
	"net"
	"sync"
)

// A Conn is a telnet connection.
type Conn struct {
	net.Conn

	buf []byte

	processor *tnProcessor
	// Event/msg (out-of-band) handlers
	handlers     []*handlerRunner
	handlerMutex sync.Mutex
	// Some channels for command sequences?
}

// AddHandler adds a new out-of-band msg handler that will be invoked for
// IAC + ... commands. It is the responsibility of the handler to check for
// how applicable the message is for them. bytes.HasPrefix(msg, []byte{...})
// works well for determining this.
//
// Each handler will run in its own goroutine, and has a small buffer of pending
// messages it can handle. It is the responsibility of the handler to return as
// quickly as possible to prevent itself from being forcibly removed.
func (t *Conn) AddHandler(h Handler) {
	runner := &handlerRunner{
		msgChan: make(chan []byte, 100),
		handler: h,
	}
	go runner.run()

	t.handlerMutex.Lock()
	defer t.handlerMutex.Unlock()
	t.handlers = append(t.handlers, runner)
}

// Dial will attempt to make a Conn to the type/address specific
// eg: conn.Dial("tcp", "somewhere.com:23")
func Dial(network string, url string) (*Conn, error) {
	tc := &Conn{
		processor: newTelnetProcessor(),
		handlers:  make([]*handlerRunner, 0),
	}
	tc.processor.conn = tc

	c, err := net.Dial(network, url)
	if err != nil {
		return nil, err
	}
	tc.Conn = c

	startSystemHandlers(tc)
	return tc, nil
}

// Conn implements the io.Reader interface.
func (t *Conn) Read(b []byte) (int, error) {
	if t.buf == nil {
		t.buf = make([]byte, 1024)
	}
	n, err := t.Conn.Read(t.buf)
	t.processor.processBytes(t.buf[:n])
	if err != nil {
		return n, err
	}

	n, err = t.processor.Read(b)
	if n < len(b) && err == nil && b[n-1] != '\x04' {
		// waiting at a prompt?
		b[n] = '\x04' // EOT
		return n + 1, nil
	}
	return n, err
}

// SendCommand formats and sends a command (series of tnSeq) to the server.
// eg: conn.SendCommand(telnet.WILL, telnet.GMCP).
// IAC is prefixed already, so there's no need to prepend it.
func (t *Conn) SendCommand(command ...tnSeq) {
	t.Write(buildCommand(command...))
}

// Internal function to IACize some commands and turn 'em into bytes
func buildCommand(c ...tnSeq) []byte {
	var cmd bytes.Buffer

	cmd.WriteByte(byte(IAC))

	// Don't want to double up on IAC
	if c[0] == IAC {
		c = c[1:]
	}

	for _, v := range c {
		cmd.WriteByte(byte(v))
	}

	return cmd.Bytes()
}
