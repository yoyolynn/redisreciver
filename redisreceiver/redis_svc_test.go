// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newFakeAPIParser() *redisSvc {
	return newRedisSvc(fakeClient{})
}

func TestParser(t *testing.T) {
	s := newFakeAPIParser()
	info, err := s.info()
	require.Nil(t, err)
	require.Equal(t, 125+5, len(info))                                                                         // with 5 additional latencyStats lines
	require.Equal(t, "1.24", info["allocator_frag_ratio"])                                                     // spot check
	require.Equal(t, "calls=2,usec=4,usec_per_call=2.00,rejected_calls=0,failed_calls=0", info["cmdstat_get"]) // check commandstats
	require.Equal(t, "p50=10.123,p99=110.234,p99.9=120.234", info["latency_percentiles_usec_info"])
}
