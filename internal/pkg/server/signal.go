// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"os"
	"os/signal"
)

var onlyOneSignalHandler = make(chan struct{})

var shutdownHandler chan os.Signal

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalHandler() <-chan struct{} {
	// 通过 close(onlyOneSignalHandler)来确保 iam-apiserver 组件的代码只调用一次 SetupSignalHandler 函数
	// 因为同一个channnel不能被关闭两次
	close(onlyOneSignalHandler) // panics when called twice
	// 创建 channel 用来接收 os.Interrupt（SIGINT）和 syscall.SIGTERM（SIGKILL）信号.
	// 这里要注意：signal.Notify(c chan<- os.Signal, sig ...os.Signal)函数不会为了向 c 发送信息而阻塞。
	//也就是说，如果发送时 c 阻塞了，signal 包会直接丢弃信号。为了不丢失信号，我们创建了有缓冲的 channel shutdownHandler。
	shutdownHandler = make(chan os.Signal, 2)

	stop := make(chan struct{})

	signal.Notify(shutdownHandler, shutdownSignals...)
	// SetupSignalHandler 函数还实现了一个功能：收到一次 SIGINT/ SIGTERM 信号，程序优雅关闭。
	// 收到两次 SIGINT/ SIGTERM 信号，程序强制关闭。实现代码如下：
	go func() {
		<-shutdownHandler
		close(stop)
		<-shutdownHandler
		os.Exit(1) // second signal. Exit directly.
	}()
	// 最后，SetupSignalHandler 函数会返回 stop，后面的代码可以通过关闭 stop 来结束代码的阻塞状态。
	return stop
}

// RequestShutdown emulates a received event that is considered as shutdown signal (SIGTERM/SIGINT)
// This returns whether a handler was notified.
func RequestShutdown() bool {
	if shutdownHandler != nil {
		select {
		case shutdownHandler <- shutdownSignals[0]:
			return true
		default:
		}
	}

	return false
}
