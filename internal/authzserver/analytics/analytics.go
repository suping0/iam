// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package analytics defines functions and structs used to store authorization audit data to redis.
package analytics

import (
	"sync"
	"sync/atomic"
	"time"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"

	"github.com/marmotedu/iam/pkg/log"
	"github.com/marmotedu/iam/pkg/storage"
)

const analyticsKeyName = "iam-system-analytics"

const (
	recordsBufferForcedFlushInterval = 1 * time.Second
)

// AnalyticsRecord encodes the details of a authorization request.
type AnalyticsRecord struct {
	TimeStamp  int64     `json:"timestamp"`
	Username   string    `json:"username"`
	Effect     string    `json:"effect"`
	Conclusion string    `json:"conclusion"`
	Request    string    `json:"request"`
	Policies   string    `json:"policies"`
	Deciders   string    `json:"deciders"`
	ExpireAt   time.Time `json:"expireAt"   bson:"expireAt"`
}

var analytics *Analytics

// SetExpiry set expiration time to a key.
func (a *AnalyticsRecord) SetExpiry(expiresInSeconds int64) {
	expiry := time.Duration(expiresInSeconds) * time.Second
	if expiresInSeconds == 0 {
		// Expiry is set to 100 years
		expiry = 24 * 365 * 100 * time.Hour
	}

	t := time.Now()
	t2 := t.Add(expiry)
	a.ExpireAt = t2
}

// Analytics will record analytics data to a redis back end as defined in the Config object.
type Analytics struct {
	store                      storage.AnalyticsHandler
	poolSize                   int
	recordsChan                chan *AnalyticsRecord
	workerBufferSize           uint64
	recordsBufferFlushInterval uint64
	shouldStop                 uint32
	poolWg                     sync.WaitGroup
}

// NewAnalytics returns a new analytics instance.
func NewAnalytics(options *AnalyticsOptions, store storage.AnalyticsHandler) *Analytics {
	ps := options.PoolSize
	recordsBufferSize := options.RecordsBufferSize
	workerBufferSize := recordsBufferSize / uint64(ps)
	log.Debug("Analytics pool worker buffer size", log.Uint64("workerBufferSize", workerBufferSize))
	// recordsChan 存放的数据类型为AnalyticsRecord，缓冲区的大小为 recordsBufferSize （通过 --analytics.records-buffer-size 选项指定）。
	recordsChan := make(chan *AnalyticsRecord, recordsBufferSize)

	return &Analytics{
		store:                      store,
		poolSize:                   ps,
		recordsChan:                recordsChan,
		workerBufferSize:           workerBufferSize,
		recordsBufferFlushInterval: options.FlushInterval,
	}
}

// GetAnalytics returns the existed analytics instance.
func GetAnalytics() *Analytics {
	return analytics
}

// Start start the analytics service.
func (r *Analytics) Start() {
	analytics = r
	r.store.Connect()

	// start worker pool
	atomic.SwapUint32(&r.shouldStop, 0)
	// options.PoolSize由命令行参数 --analytics.pool-size 指定，代表worker 的个数，默认 50
	for i := 0; i < r.poolSize; i++ {
		r.poolWg.Add(1)
		go r.recordWorker()
	}

	// stop analytics workers
	go r.Stop()
}

// Stop stop the analytics service.
func (r *Analytics) Stop() {
	// flag to stop sending records into channel
	atomic.SwapUint32(&r.shouldStop, 1)

	// close channel to stop workers
	close(r.recordsChan)

	// wait for all workers to be done
	r.poolWg.Wait()
}

// RecordHit will store an AnalyticsRecord in Redis.
// 可以通过RecordHit函数，向recordsChan 中写入 AnalyticsRecord 类型的数据
func (r *Analytics) RecordHit(record *AnalyticsRecord) error {
	// check if we should stop sending records 1st
	if atomic.LoadUint32(&r.shouldStop) > 0 {
		return nil
	}

	// just send record to channel consumed by pool of workers
	// leave all data crunching and Redis I/O work for pool workers
	r.recordsChan <- record

	return nil
}

// recordWorker函数会从 recordsChan 中读取授权日志并存入recordsBuffer中
func (r *Analytics) recordWorker() {
	defer r.poolWg.Done()

	// this is buffer to send one pipelined command to redis
	// use r.recordsBufferSize as cap to reduce slice re-allocations
	// options.RecordsBufferSize由命令行参数 --analytics.records-buffer-size 指定，代表缓存的授权日志消息数。也就是说，我们把缓存的记录平均分配给所有的worker。
	recordsBuffer := make([][]byte, 0, r.workerBufferSize)

	// read records from channel and process
	lastSentTS := time.Now()
	for {
		var readyToSend bool
		select {
		case record, ok := <-r.recordsChan:
			// check if channel was closed and it is time to exit from worker
			if !ok {
				// send what is left in buffer
				r.store.AppendToSetPipelined(analyticsKeyName, recordsBuffer)

				return
			}

			// we have new record - prepare it and add to buffer
			// 为了提高传输速率，这里将日志内容编码为msgpack格式后再传输。
			if encoded, err := msgpack.Marshal(record); err != nil {
				log.Errorf("Error encoding analytics data: %s", err.Error())
			} else {
				recordsBuffer = append(recordsBuffer, encoded)
			}

			// identify that buffer is ready to be sent
			readyToSend = uint64(len(recordsBuffer)) == r.workerBufferSize

		case <-time.After(time.Duration(r.recordsBufferFlushInterval) * time.Millisecond):
			// nothing was received for that period of time
			// anyways send whatever we have, don't hold data too long in buffer
			readyToSend = true
		}

		// 当recordsBuffer存满或者达到投递最大时间后，调用 r.Store.AppendToSetPipelined(analyticsKeyName, recordsBuffer) 将记录批量发送给Redis，
		// send data to Redis and reset buffer
		if len(recordsBuffer) > 0 && (readyToSend || time.Since(lastSentTS) >= recordsBufferForcedFlushInterval) {
			r.store.AppendToSetPipelined(analyticsKeyName, recordsBuffer)
			recordsBuffer = recordsBuffer[:0]
			lastSentTS = time.Now()
		}
	}
}

// DurationToMillisecond convert time duration type to floag64.
func DurationToMillisecond(d time.Duration) float64 {
	return float64(d) / 1e6
}
