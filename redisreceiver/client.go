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

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"strings"

	"github.com/go-redis/redis/v7"
)

// Interface for a Redis client. Implementation can be faked for testing.
type client interface {
	// retrieves a string of key/value pairs of redis metadata
	retrieveInfo() (string, error)
	// line delimiter
	// redis lines are delimited by \r\n, files (for testing) by \n
	delimiter() string
}

// Wraps a real Redis client, implements `client` interface.
type redisClient struct {
	client *redis.Client
}

var _ client = (*redisClient)(nil)

// Creates a new real Redis client from the passed-in redis.Options.
func newRedisClient(options *redis.Options) client {
	return &redisClient{
		client: redis.NewClient(options),
	}
}

// Redis strings are CRLF delimited.
func (c *redisClient) delimiter() string {
	return "\r\n"
}

// Retrieve Redis INFO. We retrieve all of the 'sections'.
func (c *redisClient) retrieveInfo() (string, error) {
	defaultInfo, err := c.client.Info().Result()
	if err != nil {
		return "", err
	}

	commandstatsInfo, err := c.client.Info("commandstats").Result()
	if err != nil {
		return "", err
	}

	lantencystatsInfo, err := c.client.Info("latencystats").Result()
	if err != nil {
		return "", err
	}

	return strings.Join([]string{defaultInfo, commandstatsInfo, lantencystatsInfo}, c.delimiter()), nil
}
