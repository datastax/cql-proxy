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

	"github.com/datastax/go-cassandra-native-protocol/datacodec"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

var codec = frame.NewRawCodec()

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

func EncodeType(dt datatype.DataType, version primitive.ProtocolVersion, val interface{}) ([]byte, error) {
	c, err := codecFromDataType(dt)
	if err != nil {
		return nil, err
	}
	return c.Encode(val, version)
}

func DecodeType(dt datatype.DataType, version primitive.ProtocolVersion, bytes []byte) (interface{}, error) {
	c, err := codecFromDataType(dt)
	if err != nil {
		return nil, err
	}
	var dest interface{}
	_, err = c.Decode(bytes, &dest, version)
	return dest, err
}

func codecFromDataType(dt datatype.DataType) (datacodec.Codec, error) {
	switch dt.Code() {
	case primitive.DataTypeCodeList:
		listType := dt.(*datatype.List)
		return datacodec.NewList(datatype.NewList(listType.ElementType))
	case primitive.DataTypeCodeSet:
		setType := dt.(*datatype.Set)
		return datacodec.NewSet(datatype.NewSet(setType.ElementType))
	case primitive.DataTypeCodeMap:
		mapType := dt.(*datatype.Map)
		return datacodec.NewMap(datatype.NewMap(mapType.KeyType, mapType.ValueType))
	default:
		codec, ok := primitiveCodecs[dt]
		if !ok {
			return nil, fmt.Errorf("no codec for data type %v", dt)
		}
		return codec, nil
	}
}
