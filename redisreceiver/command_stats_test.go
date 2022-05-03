// Copyright  The OpenTelemetry Authors
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

package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCommandStat(t *testing.T) {
	cmdstat, err := parseCommandstatString("cmdstat_get", "calls=1,usec=2,usec_per_call=3.29,rejected_calls=4,failed_calls=5")
	require.Nil(t, err)
	require.Equal(t, "cmdstat_get", cmdstat.command)
	require.Equal(t, 1, cmdstat.calls)
	require.Equal(t, 2, cmdstat.usec)
	require.Equal(t, 3.29, cmdstat.usec_per_call)
	require.Equal(t, 4, cmdstat.rejected_calls)
	require.Equal(t, 5, cmdstat.failed_calls)
}

func TestParseMalformedCommandstat(t *testing.T) {
	tests := []struct{ name, commandstat, errorMsg string }{
		{"missing value", "calls=1,usec=2,usec_per_call=3.29,rejected_calls=4,failed_calls=", "strconv.Atoi: parsing \"\": invalid syntax"},
		{"missing equals", "calls=1,usec=2,usec_per_call=3.29,rejected_calls=4,failed_calls", "unexpected commandstat pair 'failed_calls'"},
		{"unexpected key", "xyz,calls=1,usec=2", "unexpected commandstat pair 'xyz'"},
		{"no usable data", "foo", "unexpected commandstat pair 'foo'"},
		{"empty data", "", "unexpected commandstat pair ''"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseCommandstatString("cmdstat_get", test.commandstat)
			require.NotNil(t, err)
			require.Equal(t, test.errorMsg, err.Error())
		})
	}
}
