package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/clay-wangzhi/Synapse/internal/response"
)

// AuthRequired JWT認證中介軟體
func AuthRequired(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// 優先從請求頭獲取token
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			// 檢查Bearer字首
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				response.Unauthorized(c, "認證令牌格式錯誤")
				return
			}
		} else {
			// 如果請求頭沒有token，嘗試從URL查詢參數獲取（用於WebSocket）
			tokenString = c.Query("token")
		}

		if tokenString == "" {
			response.Unauthorized(c, "缺少認證令牌")
			return
		}

		// 解析JWT token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			response.Unauthorized(c, "認證令牌無效")
			return
		}

		// 提取使用者資訊
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// 處理user_id型別轉換（JWT claims中的數字預設是float64）
			if userIDFloat, ok := claims["user_id"].(float64); ok {
				c.Set("user_id", uint(userIDFloat))
			} else {
				response.Unauthorized(c, "認證令牌中缺少使用者ID")
				return
			}
			c.Set("username", claims["username"])
			c.Set("auth_type", claims["auth_type"])
		} else {
			response.Unauthorized(c, "認證令牌格式無效")
			return
		}

		c.Next()
	}
}
