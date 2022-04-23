// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"time"

	jwt "github.com/dgrijalva/jwt-go/v4"
	"github.com/gin-gonic/gin"
	"github.com/marmotedu/component-base/pkg/core"
	"github.com/marmotedu/errors"

	"github.com/marmotedu/iam/internal/pkg/code"
	"github.com/marmotedu/iam/internal/pkg/middleware"
)

// Defined errors.
var (
	ErrMissingKID    = errors.New("Invalid token format: missing kid field in claims")
	ErrMissingSecret = errors.New("Can not obtain secret information from cache")
)

// Secret contains the basic information of the secret key.
type Secret struct {
	Username string
	ID       string
	Key      string
	Expires  int64
}

// CacheStrategy defines jwt bearer authentication strategy which called `cache strategy`.
// Secrets are obtained through grpc api interface and cached in memory.
type CacheStrategy struct {
	get func(kid string) (Secret, error)
}

var _ middleware.AuthStrategy = &CacheStrategy{}

// NewCacheStrategy create cache strategy with function which can list and cache secrets.
func NewCacheStrategy(get func(kid string) (Secret, error)) CacheStrategy {
	return CacheStrategy{get}
}

// AuthFunc defines cache strategy as the gin authentication middleware.
func (cache CacheStrategy) AuthFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 第一步，从Authorization: Bearer XX.YY.ZZ请求头中获取XX.YY.ZZ，XX.YY.ZZ即为JWT Token。
		header := c.Request.Header.Get("Authorization")
		if len(header) == 0 {
			core.WriteResponse(c, errors.WithCode(code.ErrMissingHeader, "Authorization header cannot be empty."), nil)
			c.Abort()

			return
		}

		var rawJWT string
		// 解析http header, 得到jwt
		// Parse the header to get the token part.
		fmt.Sscanf(header, "Bearer %s", &rawJWT)

		// 第二步，调用github.com/dgrijalva/jwt-go包提供的ParseWithClaims函数，该函数会依次执行下面四步操作。
		// Use own validation logic, see below 验证逻辑
		var secret Secret
		// claims JWT中payload的字段
		claims := &jwt.MapClaims{}
		// 调用ParseUnverified函数，依次执行以下操作：解析token并验证
		// 调用传入的func(token *jwt.Token)  获取密钥
		// Verify the token
		parsedT, err := jwt.ParseWithClaims(rawJWT, claims,
			// 可以看到，keyFunc接受 *Token 类型的变量。返回JWT的Salt
			func(token *jwt.Token) (interface{}, error) {
				// Validate the alg is HMAC signature
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				// 获取Token Header中的kid，kid即为密钥ID：secretID。
				kid, ok := token.Header["kid"].(string)
				if !ok {
					return nil, ErrMissingKID
				}

				var err error
				// 接着，调用cache.get(kid)获取密钥secretKey。cache.get函数即为getSecretFunc
				secret, err = cache.get(kid)
				if err != nil {
					return nil, ErrMissingSecret
				}
				// getSecretFunc函数会根据kid，从内存中查找密钥信息，密钥信息中包含了secretKey。
				return []byte(secret.Key), nil
			},
			jwt.WithAudience(AuthzAudience))
		if err != nil || !parsedT.Valid {
			core.WriteResponse(c, errors.WithCode(code.ErrSignatureInvalid, err.Error()), nil)
			c.Abort()

			return
		}

		// 第三步，调用KeyExpired，验证secret是否过期。secret信息中包含过期时间，你只需要拿该过期时间和当前时间对比就行。
		if KeyExpired(secret.Expires) {
			tm := time.Unix(secret.Expires, 0).Format("2006-01-02 15:04:05")
			core.WriteResponse(c, errors.WithCode(code.ErrExpired, "expired at: %s", tm), nil)
			c.Abort()

			return
		}

		// 第四步，设置HTTP Headerusername: colin。
		c.Set(middleware.UsernameKey, secret.Username)
		c.Next()
	}
}

// KeyExpired checks if a key has expired, if the value of user.SessionState.Expires is 0, it will be ignored.
func KeyExpired(expires int64) bool {
	if expires >= 1 {
		return time.Now().After(time.Unix(expires, 0))
	}

	return false
}
