// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package apiserver

import "github.com/marmotedu/iam/internal/apiserver/config"

// Run runs the specified APIServer. This should never exit.
func Run(cfg *config.Config) error {
	// 创建HTTP/GRPC服务器所使用的配置
	server, err := createAPIServer(cfg)
	if err != nil {
		return err
	}
	// 最后，调用PrepareRun方法，进行HTTP/GRPC服务器启动前的准备。
	return server.PrepareRun().Run()
}
