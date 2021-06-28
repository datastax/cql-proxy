package proxy

import (
	"context"
	"cql-proxy/proxycore"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

func serveRequest(r *request) error {
	stream := r.raw.Header.StreamId

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
					res.Header.StreamId = stream
					r.client.sendRaw(res)
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
	qp         proxycore.QueryPlan
	raw        *frame.RawFrame
	ctx        context.Context
	res        chan *frame.RawFrame
	err        chan error
}

func (r *request) send(msg message.Message) {
	r.client.send(r.raw.Header, msg)
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
