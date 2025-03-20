package proxycore

import (
	"bytes"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/datacodec"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"io"
)

func DecodeBody(raw *frame.RawFrame) (*frame.Body, error) {
	return codec.DecodeBody(raw.Header, bytes.NewReader(raw.Body))
}

func ConvertFromRawFrame(raw *frame.RawFrame) (*frame.Frame, error) {
	return codec.ConvertFromRawFrame(raw)
}

func ConvertToRawFrame(f *frame.Frame) (*frame.RawFrame, error) {
	return codec.ConvertToRawFrame(f)
}

func EncodeFrame(f *frame.Frame, writer io.Writer) error {
	return codec.EncodeFrame(f, writer)
}

func EncodeRawFrame(f *frame.RawFrame, writer io.Writer) error {
	return codec.EncodeRawFrame(f, writer)
}

var primitiveCodecs = map[datatype.DataType]datacodec.Codec{
	datatype.Ascii:    datacodec.Ascii,
	datatype.Bigint:   datacodec.Bigint,
	datatype.Blob:     datacodec.Blob,
	datatype.Boolean:  datacodec.Boolean,
	datatype.Counter:  datacodec.Counter,
	datatype.Decimal:  datacodec.Decimal,
	datatype.Double:   datacodec.Double,
	datatype.Float:    datacodec.Float,
	datatype.Inet:     datacodec.Inet,
	datatype.Int:      datacodec.Int,
	datatype.Smallint: datacodec.Smallint,
	datatype.Varchar:  datacodec.Varchar,
	datatype.Timeuuid: datacodec.Timeuuid,
	datatype.Tinyint:  datacodec.Tinyint,
	datatype.Uuid:     datacodec.Uuid,
	datatype.Varint:   datacodec.Varint,
}

func CodecFromDataType(dt datatype.DataType) (datacodec.Codec, error) {
	switch dt.GetDataTypeCode() {
	case primitive.DataTypeCodeList:
		listType := dt.(datatype.ListType)
		return datacodec.NewList(datatype.NewListType(listType.GetElementType()))
	case primitive.DataTypeCodeSet:
		setType := dt.(datatype.SetType)
		return datacodec.NewSet(datatype.NewListType(setType.GetElementType()))
	case primitive.DataTypeCodeMap:
		mapType := dt.(datatype.MapType)
		return datacodec.NewMap(datatype.NewMapType(mapType.GetKeyType(), mapType.GetValueType()))
	default:
		codec, ok := primitiveCodecs[dt]
		if !ok {
			return nil, fmt.Errorf("no codec for data type %v", dt)
		}
		return codec, nil
	}
}
