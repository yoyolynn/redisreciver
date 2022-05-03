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
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v7"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

// Runs intermittently, fetching info from Redis, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type redisScraper struct {
	redisSvc *redisSvc
	settings component.ReceiverCreateSettings
	mb       *metadata.MetricsBuilder
	uptime   time.Duration
}

const redisMaxDbs = 16 // Maximum possible number of redis databases

func newRedisScraper(cfg *Config, settings component.ReceiverCreateSettings) (scraperhelper.Scraper, error) {
	opts := &redis.Options{
		Addr:     cfg.Endpoint,
		Password: cfg.Password,
		Network:  cfg.Transport,
	}

	var err error
	if opts.TLSConfig, err = cfg.TLS.LoadTLSConfig(); err != nil {
		return nil, err
	}
	return newRedisScraperWithClient(newRedisClient(opts), settings, cfg)
}

func newRedisScraperWithClient(client client, settings component.ReceiverCreateSettings, cfg *Config) (scraperhelper.Scraper, error) {
	rs := &redisScraper{
		redisSvc: newRedisSvc(client),
		settings: settings,
		mb:       metadata.NewMetricsBuilder(cfg.Metrics),
	}
	return scraperhelper.NewScraper(typeStr, rs.Scrape)
}

// Scrape is called periodically, querying Redis and building Metrics to send to
// the next consumer. First builds 'fixed' metrics (non-keyspace metrics)
// defined at startup time. Then builds 'keyspace' metrics if there are any
// keyspace lines returned by Redis. There should be one keyspace line per
// active Redis database, of which there can be 16.
func (rs *redisScraper) Scrape(context.Context) (pdata.Metrics, error) {
	inf, err := rs.redisSvc.info()
	if err != nil {
		return pdata.Metrics{}, err
	}

	now := pdata.NewTimestampFromTime(time.Now())
	currentUptime, err := inf.getUptimeInSeconds()
	if err != nil {
		return pdata.Metrics{}, err
	}

	if rs.uptime == time.Duration(0) || rs.uptime > currentUptime {
		rs.mb.Reset(metadata.WithStartTime(pdata.NewTimestampFromTime(now.AsTime().Add(-currentUptime))))
	}
	rs.uptime = currentUptime

	pdm := pdata.NewMetrics()
	rm := pdm.ResourceMetrics().AppendEmpty()
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/" + typeStr)

	rs.recordCommonMetrics(now, inf)
	rs.recordKeyspaceMetrics(now, inf)
	rs.recordCommandStatsMetrics(now, inf)
	rs.recordLatencyStatsMetrics(now, inf)

	rs.mb.Emit(ilm.Metrics())

	return pdm, nil
}

// recordCommonMetrics records metrics from Redis info key-value pairs.
func (rs *redisScraper) recordCommonMetrics(ts pdata.Timestamp, inf info) {
	recorders := rs.dataPointRecorders()
	for infoKey, infoVal := range inf {
		recorder, ok := recorders[infoKey]
		if !ok {
			// Skip unregistered metric.
			continue
		}
		switch recordDataPoint := recorder.(type) {
		case func(pdata.Timestamp, int64):
			val, err := strconv.ParseInt(infoVal, 10, 64)
			if err != nil {
				rs.settings.Logger.Warn("failed to parse info int val", zap.String("key", infoKey),
					zap.String("val", infoVal), zap.Error(err))
			}
			recordDataPoint(ts, val)
		case func(pdata.Timestamp, float64):
			val, err := strconv.ParseFloat(infoVal, 64)
			if err != nil {
				rs.settings.Logger.Warn("failed to parse info float val", zap.String("key", infoKey),
					zap.String("val", infoVal), zap.Error(err))
			}
			recordDataPoint(ts, val)
		}
	}
}

