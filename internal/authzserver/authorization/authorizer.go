// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package authorization

import (
	authzv1 "github.com/marmotedu/api/authz/v1"
	"github.com/ory/ladon"

	"github.com/marmotedu/iam/pkg/log"
)

// Authorizer implement the authorize interface that use local repository to
// authorize the subject access review.
type Authorizer struct {
	warden ladon.Warden
}

// NewAuthorizer creates a local repository authorizer and returns it.
func NewAuthorizer(authorizationClient AuthorizationInterface) *Authorizer {
	return &Authorizer{
		warden: &ladon.Ladon{
			// Manager包含一些函数，比如 Create、Update和FindRequestCandidates等，
			// 用来对授权策略进行增删改查。
			Manager:     NewPolicyManager(authorizationClient),
			// AuditLogger包含 LogRejectedAccessRequest 和 LogGrantedAccessRequest 函数，
			// 分别用来记录被拒绝的授权请求和被允许的授权请求，将其作为审计数据使用。
			AuditLogger: NewAuditLogger(authorizationClient),
		},
	}
}

// Authorize to determine the subject access.
func (a *Authorizer) Authorize(request *ladon.Request) *authzv1.Response {
	log.Debug("authorize request", log.Any("request", request))
	// IsAllowed函数会调用 FindRequestCandidates(r) 查询所有的策略列表
	// 这里要注意，我们只需要查询请求用户的policy列表。
	// 在Authorize函数中，我们将username存入ladon Request的context中： r.Context["username"] = c.GetString("username")
	//
	// 访问授权判断逻辑
	// IsAllowed会调用 DoPoliciesAllow(r, policies) 函数，来匹配policy列表。如果policy列表中有一条记录跟request不匹配，就调用LogRejectedAccessRequest函数记录拒绝的请求，并返回值为非nil的error，error中记录了授权失败的错误信息。
	// 如果所有的policy都匹配request，则调用LogGrantedAccessRequest函数记录允许的请求，并返回值为nil的error。
	if err := a.warden.IsAllowed(request); err != nil {
		return &authzv1.Response{
			Denied: true,
			Reason: err.Error(),
		}
	}

	return &authzv1.Response{
		Allowed: true,
	}
}
