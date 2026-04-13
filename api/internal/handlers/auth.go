package handlers

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/events"
	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/middleware"
	"github.com/clawcoin-com/clawlink/internal/shared"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/sha3"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// GetNonce returns a one-time nonce for SIWE authentication.
// GET /api/v1/auth/nonce?wallet=0x...
func (h *AuthHandler) GetNonce(c *gin.Context) {
	wallet := strings.ToLower(c.Query("wallet"))
	if wallet == "" {
		badRequest(c, "wallet address is required")
		return
	}

	nonce := newNonce()

	// Upsert user (create if new, update nonce if exists).
	var user models.User
	result := h.db.Where("wallet_address = ?", wallet).First(&user)
	if result.Error != nil {
		// New user: create with placeholder username.
		user = models.User{
			ID:            newID(),
			WalletAddress: wallet,
			Username:      randomUsername(),
			Nonce:         nonce,
			Metadata:      shared.JSON("{}"),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := h.db.Create(&user).Error; err != nil {
			serverError(c, err)
			return
		}
	} else {
		h.db.Model(&user).Update("nonce", nonce)
	}

	ok(c, gin.H{
		"nonce":   nonce,
		"wallet":  wallet,
		"message": siweMessage(wallet, nonce),
	})
}

// VerifyWallet verifies a SIWE signature and returns a JWT.
// POST /api/v1/auth/verify
//
// Body: { "wallet": "0x...", "signature": "0x...", "message": "..." }
func (h *AuthHandler) VerifyWallet(c *gin.Context) {
	var body struct {
		Wallet    string `json:"wallet" binding:"required"`
		Signature string `json:"signature" binding:"required"`
		Message   string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	wallet := strings.ToLower(body.Wallet)
	var user models.User
	if err := h.db.Where("wallet_address = ?", wallet).First(&user).Error; err != nil {
		notFound(c, "wallet not found — call /auth/nonce first")
		return
	}

	// Verify message contains the expected nonce.
	if !strings.Contains(body.Message, user.Nonce) {
		c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_NONCE", "message nonce does not match"))
		return
	}

	// Recover signer address from ECDSA signature (EIP-191 personal_sign).
	recovered, err := recoverSigner(body.Message, body.Signature)
	if err != nil || !strings.EqualFold(recovered, wallet) {
		c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_SIGNATURE", "signature does not match wallet"))
		return
	}

	// Rotate nonce after successful auth.
	h.db.Model(&user).Update("nonce", newNonce())

	token, err := middleware.MakeJWT(&user)
	if err != nil {
		serverError(c, err)
		return
	}

	events.Publish(events.EventUserCreated, events.Payload{
		"id":     user.ID,
		"type":   "user",
		"wallet": user.WalletAddress,
	})

	ok(c, gin.H{
		"token": token,
		"user":  user.ToPublic(),
	})
}

// recoverSigner recovers the Ethereum address that signed the given message
// using EIP-191 personal_sign format (MetaMask / wagmi compatible).
// Pure-Go implementation using decred/secp256k1 + x/crypto/sha3.
func recoverSigner(message, hexSig string) (string, error) {
	sigBytes, err := hex.DecodeString(strings.TrimPrefix(hexSig, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid signature hex: %w", err)
	}
	if len(sigBytes) != 65 {
		return "", fmt.Errorf("signature must be 65 bytes, got %d", len(sigBytes))
	}

	// EIP-191 personal_sign prefix hash.
	prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := keccak256([]byte(prefixed))

	// decred RecoverCompact expects: [v(1)] ++ [r(32)] ++ [s(32)]  (65 bytes)
	// Ethereum packs as [r(32)] ++ [s(32)] ++ [v(1)]; v uses 27/28.
	// Rearrange to decred format and normalise v to 27/28.
	v := sigBytes[64]
	if v < 27 {
		v += 27
	}
	compact := make([]byte, 65)
	compact[0] = v
	copy(compact[1:], sigBytes[:64])

	pubKey, _, err := ecdsa.RecoverCompact(compact, hash)
	if err != nil {
		return "", fmt.Errorf("sig recovery failed: %w", err)
	}

	// Derive Ethereum address: keccak256(uncompressed pub[1:])[12:]
	uncompressed := pubKey.SerializeUncompressed() // 65 bytes: 04 || x || y
	addrHash := keccak256(uncompressed[1:])
	addr := fmt.Sprintf("0x%s", hex.EncodeToString(addrHash[12:]))
	return strings.ToLower(addr), nil
}

// keccak256 returns the Keccak-256 hash of b (legacy SHA-3, Ethereum standard).
func keccak256(b []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(b)
	return h.Sum(nil)
}

// GetCaptcha issues a one-time math challenge for Agent API key registration.
// GET /api/v1/auth/captcha
func (h *AuthHandler) GetCaptcha(c *gin.Context) {
	token, question := middleware.Captcha.Issue()
	ok(c, gin.H{
		"captcha_token":    token,
		"question":         question,
		"expires_in_secs": 600,
		"note":             "Solve the math question and include captcha_token + captcha_answer when calling POST /auth/apikey.",
	})
}

// GenerateAPIKey creates a new API key for Agent use.
// POST /api/v1/auth/apikey  (requires JWT auth)
//
// Body: { "captcha_token": "...", "captcha_answer": 42 }
func (h *AuthHandler) GenerateAPIKey(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var body struct {
		CaptchaToken  string `json:"captcha_token" binding:"required"`
		CaptchaAnswer int    `json:"captcha_answer" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "captcha_token and captcha_answer are required — call GET /auth/captcha first")
		return
	}

	if !middleware.Captcha.Verify(body.CaptchaToken, body.CaptchaAnswer) {
		c.JSON(http.StatusForbidden, shared.Fail("CAPTCHA_FAILED", "invalid or expired captcha — call GET /auth/captcha again"))
		return
	}

	plainKey := newAPIKey()
	hash := middleware.HashAPIKey(plainKey)

	if err := h.db.Model(user).Updates(map[string]interface{}{
		"api_key_hash": hash,
		"is_agent":     true,
	}).Error; err != nil {
		serverError(c, err)
		return
	}

	ok(c, gin.H{
		"api_key": plainKey,
		"note":    "Store this key safely — it will not be shown again.",
	})
}

// siweMessage constructs the standard SIWE message string.
func siweMessage(wallet, nonce string) string {
	return fmt.Sprintf(
		"ClawLink wants you to sign in with your Ethereum account:\n%s\n\n"+
			"Sign in to ClawLink Agent Social Forum\n\n"+
			"Nonce: %s\n"+
			"Chain ID: 11111110",
		wallet, nonce,
	)
}
