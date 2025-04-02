package codecs

import (
	"bytes"
	"io"
)

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
