package proxy

import (
	"context"
	"cql-proxy/proxycore"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"io"
)

func serveRequest(r *request) error {
	done := false
	var err error
	for !done {
		host := r.qp.Next()
		if host == nil {
			r.send(&message.Unavailable{ErrorMessage: "No more hosts available"}) // TODO: Is this the correct error to send back?
			done = true
		} else {
			err = r.session.Send(host, r)
			if err == nil {
				select {
				case err = <-r.err:
					// TODO: Handle specific errors
				case res := <-r.res:
					r.sendRaw(res)
				}
				done = true
			}
		}
	}

	if err != nil {
		r.send(&message.ServerError{ErrorMessage: fmt.Sprintf("Unable to handle request %v", err)})
	}

	return err
}

type request struct {
	client     *client
	session    *proxycore.Session
	idempotent bool
	stream     int16
	qp         proxycore.QueryPlan
	raw        *frame.RawFrame
	ctx        context.Context
	res        chan *frame.RawFrame
	err        chan error
}

func (r *request) send(msg message.Message) {
	r.client.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return codec.EncodeFrame(frame.NewFrame(r.raw.Header.Version, r.stream, msg), writer)
	}))
}

func (r *request) sendRaw(raw *frame.RawFrame) {
	raw.Header.StreamId = r.stream
	r.client.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return codec.EncodeRawFrame(raw, writer)
	}))
}

func (r *request) Frame() interface{} {
	return r.raw
}

func (r *request) OnError(err error) {
	r.err <- err
}

func (r *request) OnResult(raw *frame.RawFrame) {
	r.res <- raw
}
