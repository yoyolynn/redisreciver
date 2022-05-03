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
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

func TestRedisRunnable(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	settings := componenttest.NewNopReceiverCreateSettings()
	settings.Logger = logger
	cfg := createDefaultConfig().(*Config)
	rs := &redisScraper{mb: metadata.NewMetricsBuilder(cfg.Metrics)}
	runner, err := newRedisScraperWithClient(newFakeClient(), settings, cfg)
	require.NoError(t, err)
	md, err := runner.Scrape(context.Background())
	require.NoError(t, err)
	// + 16 because there are two keyspace entries each of which has three metrics and two commandstats entries each of which has five metrcis
	// + 15 because there are five latency entries in ./testdata/info.txt and each of them has three different percentile stats.
	// rs.dataPointRecorders() is the number of pre-defined metrics in ./metric_functions.go
	// md.DataPointCount() is the number of recorded data points
	assert.Equal(t, len(rs.dataPointRecorders())+16+15, md.DataPointCount())
	rm := md.ResourceMetrics().At(0)
	ilm := rm.InstrumentationLibraryMetrics().At(0)
	il := ilm.InstrumentationLibrary()
	assert.Equal(t, "otelcol/redis", il.Name())
	// test the latency stats item "dbsize:p50=30.345" in testdata/info.txt
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rmi := rms.At(i)
		ilms := rmi.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			ms := ilm.Metrics()
			for k := 0; k < ms.Len(); k++ { // enumerate all metrics
				m := ms.At(k) // the k-th metrics in log
				if m.Name() == "redis.latencystat.p50" {
					for idx := 0; idx < m.Gauge().DataPoints().Len(); idx++ { // enumerate datapoints with metric name "redis.latencystat.p50".
						if m.Gauge().DataPoints().At(idx).Attributes().AsRaw()["command"] == "dbsize" { // to validate "latency_percentiles_usec_dbsize:p50=30.345" is parsed and recorded correctly.
							assert.Equal(t, 30.345, m.Gauge().DataPoints().At(idx).DoubleVal())
						}
					}
				}
			}
		}
	}
}

type customFakeClient struct {
	fakeClient
}

func newCustomFakeClient() *customFakeClient {
	return &customFakeClient{}
}

// remove logs of latency stats from fakeClient and thus set empty latency stats
func (c *customFakeClient) retrieveInfo() (string, error) {
	str, err := c.fakeClient.retrieveInfo()
	if err != nil {
		return str, err
	}
	lines := strings.Split(str, c.fakeClient.delimiter())
	newlines := make([]string, 0, len(lines))
	var isLatencyStats bool
	for _, line := range lines {
		if strings.HasPrefix(line, "+") {
			isLatencyStats = false
		}
		if strings.TrimSpace(line) == "# Latencystats" {
			isLatencyStats = true
		}
		if !isLatencyStats || len(line) == 0 { // not latency stats log
			newlines = append(newlines, line)
		}
	}
	return strings.Join(newlines, c.fakeClient.delimiter()), nil
}

func TestRedisRunnableWithEmptyLatencyStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	settings := componenttest.NewNopReceiverCreateSettings()
	settings.Logger = logger
	cfg := createDefaultConfig().(*Config)
	// load logs with empty latency stats
	runner, err := newRedisScraperWithClient(newCustomFakeClient(), settings, cfg)
	require.NoError(t, err)
	md, err := runner.Scrape(context.Background())
	require.NoError(t, err)
	// validate no latency stats data point is recorded
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rmi := rms.At(i)
		ilms := rmi.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			ms := ilm.Metrics()
			for k := 0; k < ms.Len(); k++ { // enumerate all metrics
				metricName := ms.At(k).Name() // the k-th metrics name in log
				assert.Equal(t, false, strings.HasPrefix(metricName, "redis.latencystat."))
			}
		}
	}
}

func TestNewReceiver_invalid_auth_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.TLS = configtls.TLSClientSetting{
		TLSSetting: configtls.TLSSetting{
			CAFile: "/invalid",
		},
	}
	r, err := createMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), c, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
	assert.Nil(t, r)
}
