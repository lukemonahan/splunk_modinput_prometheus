// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Example: go test -run ''

package main

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/util/testutil"
)

func TestFormatMetricLabelValue(t *testing.T) {
	tests := []struct {
		value  string
		parse  bool
		prefix string
		output model.LabelValue
	}{
		{
			value:  "__name__",
			parse:  true,
			prefix: "",
			output: "__name__",
		},
		{
			value:  "__name__",
			parse:  true,
			prefix: "DEV.",
			output: "__name__",
		},
		{
			value:  "container_network_receive_errors_total",
			output: "container_network_receive_errors_total",
		},
		{
			value:  "container_network_receive_errors_total",
			output: "container.network.receive.errors.total",
			parse:  true,
		},
		{
			value:  "container_network_receive_errors_total",
			output: "container.network.receive.errors.total",
			parse:  true,
			prefix: "",
		},
		{
			value:  "container_network_receive_errors_total",
			output: "DEV.container.network.receive.errors.total",
			parse:  true,
			prefix: "DEV.",
		},
		{
			value:  "container_network_receive_errors_total",
			output: "DEV.container_network_receive_errors_total",
			parse:  false,
			prefix: "DEV.",
		},
	}

	for _, test := range tests {
		res := formatMetricLabelValue(test.value, test.parse, test.prefix)
		testutil.Equals(t, res, test.output)
	}
}
