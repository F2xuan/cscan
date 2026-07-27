package middleware

import (
	"fmt"

	"github.com/golang-jwt/jwt/v4"
)

// ValidateJWTToken 验证 HMAC 签名的 JWT,并返回 MapClaims。
// 供 WebSocket / SSE 等无法走标准 AuthMiddleware 的入口复用,
// 避免各 handler 各自复制签名校验逻辑导致后续演进漂移。
//
// 返回:
//   - 成功: claims, nil
//   - 签名方法不匹配 / 解析失败 / claims 无效: nil, err
func ValidateJWTToken(tokenString, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}
