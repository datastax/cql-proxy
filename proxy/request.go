package proxy

import (
	"github.com/datastax/go-cassandra-native-protocol/frame"
)

func serveRequest(r request) error {
	return nil
}

type request struct {
	client *client
	raw    *frame.RawFrame
}

func (r request) Frame() interface{} {
	return r.raw
}

func (r request) OnError(err error) {
	panic("implement me")
}

func (r request) OnResult(frame *frame.Frame) {
	panic("implement me")
}
