// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package load

import (
	"context"
	"sync"
	"time"

	"github.com/marmotedu/iam/pkg/log"
)

// Store defines function to reload storage.
type Store interface {
	Reload() error
}

// Load is used to reload given storage.
type Load struct {
	ctx   context.Context
	lock  *sync.RWMutex
	store Store
}

// NewLoader return a loader.
func NewLoader(ctx context.Context, store Store) *Load {
	return &Load{
		ctx:   ctx,
		lock:  new(sync.RWMutex),
		store: store,
	}
}

// Start start a loop service.
// Start函数先启动了3个协程，再调用 l.DoReload() 完成一次密钥和策略的同步：
func (l *Load) Start() {
	// startPubSubLoop函数通过StartPubSubHandler函数，订阅Redis的 iam.cluster.notifications channel，并注册一个回调函数：
	go startPubSubLoop()
	// 1s is the minimum amount of time between hot reloads. The
	// interval counts from the start of one reload to the next.
	// reloadQueueLoop函数会监听 reloadQueue ，当发现有新的消息（这里是回调函数）写入时，会实时将消息缓存到 requeue 切片中
	go l.reloadLoop()
	// 通过reloadLoop函数启动一个timer定时器，每隔1秒会检查 requeue 切片是否为空，如果不为空，则调用 l.DoReload 方法，从iam-apiserver中拉取密钥和策略，并缓存在内存中。
	go l.reloadQueueLoop()
	l.DoReload()
}

// shouldReload returns true if we should perform any reload. Reloads happens if
// we have reload callback queued.
func shouldReload() ([]func(), bool) {
	requeueLock.Lock()
	defer requeueLock.Unlock()
	if len(requeue) == 0 {
		return nil, false
	}
	n := requeue
	requeue = []func(){}

	return n, true
}

func (l *Load) reloadLoop(complete ...func()) {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-l.ctx.Done():
			return
		// We don't check for reload right away as the gateway peroms this on the
		// startup sequence. We expect to start checking on the first tick after the
		// gateway is up and running.
		case <-ticker.C:
			cb, ok := shouldReload()
			if !ok {
				continue
			}
			start := time.Now()
			l.DoReload()
			for _, c := range cb {
				// most of the callbacks are nil, we don't want to execute nil functions to
				// avoid panics.
				if c != nil {
					c()
				}
			}
			if len(complete) != 0 {
				complete[0]()
			}
			log.Infof("reload: cycle completed in %v", time.Since(start))
		}
	}
}

// reloadQueue used to queue a reload. It's not
// buffered, as reloadQueueLoop should pick these up immediately.
var reloadQueue = make(chan func())

var requeueLock sync.Mutex

// This is a list of callbacks to execute on the next reload. It is protected by
// requeueLock for concurrent use.
var requeue []func()

func (l *Load) reloadQueueLoop(cb ...func()) {
	for {
		select {
		case <-l.ctx.Done():
			return
		case fn := <-reloadQueue:
			requeueLock.Lock()
			requeue = append(requeue, fn)
			requeueLock.Unlock()
			log.Info("Reload queued")
			if len(cb) != 0 {
				cb[0]()
			}
		}
	}
}

// DoReload reload secrets and policies.
func (l *Load) DoReload() {
	l.lock.Lock()
	defer l.lock.Unlock()

	if err := l.store.Reload(); err != nil {
		log.Errorf("faild to refresh target storage: %s", err.Error())
	}

	log.Debug("refresh target storage succ")
}
