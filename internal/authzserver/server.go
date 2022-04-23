// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package authzserver

import (
	"context"

	"github.com/marmotedu/errors"

	"github.com/marmotedu/iam/internal/authzserver/analytics"
	"github.com/marmotedu/iam/internal/authzserver/config"
	"github.com/marmotedu/iam/internal/authzserver/store"
	"github.com/marmotedu/iam/internal/authzserver/store/load"
	genericoptions "github.com/marmotedu/iam/internal/pkg/options"
	genericapiserver "github.com/marmotedu/iam/internal/pkg/server"
	"github.com/marmotedu/iam/pkg/log"
	"github.com/marmotedu/iam/pkg/shutdown"
	"github.com/marmotedu/iam/pkg/shutdown/shutdownmanagers/posixsignal"
	"github.com/marmotedu/iam/pkg/storage"
)

// RedisKeyPrefix defines the prefix key in redis for analytics data.
const RedisKeyPrefix = "analytics-"

type authzServer struct {
	gs               *shutdown.GracefulShutdown
	rpcServer        string
	clientCA         string
	redisOptions     *genericoptions.RedisOptions
	genericAPIServer *genericapiserver.GenericAPIServer
	analyticsOptions *analytics.AnalyticsOptions
}

type preparedAuthzServer struct {
	*authzServer
}

// func createAuthzServer(cfg *config.Config) (*authzServer, error) {.
func createAuthzServer(cfg *config.Config) (*authzServer, error) {
	gs := shutdown.New()
	gs.AddShutdownManager(posixsignal.NewPosixSignalManager())

	genericConfig, err := buildGenericConfig(cfg)
	if err != nil {
		return nil, err
	}

	genericServer, err := genericConfig.Complete().New()
	if err != nil {
		return nil, err
	}

	server := &authzServer{
		gs:               gs,
		redisOptions:     cfg.RedisOptions,
		analyticsOptions: cfg.AnalyticsOptions,
		rpcServer:        cfg.RPCServer,
		clientCA:         cfg.ClientCA,
		genericAPIServer: genericServer,
	}

	return server, nil
}

func (s *authzServer) PrepareRun() preparedAuthzServer {
	_ = s.initialize()

	installHandler(s.genericAPIServer.Engine)

	s.gs.AddShutdownCallback(shutdown.ShutdownFunc(func(string) error {
		s.genericAPIServer.Close()

		return nil
	}))

	return preparedAuthzServer{s}
}

// Run start to run AuthzServer.
func (s preparedAuthzServer) Run() error {
	// start shutdown managers
	if err := s.gs.Start(); err != nil {
		log.Fatalf("start shutdown manager failed: %s", err.Error())
	}

	return s.genericAPIServer.Run()
}

func buildGenericConfig(cfg *config.Config) (genericConfig *genericapiserver.Config, lastErr error) {
	genericConfig = genericapiserver.NewConfig()
	if lastErr = cfg.GenericServerRunOptions.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	if lastErr = cfg.FeatureOptions.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	if lastErr = cfg.SecureServing.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	if lastErr = cfg.InsecureServing.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	return
}

func (s *authzServer) buildStorageConfig() *storage.Config {
	return &storage.Config{
		Host:                  s.redisOptions.Host,
		Port:                  s.redisOptions.Port,
		Addrs:                 s.redisOptions.Addrs,
		MasterName:            s.redisOptions.MasterName,
		Username:              s.redisOptions.Username,
		Password:              s.redisOptions.Password,
		Database:              s.redisOptions.Database,
		MaxIdle:               s.redisOptions.MaxIdle,
		MaxActive:             s.redisOptions.MaxActive,
		Timeout:               s.redisOptions.Timeout,
		EnableCluster:         s.redisOptions.EnableCluster,
		UseSSL:                s.redisOptions.UseSSL,
		SSLInsecureSkipVerify: s.redisOptions.SSLInsecureSkipVerify,
	}
}

//nolint: govet
func (s *authzServer) initialize() error {
	ctx, cancel := context.WithCancel(context.Background())

	// keep redis connected
	go storage.ConnectToRedis(ctx, s.buildStorageConfig())

	storeIns, err := store.GetStoreInsOr(store.GetGRPCClientOrDie(s.rpcServer, s.clientCA))
	if err != nil {
		return errors.Wrap(err, "get store instance failed")
	}
	// cron to reload all secrets and policies from iam-apiserver
	/// 启动一个Load服务，用于秘钥和策略的缓存。
	load.NewLoader(ctx, storeIns).Start()

	// start analytics service
	// 启动一个Analytics服务, 用于日志缓存
	if s.analyticsOptions.Enable {
		analyticsStore := storage.RedisCluster{KeyPrefix: RedisKeyPrefix}
		// NewAnalytics函数会根据配置，创建一个Analytics实例
		analyticsIns := analytics.NewAnalytics(s.analyticsOptions, &analyticsStore)
		// 启动一个workerpoll 进程从recordsChan中读取数据，并在数据达到一定阈值之后，批量写入Redis中
		analyticsIns.Start()
		s.gs.AddShutdownCallback(shutdown.ShutdownFunc(func(string) error {
			analyticsIns.Stop()

			return nil
		}))
	}

	s.gs.AddShutdownCallback(shutdown.ShutdownFunc(func(string) error {
		cancel()

		return nil
	}))

	return nil
}
