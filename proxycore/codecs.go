// Copyright (c) DataStax, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxycore

import (
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

var codec = frame.NewRawCodec()

var primitiveCodecs = map[datatype.DataType]datatype.Codec{
	datatype.Ascii:    &datatype.AsciiCodec{},
	datatype.Bigint:   &datatype.BigintCodec{},
	datatype.Blob:     &datatype.BlobCodec{},
	datatype.Boolean:  &datatype.BooleanCodec{},
	datatype.Counter:  &datatype.CounterCodec{},
	datatype.Decimal:  &datatype.DecimalCodec{},
	datatype.Double:   &datatype.DoubleCodec{},
	datatype.Float:    &datatype.FloatCodec{},
	datatype.Inet:     &datatype.InetCodec{},
	datatype.Int:      &datatype.IntCodec{},
	datatype.Smallint: &datatype.SmallintCodec{},
	datatype.Text:     &datatype.TextCodec{},
	datatype.Varchar:  &datatype.VarcharCodec{},
	datatype.Timeuuid: &datatype.TimeuuidCodec{},
	datatype.Tinyint:  &datatype.TinyintCodec{},
	datatype.Uuid:     &datatype.UuidCodec{},
	datatype.Varint:   &datatype.VarintCodec{},
}

func EncodeType(dt datatype.DataType, version primitive.ProtocolVersion, val interface{}) ([]byte, error) {
	codec, err := codecFromDataType(dt)
	if err != nil {
		return nil, err
	}
	return codec.Encode(val, version)
}

func DecodeType(dt datatype.DataType, version primitive.ProtocolVersion, bytes []byte) (interface{}, error) {
	codec, err := codecFromDataType(dt)
	if err != nil {
		return nil, err
	}
	return codec.Decode(bytes, version)
}

func codecFromDataType(dt datatype.DataType) (datatype.Codec, error) {
	switch dt.GetDataTypeCode() {
	case primitive.DataTypeCodeList:
		listType := dt.(datatype.ListType)
		elemCodec, err := codecFromDataType(listType.GetElementType())
		if err != nil {
			return nil, err
		}
		return datatype.NewListCodec(elemCodec), nil
	case primitive.DataTypeCodeSet:
		setType := dt.(datatype.SetType)
		elemCodec, err := codecFromDataType(setType.GetElementType())
		if err != nil {
			return nil, err
		}
		return datatype.NewSetCodec(elemCodec), nil
	case primitive.DataTypeCodeMap:
		mapType := dt.(datatype.MapType)
		keyCodec, err := codecFromDataType(mapType.GetKeyType())
		if err != nil {
			return nil, err
		}
		valueCodec, err := codecFromDataType(mapType.GetValueType())
		if err != nil {
			return nil, err
		}
		return datatype.NewMapCodec(keyCodec, valueCodec), nil
	case primitive.DataTypeCodeCustom, primitive.DataTypeCodeUdt:
		return &datatype.NilDecoderCodec{}, nil
	default:
		codec, ok := primitiveCodecs[dt]
		if !ok {
			return nil, fmt.Errorf("no codec for data type %v", dt)
		}
		return codec, nil
	}
}
