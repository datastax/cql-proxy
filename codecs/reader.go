package codecs

import (
	"bytes"
	"io"
)

// FrameBodyReader is an [io.Reader] that also contains a reference to the underlying bytes buffer for a frame body.
// This is used to decode "partial" decode message types without requiring copying the underlying data for certain frame
// fields. This can avoid extra allocations and copies.
type FrameBodyReader struct {
	*bytes.Reader
	Body []byte
}

func NewFrameBodyReader(b []byte) *FrameBodyReader {
	return &FrameBodyReader{
		Reader: bytes.NewReader(b),
		Body:   b,
	}
}

func (r *FrameBodyReader) Position() int64 {
	pos, _ := r.Seek(0, io.SeekCurrent) // Doesn't fail
	return pos
}

func (r *FrameBodyReader) BytesSince(pos int64) []byte {
	return r.Body[pos:r.Position()]
}

func (r *FrameBodyReader) RemainingBytes() []byte {
	return r.Body[r.Position():]
}
