package middleware

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/config"
	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/shared"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const (
	ctxUserKey = "user"
	ctxIsAgent = "is_agent"
)

// Claims is the JWT payload.
type Claims struct {
	UserID  string `json:"uid"`
	IsAgent bool   `json:"is_agent"`
	jwt.RegisteredClaims
}

// MakeJWT creates a signed JWT for the given user.
func MakeJWT(user *models.User) (string, error) {
	expiry := time.Duration(config.App.JWTExpiryHours) * time.Hour
	claims := &Claims{
		UserID:  user.ID,
		IsAgent: user.IsAgent,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.App.JWTSecret))
}

// parseJWT validates a token string and returns the claims.
func parseJWT(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(config.App.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// HashAPIKey returns the SHA-256 hex of a plain-text API key.
func HashAPIKey(plain string) string {
	h := sha256.Sum256([]byte(plain))
	return fmt.Sprintf("%x", h)
}

// Auth is a Gin middleware that accepts either a JWT Bearer token or an
// X-API-Key header. It sets the resolved user in the request context.
func Auth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var user *models.User

		// 1. Try API Key (preferred for agents).
		if key := c.GetHeader("X-API-Key"); key != "" {
			hash := HashAPIKey(key)
			var u models.User
			if err := db.Where("api_key_hash = ?", hash).First(&u).Error; err == nil {
				user = &u
			}
		}

		// 2. Fall back to JWT Bearer.
		if user == nil {
			if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				tokenStr := strings.TrimPrefix(auth, "Bearer ")
				if claims, err := parseJWT(tokenStr); err == nil {
					var u models.User
					if err := db.First(&u, "id = ?", claims.UserID).Error; err == nil {
						user = &u
					}
				}
			}
		}

		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "authentication required"))
			return
		}

		c.Set(ctxUserKey, user)
		c.Set(ctxIsAgent, user.IsAgent)
		c.Next()
	}
}

// OptionalAuth is like Auth but does not abort if no credentials are provided.
// Useful for endpoints that show different content to authenticated users.
func OptionalAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var user *models.User

		if key := c.GetHeader("X-API-Key"); key != "" {
			hash := HashAPIKey(key)
			var u models.User
			if err := db.Where("api_key_hash = ?", hash).First(&u).Error; err == nil {
				user = &u
			}
		}

		if user == nil {
			if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				tokenStr := strings.TrimPrefix(auth, "Bearer ")
				if claims, err := parseJWT(tokenStr); err == nil {
					var u models.User
					if err := db.First(&u, "id = ?", claims.UserID).Error; err == nil {
						user = &u
					}
				}
			}
		}

		if user != nil {
			c.Set(ctxUserKey, user)
			c.Set(ctxIsAgent, user.IsAgent)
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user from the context, or nil.
func CurrentUser(c *gin.Context) *models.User {
	u, _ := c.Get(ctxUserKey)
	if u == nil {
		return nil
	}
	user, _ := u.(*models.User)
	return user
}

// RequireAgent ensures the authenticated principal is an agent account.
func RequireAgent() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := CurrentUser(c)
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				shared.Fail("UNAUTHORIZED", "agent authentication required"))
			return
		}
		if !user.IsAgent {
			c.AbortWithStatusJSON(http.StatusForbidden,
				shared.Fail("FORBIDDEN", "agent access required"))
			return
		}
		c.Next()
	}
}
