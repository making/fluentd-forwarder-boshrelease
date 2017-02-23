/*
 * Copyright 2017 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package heartbeat_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry-community/stackdriver-tools/src/stackdriver-nozzle/heartbeat"
	"github.com/cloudfoundry-community/stackdriver-tools/src/stackdriver-nozzle/mocks"
	"github.com/cloudfoundry/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Heartbeater", func() {
	var (
		subject       heartbeat.Heartbeater
		logger        *mocks.MockLogger
		trigger       chan time.Time
		client        *mockClient
		metricAdapter heartbeat.MetricAdapter
		metricHandler heartbeat.Handler
	)

	BeforeEach(func() {
		trigger = make(chan time.Time)

		// Mock logger
		logger = &mocks.MockLogger{}

		// Mock metric handler
		client = &mockClient{}
		metricAdapter, _ = heartbeat.NewMetricAdapter("my-awesome-project", client)
		metricHandler = heartbeat.NewMetricHandler(metricAdapter, logger)

		subject = heartbeat.NewLoggerMetricHeartbeater(metricHandler, logger, trigger)
		subject.Start()
	})

	It("should start at zero", func() {
		trigger <- time.Now()

		Eventually(func() mocks.Log {
			return logger.LastLog()
		}).Should(Equal(mocks.Log{
			Level:  lager.INFO,
			Action: "heartbeater",
			Datas: []lager.Data{
				{"counters": map[string]uint{}},
			},
		}))
	})

	It("should count events", func() {
		for i := 0; i < 10; i++ {
			subject.Increment("foo")
		}

		trigger <- time.Now()

		Eventually(func() mocks.Log {
			return logger.LastLog()
		}).Should(Equal(mocks.Log{
			Level:  lager.INFO,
			Action: "heartbeater",
			Datas: []lager.Data{
				{"counters": map[string]uint{"foo": 10}},
			},
		}))

		Eventually(func() int {
			client.mutex.Lock()
			defer client.mutex.Unlock()
			return len(client.metricReqs[0].TimeSeries)
		}).Should(Equal(1))
	})

	It("should reset the heartbeater on triggers", func() {
		for i := 0; i < 10; i++ {
			subject.Increment("foo")
		}

		trigger <- time.Now()

		for i := 0; i < 5; i++ {
			subject.Increment("foo")
		}

		trigger <- time.Now()

		Eventually(func() mocks.Log {
			return logger.LastLog()
		}).Should(Equal(mocks.Log{
			Level:  lager.INFO,
			Action: "heartbeater",
			Datas: []lager.Data{
				{"counters": map[string]uint{"foo": 5}},
			},
		}))

		Eventually(func() int {
			client.mutex.Lock()
			defer client.mutex.Unlock()
			return len(client.metricReqs[len(client.metricReqs)-1].TimeSeries)
		}).Should(Equal(1))

	})

	It("should stop counting", func() {
		for i := 0; i < 5; i++ {
			subject.Increment("foo")
		}
		subject.Stop()

		Eventually(func() mocks.Log {
			return logger.LastLog()
		}).Should(Equal(mocks.Log{
			Level:  lager.INFO,
			Action: "heartbeater",
			Datas: []lager.Data{
				{"counters": map[string]uint{"foo": 5}},
			},
		}))

		subject.Increment("foo")
		Expect(logger.LastLog()).To(Equal(mocks.Log{
			Level:  lager.ERROR,
			Action: "heartbeater",
			Err:    errors.New("attempted to increment counter without starting heartbeater"),
		}))
	})

	It("can count multiple events", func() {
		for i := 0; i < 10; i++ {
			subject.Increment("foo")
		}

		for i := 0; i < 5; i++ {
			subject.Increment("bar")
		}

		trigger <- time.Now()

		Eventually(func() mocks.Log {
			return logger.LastLog()
		}).Should(Equal(mocks.Log{
			Level:  lager.INFO,
			Action: "heartbeater",
			Datas: []lager.Data{
				{"counters": map[string]uint{
					"foo": 10,
					"bar": 5,
				}},
			},
		}))

		Eventually(func() int {
			client.mutex.Lock()
			defer client.mutex.Unlock()
			return len(client.metricReqs[len(client.metricReqs)-1].TimeSeries)
		}).Should(Equal(2))

	})
})
