// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package apiserver does all of the work necessary to create a iam APIServer.
package apiserver

import (
	"github.com/marmotedu/iam/internal/apiserver/config"
	"github.com/marmotedu/iam/internal/apiserver/options"
	"github.com/marmotedu/iam/pkg/app"
	"github.com/marmotedu/iam/pkg/log"
)

const commandDesc = `The IAM API server validates and configures data
for the api objects which include users, policies, secrets, and
others. The API Server services REST operations to do the api objects management.

Find more iam-apiserver information at:
    https://github.com/marmotedu/iam/blob/master/docs/guide/en-US/cmd/iam-apiserver.md`

// NewApp creates a App object with default parameters.
func NewApp(basename string) *app.App {
	// 过`opts := options.NewOptions()`创建带有默认值的Options类型变量opts
	opts := options.NewOptions()
	application := app.NewApp("IAM API Server",
		basename,
		app.WithOptions(opts),
		app.WithDescription(commandDesc),
		app.WithDefaultValidArgs(),
		// run函数是iam-apiserver的启动函数，里面封装了我们自定义的启动逻辑。
		// run函数中，首先会初始化日志包，这样我们就可以根据需要，在后面的代码中随时记录日志了。
		app.WithRunFunc(run(opts)),
	)

	return application
}

func run(opts *options.Options) app.RunFunc {
	return func(basename string) error {
		log.Init(opts.Log)
		defer log.Flush()
		// 通过CreateConfigFromOptions函数来构建应用配置。
		// 创建应用配置：应用配置和Options配置其实是完全独立的，二者可能完全不同，
		// 但在iam-apiserver中，二者配置项是相同的。
		cfg, err := config.CreateConfigFromOptions(opts)
		if err != nil {
			return err
		}
		// 之后，根据应用配置，创建HTTP/GRPC服务器所使用的配置
		return Run(cfg)
	}
}
