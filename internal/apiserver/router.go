// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package apiserver

import (
	"github.com/gin-gonic/gin"
	"github.com/marmotedu/component-base/pkg/core"
	"github.com/marmotedu/errors"

	"github.com/marmotedu/iam/internal/apiserver/api/v1/policy"
	"github.com/marmotedu/iam/internal/apiserver/api/v1/secret"
	"github.com/marmotedu/iam/internal/apiserver/api/v1/user"
	"github.com/marmotedu/iam/internal/apiserver/store/mysql"
	"github.com/marmotedu/iam/internal/pkg/code"
	"github.com/marmotedu/iam/internal/pkg/middleware"
	"github.com/marmotedu/iam/internal/pkg/middleware/auth"

	// custom gin validators.
	_ "github.com/marmotedu/iam/pkg/validator"
)

func initRouter(g *gin.Engine) {
	installMiddleware(g)
	installAPI(g)
}

func installMiddleware(g *gin.Engine) {
}

func installAPI(g *gin.Engine) *gin.Engine {
	// Middlewares.
	jwtStrategy, _ := newJWTAuth().(auth.JWTStrategy)
	// 登录的认证和JWT的签发
	g.POST("/login", jwtStrategy.LoginHandler)
	// LogoutHandler其实是用来清空Cookie中Bearer认证相关信息的
	g.POST("/logout", jwtStrategy.LogoutHandler)
	// Refresh time can be longer than token timeout
	g.POST("/refresh", jwtStrategy.RefreshHandler)

	auto := newAutoAuth()
	g.NoRoute(auto.AuthFunc(), func(c *gin.Context) {
		core.WriteResponse(c, errors.WithCode(code.ErrPageNotFound, "Page not found."), nil)
	})

	// v1 handlers, requiring authentication
	storeIns, _ := mysql.GetMySQLFactoryOr(nil)
	v1 := g.Group("/v1")
	{
		// user RESTful resource
		userv1 := v1.Group("/users")
		{
			userHandler := user.NewUserHandler(storeIns)

			userv1.POST("", userHandler.Create)
			userv1.Use(auto.AuthFunc(), middleware.Validation())
			// v1.PUT("/find_password", userHandler.FindPassword)
			userv1.DELETE("", userHandler.DeleteCollection) // admin api
			userv1.DELETE(":name", userHandler.Delete)      // admin api
			userv1.PUT(":name/change-password", userHandler.ChangePassword)
			userv1.PUT(":name", userHandler.Update)
			userv1.GET("", userHandler.List)
			userv1.GET(":name", userHandler.Get) // admin api
		}

		v1.Use(auto.AuthFunc())

		// policy RESTful resource
		policyv1 := v1.Group("/policies", middleware.Publish())
		{
			policyHandler := policy.NewPolicyHandler(storeIns)

			policyv1.POST("", policyHandler.Create)
			policyv1.DELETE("", policyHandler.DeleteCollection)
			policyv1.DELETE(":name", policyHandler.Delete)
			policyv1.PUT(":name", policyHandler.Update)
			policyv1.GET("", policyHandler.List)
			policyv1.GET(":name", policyHandler.Get)
		}

		// secret RESTful resource
		secretv1 := v1.Group("/secrets", middleware.Publish())
		{
			secretHandler := secret.NewSecretHandler(storeIns)

			secretv1.POST("", secretHandler.Create)
			secretv1.DELETE(":name", secretHandler.Delete)
			secretv1.PUT(":name", secretHandler.Update)
			secretv1.GET("", secretHandler.List)
			secretv1.GET(":name", secretHandler.Get)
		}
	}

	return g
}