// recordKeyspaceMetrics records metrics from 'keyspace' Redis info key-value pairs,
// e.g. "db0: keys=1,expires=2,avg_ttl=3".
func (rs *redisScraper) recordKeyspaceMetrics(ts pdata.Timestamp, inf info) {
	for db := 0; db < redisMaxDbs; db++ {
		key := "db" + strconv.Itoa(db)
		str, ok := inf[key]
		if !ok {
			break
		}
		keyspace, parsingError := parseKeyspaceString(db, str)
		if parsingError != nil {
			rs.settings.Logger.Warn("failed to parse keyspace string", zap.String("key", key),
				zap.String("val", str), zap.Error(parsingError))
			continue
		}
		rs.mb.RecordRedisDbKeysDataPoint(ts, int64(keyspace.keys), keyspace.db)
		rs.mb.RecordRedisDbExpiresDataPoint(ts, int64(keyspace.expires), keyspace.db)
		rs.mb.RecordRedisDbAvgTTLDataPoint(ts, int64(keyspace.avgTTL), keyspace.db)
	}
}

// recordCommandStatsMetrics records metrics from 'commandstats' Redis info key-value pairs,
// e.g. "cmdstat_set:calls=1,usec=11,usec_per_call=11.00,rejected_calls=0,failed_calls=0".
func (rs *redisScraper) recordCommandStatsMetrics(ts pdata.Timestamp, inf info) {
	for infoKey, infoVal := range inf {
		if strings.HasPrefix(infoKey, "cmdstat") {
			commandstat, parsingError := parseCommandstatString(infoKey, infoVal)
			if parsingError != nil {
				rs.settings.Logger.Warn("failed to parse commandstat string", zap.String("key", infoKey),
					zap.String("val", infoVal), zap.Error(parsingError))
				continue
			}
			rs.mb.RecordRedisCommandCallsDataPoint(ts, int64(commandstat.calls), commandstat.command)
			rs.mb.RecordRedisCommandUsecDataPoint(ts, int64(commandstat.usec), commandstat.command)
			rs.mb.RecordRedisCommandUsecPerCallDataPoint(ts, commandstat.usec_per_call, commandstat.command)
			rs.mb.RecordRedisCommandRejectedCallsDataPoint(ts, int64(commandstat.rejected_calls), commandstat.command)
			rs.mb.RecordRedisCommandFailedCallsDataPoint(ts, int64(commandstat.failed_calls), commandstat.command)
		}
	}
}

// recordLatencyStatsMetrics records metrics from 'LatencyStatsMetrics' Redis info key-value pairs,
// e.g. "latency_percentiles_usec_info:p50=10.123,p99=110.234,p99.9=120.234".
func (rs *redisScraper) recordLatencyStatsMetrics(ts pdata.Timestamp, inf info) {
	keyPrefix := "latency_percentiles_usec_"
	for infoKey, infoVal := range inf {
		if (!strings.HasPrefix(infoKey, keyPrefix)) || len(infoKey) <= len(keyPrefix) {
			continue
		}
		command := infoKey[len(keyPrefix):]
		latencystats, parsingError := parseLatencystatsString(command, infoVal)
		if parsingError != nil {
			rs.settings.Logger.Warn("failed to parse latency stats string", zap.String("command", command),
				zap.String("latencystats", infoVal), zap.Error(parsingError))
			continue
		}
		for percentile, latency := range latencystats.stats {
			switch percentile {
			case "p50":
				rs.mb.RecordRedisLatencystatP50DataPoint(ts, float64(latency), command)
			case "p90":
				rs.mb.RecordRedisLatencystatP90DataPoint(ts, float64(latency), command)
			case "p99":
				rs.mb.RecordRedisLatencystatP99DataPoint(ts, float64(latency), command)
			case "p99.9":
				rs.mb.RecordRedisLatencystatP999DataPoint(ts, float64(latency), command)
			case "p99.99":
				rs.mb.RecordRedisLatencystatP9999DataPoint(ts, float64(latency), command)
			case "p100":
				rs.mb.RecordRedisLatencystatP100DataPoint(ts, float64(latency), command)
			}
		}
	}
}
