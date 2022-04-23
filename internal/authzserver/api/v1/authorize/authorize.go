// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package authorize implements the authorize handlers.
package authorize

import (
	"github.com/gin-gonic/gin"
	"github.com/marmotedu/component-base/pkg/core"
	"github.com/marmotedu/errors"
	"github.com/ory/ladon"

	"github.com/marmotedu/iam/internal/authzserver/authorization"
	"github.com/marmotedu/iam/internal/authzserver/authorization/authorizer"
	"github.com/marmotedu/iam/internal/pkg/code"
)

// AuthzHandler create a authorize handler used to handle authorize request.
type AuthzHandler struct {
	store authorizer.PolicyGetter
}

// NewAuthzHandler creates a authorize handler.
func NewAuthzHandler(store authorizer.PolicyGetter) *AuthzHandler {
	return &AuthzHandler{
		store: store,
	}
}

// 该函数使用 github.com/ory/ladon 包进行资源访问授权，授权流程如下图所示：
// Authorize returns whether a request is allow or deny to access a resource and do some action
// under specified condition.
func (a *AuthzHandler) Authorize(c *gin.Context) {
	// 在Authorize方法中调用 c.ShouldBind(&r) ，将API请求参数解析到 ladon.Request 类型的结构体变量中。
	var r ladon.Request
	if err := c.ShouldBind(&r); err != nil {
		core.WriteResponse(c, errors.WithCode(code.ErrBind, err.Error()), nil)

		return
	}
	// 调用authorization.NewAuthorizer函数，
	// 该函数会创建并返回包含Manager和AuditLogger字段的Authorizer类型的变量。
	auth := authorization.NewAuthorizer(authorizer.NewAuthorization(a.store))
	if r.Context == nil {
		r.Context = ladon.Context{}
	}
	// 在Authorize函数中，我们将username存入ladon Request的context中：
	r.Context["username"] = c.GetString("username")
	// 调用auth.Authorize函数，对请求进行访问授权
	rsp := auth.Authorize(&r)

	core.WriteResponse(c, nil, rsp)
}
