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

import "fmt"

type Host struct {
	Endpoint
	DC     string
	Rack   string
	Tokens []Token
}

func NewHostFromRow(endpoint Endpoint, partitioner Partitioner, row Row) (*Host, error) {
	dc, err := row.ByName("data_center")
	if err != nil {
		return nil, fmt.Errorf("error attmpting to get 'data_center' column: %v", err)
	}
	rack, err := row.ByName("rack")
	if err != nil {
		return nil, fmt.Errorf("error attmpting to get 'rack' column: %v", err)
	}
	tokensVal, err := row.ByName("tokens")
	if err != nil {
		return nil, fmt.Errorf("error attmpting to get 'tokens' column: %v", err)
	}
	tokens := make([]Token, 0, len(tokensVal.([]string)))
	for _, token := range tokensVal.([]string) {
		tokens = append(tokens, partitioner.FromString(token))
	}
	return &Host{endpoint, dc.(string), rack.(string), tokens}, nil
}
