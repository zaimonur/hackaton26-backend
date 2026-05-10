package http

import (
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// AuthMiddleware: İstekte geçerli bir JWT olup olmadığını kontrol eder.
func AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return respondError(c, http.StatusUnauthorized, "Erişim reddedildi: Token bulunamadı")
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return respondError(c, http.StatusUnauthorized, "Geçersiz token formatı. Beklenen: Bearer <token>")
			}

			tokenString := parts[1]
			secret := os.Getenv("JWT_SECRET")
			if secret == "" {
				secret = "drewisy_hackathon_super_secret"
			}

			token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				return respondError(c, http.StatusUnauthorized, "Geçersiz veya süresi dolmuş token")
			}

			// Token geçerliyse, içindeki verileri (claims) al ve Context'e yükle
			claims, _ := token.Claims.(jwt.MapClaims)
			c.Set("user_id", claims["user_id"])
			c.Set("role", claims["role"])

			return next(c)
		}
	}
}

// AdminMiddleware: Kullanıcının rolünün admin olup olmadığını kontrol eder.
func AdminMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := c.Get("role")
			if role != "admin" {
				return respondError(c, http.StatusForbidden, "Sadece adminler bu işlemi yapabilir") // 403 Forbidden
			}
			return next(c)
		}
	}
}
