// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// this file contains factories with no other dependencies

package util

import (
	"github.com/marmotedu/marmotedu-sdk-go/marmotedu/service/iam"
	restclient "github.com/marmotedu/marmotedu-sdk-go/rest"
	"github.com/marmotedu/marmotedu-sdk-go/tools/clientcmd"

	"github.com/marmotedu/iam/pkg/cli/genericclioptions"
)

type factoryImpl struct {
	clientGetter genericclioptions.RESTClientGetter
}

func NewFactory(clientGetter genericclioptions.RESTClientGetter) Factory {
	if clientGetter == nil {
		panic("attempt to instantiate client_access_factory with nil clientGetter")
	}

	f := &factoryImpl{
		clientGetter: clientGetter,
	}

	return f
}

func (f *factoryImpl) ToRESTConfig() (*restclient.Config, error) {
	// 调用 f.ToRawIAMConfigLoader().ClientConfig()
	return f.clientGetter.ToRESTConfig()
}

func (f *factoryImpl) ToRawIAMConfigLoader() clientcmd.ClientConfig {
	return f.clientGetter.ToRawIAMConfigLoader()
}

// 通过IAMClient返回SDK客户端。
// marmotedu.Clientset 提供了iam-apiserver的所有接口。
func (f *factoryImpl) IAMClient() (*iam.IamClient, error) {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return iam.NewForConfig(clientConfig)
}

// 通过RESTClient()返回RESTful API客户端，
func (f *factoryImpl) RESTClient() (*restclient.RESTClient, error) {
	// f.ToRESTConfig 函数最终是调用toRawIAMConfigLoader函数来生成配置的。
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	setIAMDefaults(clientConfig)
	return restclient.RESTClientFor(clientConfig)
}
