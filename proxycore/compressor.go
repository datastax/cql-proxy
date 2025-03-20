package proxycore

import (
	"encoding/binary"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/golang/snappy"
	"github.com/pierrec/lz4/v4"
	"golang.org/x/exp/maps"
	"io"
	"slices"
)

const sizeOfUint32 = 4

var CompressionCodecs = map[string]CompressionCodec{
	"snappy": SnappyCompressionCodec{},
	"lz4":    LZ4CompressionCodec{},
}

var SupportedCompressionCodecs = buildSupportedCompressionCodecs()

// CompressionCodec is a wrapper around frame.RawCodec that handles frame compression and decompression. It's useful because
// it avoids decompressing the frame body twice on the way in; once for the raw frame to be sent to the coordinator and
// once for the "partial" types (PartialQuery, PartialExecute, etc.) that we use for internal inspection.
type CompressionCodec interface {
	DecodeFrame(reader io.Reader, maxBodyLen int32) (*frame.RawFrame, error)
	EncodeFrame(frm *frame.Frame, writer io.Writer) error
}

type UncompressedCodec struct{}

func (u UncompressedCodec) DecodeFrame(reader io.Reader, maxBodyLen int32) (*frame.RawFrame, error) {
	return decodeRawFrame(reader, maxBodyLen)
}

func (u UncompressedCodec) EncodeFrame(frm *frame.Frame, writer io.Writer) error {
	return codec.EncodeFrame(frm, writer)
}

type SnappyCompressionCodec struct{}

func (s SnappyCompressionCodec) DecodeFrame(reader io.Reader, maxBodyLen int32) (*frame.RawFrame, error) {
	raw, err := decodeRawFrame(reader, maxBodyLen)
	if err != nil {
		return nil, err
	}

	if raw.Header.BodyLength == 0 || // Nothing to decompress
		!raw.Header.Flags.Contains(primitive.HeaderFlagCompressed) { // Not compressed
		return raw, nil
	}

	decompressedLen, err := snappy.DecodedLen(raw.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to determine decompressed length: %w", err)
	}

	if int32(decompressedLen) > maxBodyLen {
		return nil, fmt.Errorf("decompressed body length %d exceeds maximum allowed %d", decompressedLen, maxBodyLen)
	}

	if raw.Body, err = snappy.Decode(nil, raw.Body); err != nil {
		return nil, fmt.Errorf("unable decompress frame body: %w", err)
	}
	raw.Header.Flags = raw.Header.Flags.Remove(primitive.HeaderFlagCompressed)
	return raw, nil
}

func (s SnappyCompressionCodec) EncodeFrame(frm *frame.Frame, writer io.Writer) error {
	return snappyCodec.EncodeFrame(frm, writer)
}

type LZ4CompressionCodec struct{}

func (l LZ4CompressionCodec) DecodeFrame(reader io.Reader, maxBodyLen int32) (*frame.RawFrame, error) {
	raw, err := decodeRawFrame(reader, maxBodyLen)
	if err != nil {
		return nil, err
	}

	if raw.Header.BodyLength == 0 || // Nothing to decompress
		!raw.Header.Flags.Contains(primitive.HeaderFlagCompressed) { // Not compressed
		return raw, nil
	}

	if len(raw.Body) < sizeOfUint32 {
		return nil, fmt.Errorf("unable to read compressed data length; expected at least %d bytes, got %d",
			sizeOfUint32, len(raw.Body))
	}

	decompressedLen := int32(binary.BigEndian.Uint32(raw.Body))

	if decompressedLen < 0 {
		return nil, fmt.Errorf("decompressed body length %d is invalid", decompressedLen)
	}

	if decompressedLen > maxBodyLen {
		return nil, fmt.Errorf("decompressed body length %d exceeds maximum allowed %d", decompressedLen, maxBodyLen)
	}

	decompressed := make([]byte, decompressedLen)

	// Skip decompression if the length is zero; otherwise, this can cause lzr.UncompressBlock to return an error.
	// Version 3.x of the java-driver has sent empty lz4-compressed bodies, so we need to handle this case.
	if decompressedLen > 0 {
		if _, err = lz4.UncompressBlock(raw.Body[sizeOfUint32:], decompressed); err != nil {
			return nil, fmt.Errorf("unable decompress frame body: %w", err)
		}
	}

	raw.Body = decompressed
	raw.Header.Flags = raw.Header.Flags.Remove(primitive.HeaderFlagCompressed)
	return raw, nil
}

func (l LZ4CompressionCodec) EncodeFrame(frm *frame.Frame, writer io.Writer) error {
	return lz4Codec.EncodeFrame(frm, writer)
}

func decodeRawFrame(reader io.Reader, maxBodyLen int32) (*frame.RawFrame, error) {
	header, err := codec.DecodeHeader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot decode frame header: %w", err)
	}

	if header.BodyLength > maxBodyLen {
		return nil, fmt.Errorf("frame body length %d exceeds maximum allowed %d", header.BodyLength, maxBodyLen)
	}

	body, err := codec.DecodeRawBody(header, reader) // Doesn't handle compression
	if err != nil {
		return nil, fmt.Errorf("cannot decode frame body: %w", err)
	}

	return &frame.RawFrame{Header: header, Body: body}, nil
}

func buildSupportedCompressionCodecs() []string {
	supported := maps.Keys(CompressionCodecs)
	slices.Sort(supported)
	return supported
}
