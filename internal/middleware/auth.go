package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
)

// AuthRequired JWT 認證中介軟體
//
// 安全強化（P0-5）：
//  1. 簽名算法白名單：只接受 HMAC-SHA256，防止 Algorithm Substitution Attack
//  2. 驗證 iss / aud 必須符合 synapse 簽發的 token，避免跨系統 token 誤用
//  3. 檢查 jti 是否在黑名單中（登出 / 密碼變更 / 強制登出後立即失效）
//  4. 將 jti 與 exp 存入 Gin context 供 Logout handler 寫入黑名單
//
// blacklistSvc 為必要依賴；若未啟用黑名單（測試場景），可傳 nil，
// 此時僅跳過黑名單查詢，其他驗證仍會執行。
func AuthRequired(secret string, blacklistSvc *services.TokenBlacklistService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			response.Unauthorized(c, "缺少認證令牌")
			return
		}

		// 解析並驗證 JWT（簽名算法 + iss + aud）
		token, err := jwt.Parse(
			tokenString,
			func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(secret), nil
			},
			jwt.WithIssuer(services.JWTIssuer),
			jwt.WithAudience(services.JWTAudience),
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		)
		if err != nil || !token.Valid {
			response.Unauthorized(c, "認證令牌無效")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			response.Unauthorized(c, "認證令牌格式無效")
			return
		}

		// 提取 user_id（JWT claims 中的數字預設是 float64）
		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			response.Unauthorized(c, "認證令牌中缺少使用者ID")
			return
		}

		// 提取 jti 並檢查黑名單
		jti, _ := claims["jti"].(string)
		if blacklistSvc != nil && jti != "" && blacklistSvc.IsRevoked(jti) {
			response.Unauthorized(c, "認證令牌已被撤銷")
			return
		}

		// 提取 exp 並存入 context，供 Logout handler 寫入黑名單
		var expTime time.Time
		if expFloat, ok := claims["exp"].(float64); ok {
			expTime = time.Unix(int64(expFloat), 0)
		}

		c.Set("user_id", uint(userIDFloat))
		c.Set("username", claims["username"])
		c.Set("auth_type", claims["auth_type"])
		c.Set("system_role", claims["system_role"])
		c.Set("jti", jti)
		c.Set("token_exp", expTime)

		c.Next()
	}
}

// extractToken 從 Authorization header 或 query string 取出 token 字串
// WebSocket 連線無法設定 header，只能從 query string 帶
func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			// header 存在但沒有 Bearer 前綴 → 回傳空字串，由呼叫端回 401
			return ""
		}
		return tokenString
	}
	return c.Query("token")
}
