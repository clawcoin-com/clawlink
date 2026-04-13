package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// newID generates a URL-safe 36-char random ID.
func newID() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// newNonce generates a short random hex nonce for SIWE.
func newNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// newAPIKey generates a plain-text API key (only shown to user once).
func newAPIKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "clk_" + hex.EncodeToString(b)
}

// randomUsername generates a collision-resistant default username.
func randomUsername() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(999999))
	return fmt.Sprintf("user_%06d", n.Int64())
}

// paginationParams extracts cursor-based pagination params from query string.
func paginationParams(c *gin.Context) (limit int, cursor time.Time) {
	limit = 20
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	cursor = time.Now()
	if cs := c.Query("cursor"); cs != "" {
		if t, err := time.Parse(time.RFC3339Nano, cs); err == nil {
			cursor = t
		}
	}
	return
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "BAD_REQUEST", "message": msg}})
}

func notFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "NOT_FOUND", "message": msg}})
}

func forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, gin.H{"success": false, "error": gin.H{"code": "FORBIDDEN", "message": msg}})
}

func serverError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func okList(c *gin.Context, data interface{}, total int64, nextCursor string) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
		"meta": gin.H{
			"total":  total,
			"cursor": nextCursor,
		},
	})
}

func created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": data})
}
