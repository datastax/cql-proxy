package codecs

import (
	"github.com/datastax/go-cassandra-native-protocol/compression/lz4"
	"github.com/datastax/go-cassandra-native-protocol/compression/snappy"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

var (
	CustomMessageCodecs = []message.Codec{
		&partialQueryCodec{}, &partialExecuteCodec{}, &partialBatchCodec{},
	}

	CustomRawCodec = frame.NewRawCodec(CustomMessageCodecs...)

	CustomRawCodecsWithCompression = map[string]frame.RawCodec{
		"lz4":    frame.NewRawCodecWithCompression(&lz4.Compressor{}, CustomMessageCodecs...),
		"snappy": frame.NewRawCodecWithCompression(&snappy.Compressor{}, CustomMessageCodecs...),
	}

	DefaultRawCodec                 = frame.NewRawCodec()
	DefaultRawCodecsWithCompression = map[string]frame.RawCodec{
		"lz4":    frame.NewRawCodecWithCompression(&lz4.Compressor{}),
		"snappy": frame.NewRawCodecWithCompression(&snappy.Compressor{}),
	}

	CompressionNames = []string{"lz4", "snappy"}
)
