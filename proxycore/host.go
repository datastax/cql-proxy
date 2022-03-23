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

import "errors"

type Host struct {
	endpoint Endpoint
	DC       string
}

func NewHostFromRow(endpoint Endpoint, row Row) (*Host, error) {
	val, err := row.ByName("data_center")
	if err != nil {
		return nil, err
	}
	if dc, ok := val.(string); !ok {
		return nil, errors.New("'data_center' is not a string")
	} else {
		return &Host{endpoint, dc}, nil
	}
}

func (h *Host) Endpoint() Endpoint {
	return h.endpoint
}

func (h *Host) Key() string {
	return h.endpoint.Key()
}

func (h *Host) String() string {
	return h.endpoint.String()
}
