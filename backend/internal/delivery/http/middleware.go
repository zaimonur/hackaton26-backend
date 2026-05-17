package http

import (
	"errors"
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

			claims, err := parseToken(parts[1])
			if err != nil {
				return respondError(c, http.StatusUnauthorized, err.Error())
			}

			c.Set("user_id", claims["user_id"])
			c.Set("role", claims["role"])

			return next(c)
		}
	}
}

// RBACMiddleware: Belirtilen rollerden herhangi birine sahip olmayı zorunlu kılar.
func RBACMiddleware(allowedRoles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			roleObj := c.Get("role")
			if roleObj == nil {
				return respondError(c, http.StatusForbidden, "Erişim reddedildi: Rol bulunamadı")
			}

			userRole := roleObj.(string)
			for _, role := range allowedRoles {
				if role == userRole {
					return next(c)
				}
			}
			return respondError(c, http.StatusForbidden, "Bu işlem için yetkiniz yok")
		}
	}
}

func parseToken(tokenString string) (jwt.MapClaims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("sunucu yapılandırma hatası: JWT_SECRET eksik")
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("geçersiz veya süresi dolmuş token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("token claims okunamadı")
	}

	return claims, nil
}
