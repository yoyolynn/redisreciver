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

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"fmt"
	"strconv"
	"strings"
)

// Holds fields returned by the commandstats parameter of the INFO command: e.g.
// cmdstat_get:calls=3890526,usec=12797690,usec_per_call=3.29,rejected_calls=9,failed_calls=0
type commandstat struct {
	command        string
	calls          int
	usec           int
	usec_per_call  float64
	rejected_calls int
	failed_calls   int
}

// Turns a commandstat value (the part after the colon
// e.g. "calls=1,usec=11,usec_per_call=11.00,rejected_calls=0,failed_calls=0") into a commandstat struct
func parseCommandstatString(command string, value string) (*commandstat, error) {
	pairs := strings.Split(value, ",")
	cmdstat := commandstat{command: command}
	for _, pairStr := range pairs {
		var field *int
		var usec_per_call_field *float64
		pair := strings.Split(pairStr, "=")
		if len(pair) != 2 {
			return nil, fmt.Errorf(
				"unexpected commandstat pair '%s'",
				pairStr,
			)
		}
		key := pair[0]
		value := pair[1]
		switch key {
		case "calls":
			field = &cmdstat.calls
		case "usec":
			field = &cmdstat.usec
		case "usec_per_call":
			usec_per_call_field = &cmdstat.usec_per_call
		case "rejected_calls":
			field = &cmdstat.rejected_calls
		case "failed_calls":
			field = &cmdstat.failed_calls
		}
		if field != nil {
			val, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			*field = val
		} else if usec_per_call_field != nil {
			val, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, err
			}
			*usec_per_call_field = val
		}
	}
	return &cmdstat, nil
}
