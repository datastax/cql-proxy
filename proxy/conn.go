package proxy

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"sync"
)

var (
	Closed = errors.New("connection closed")
	AlreadyClosed = errors.New("connection already closed")
)

const (
	MaxMessages = 1024
	MaxCoalesceSize = 16 * 1024
)

type Conn struct {
	conn   net.Conn
	closed chan struct{}
	messages chan Sender
	err    error
	recv   Receiver
	mu     sync.Mutex
}

type Receiver interface {
	Receive(reader io.Reader) error
}

type Sender interface {
	Send(writer io.Writer) error
	Closing(err error)
}

func Connect(network string, address string, recv Receiver) (*Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}

	return FromConn(conn, recv)
}

func FromConn(conn net.Conn, recv Receiver) (*Conn, error) {
	c := &Conn{
		conn:   conn,
		closed: make(chan struct{}),
		messages: make(chan Sender, MaxMessages),
		recv: recv,
	}

	go c.read()
	go c.write()

	return c, nil
}

func (c *Conn) read() {
	done := false
	for !done {
		done = c.checkErr(c.recv.Receive(c.conn))
	}
	log.Println("reader closed")
}

func (c *Conn) write() {
	done := false
	writer := bytes.NewBuffer(make([]byte, 0))
	senders := make([]Sender, 0)

	for {
		select {
		case sender := <- c.messages:
			done = c.checkErr(sender.Send(writer))
			if !done {
				senders = append(senders, sender)
			}
			coalescing := true
			for coalescing && !done && writer.Len() < MaxCoalesceSize {
				select {
				case sender, coalescing = <- c.messages:
					done = c.checkErr(sender.Send(writer))
					if !done {
						senders = append(senders, sender)
					}
				case <- c.closed:
					done = true
				default:
					coalescing = false
				}
			}
		case <- c.closed:
			done = true
		}

		n, err := c.conn.Write(writer.Bytes())
		done = c.checkErr(err)
		if done {
			for _, sender := range senders {
				sender.Closing(c.Err())
			}
			break
		}
		log.Printf("wrote %d bytes, %d senders", n, len(senders))
		senders = senders[:0]
		writer.Reset()
	}
	log.Println("writer closed")
}

func (c *Conn) Write(sender Sender) error {
	select {
	case c.messages <- sender:
		return nil
	case <- c.closed:
		return c.Err()
	}
}

func (c *Conn) checkErr(err error) bool {
	if err != nil {
		c.mu.Lock()
		if c.err == nil {
			c.err = err
			close(c.closed)
		}
		c.mu.Unlock()
		return true
	}
	return false
}

func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return AlreadyClosed
	}
	close(c.closed)
	c.err = Closed
	return c.conn.Close()
}

func (c *Conn) Err() error {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	return err
}

func (c *Conn) IsClosed() chan struct{} {
	return c.closed
}