package handlers

import (
	"crypto/tls"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/config"
	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/middleware"
	"github.com/clawcoin-com/clawlink/internal/shared"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/sha3"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// ─── Email / Password ─────────────────────────────────────────────────────────

// Register creates a new account with email + password.
// POST /api/v1/auth/register
//
// Body: { "email": "user@example.com", "password": "..." }
func (h *AuthHandler) Register(c *gin.Context) {
	var body struct {
		Email    string `json:"email"    binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	email := strings.ToLower(strings.TrimSpace(body.Email))

	// Reject if email already taken.
	var existing models.User
	if h.db.Where("email = ?", email).First(&existing).Error == nil {
		if !existing.EmailVerified {
			verifyToken := newToken()
			if err := h.db.Model(&existing).Update("email_verify_token", verifyToken).Error; err != nil {
				serverError(c, err)
				return
			}
			sendVerificationEmail(email, verifyToken)
			c.JSON(http.StatusOK, shared.OK(gin.H{
				"message": "Account exists but is not verified — we sent a fresh verification email.",
			}))
			return
		}

		c.JSON(http.StatusConflict, shared.Fail("EMAIL_TAKEN", "an account with this email already exists"))
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err != nil {
		serverError(c, err)
		return
	}

	verifyToken := newToken()
	user := models.User{
		ID:               newID(),
		Username:         randomUsername(),
		Email:            &email,
		PasswordHash:     string(hash),
		EmailVerified:    false,
		EmailVerifyToken: verifyToken,
		Metadata:         shared.JSON("{}"),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := h.db.Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "SQLSTATE 23505") {
			c.JSON(http.StatusConflict, shared.Fail("CONFLICT", "an account with the same unique identity already exists"))
			return
		}
		serverError(c, err)
		return
	}

	sendVerificationEmail(email, verifyToken)

	c.JSON(http.StatusCreated, shared.OK(gin.H{
		"message": "Registration successful — check your email to verify your account.",
	}))
}

// Login authenticates with email + password and returns a JWT.
// POST /api/v1/auth/login
//
// Body: { "email": "user@example.com", "password": "..." }
func (h *AuthHandler) Login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"    binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	email := strings.ToLower(strings.TrimSpace(body.Email))
	var user models.User
	if err := h.db.Where("email = ?", email).First(&user).Error; err != nil {
		// Generic message to prevent user enumeration.
		c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_CREDENTIALS", "incorrect email or password"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_CREDENTIALS", "incorrect email or password"))
		return
	}

	if !user.EmailVerified {
		c.JSON(http.StatusForbidden, shared.Fail("EMAIL_NOT_VERIFIED", "please verify your email before logging in"))
		return
	}

	token, err := middleware.MakeJWT(&user)
	if err != nil {
		serverError(c, err)
		return
	}

	ok(c, gin.H{
		"token": token,
		"user":  user.ToPublic(),
	})
}

// VerifyEmail marks an email address as verified and redirects to the frontend.
// GET /api/v1/auth/verify-email?token=...
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		badRequest(c, "token is required")
		return
	}

	var user models.User
	if err := h.db.Where("email_verify_token = ?", token).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, shared.Fail("INVALID_TOKEN", "verification token not found or already used"))
		return
	}

	h.db.Model(&user).Updates(map[string]any{
		"email_verified":     true,
		"email_verify_token": "",
	})

	jwt, err := middleware.MakeJWT(&user)
	if err != nil {
		serverError(c, err)
		return
	}

	// Redirect frontend with a short-lived one-time code, not the JWT itself.
	frontendURL := config.App.FrontendURL
	code := issueAuthCode(jwt)
	redirectURL := fmt.Sprintf("%s/auth/callback?code=%s", frontendURL, url.QueryEscape(code))
	c.Redirect(http.StatusFound, redirectURL)
}

// ─── OAuth ────────────────────────────────────────────────────────────────────

// oauthState is an in-memory CSRF state store (mirrors captcha pattern).
var oauthState = struct {
	sync.Mutex
	m map[string]time.Time
}{m: make(map[string]time.Time)}

var authCodeStore = struct {
	sync.Mutex
	m map[string]authCodeEntry
}{m: make(map[string]authCodeEntry)}

type authCodeEntry struct {
	JWT     string
	Expires time.Time
}

const authCodeTTL = 60 * time.Second

func issueOAuthState() string {
	state := newToken()
	oauthState.Lock()
	oauthState.m[state] = time.Now().Add(10 * time.Minute)
	oauthState.Unlock()
	return state
}

func consumeOAuthState(state string) bool {
	oauthState.Lock()
	defer oauthState.Unlock()
	exp, ok := oauthState.m[state]
	if !ok || time.Now().After(exp) {
		return false
	}
	delete(oauthState.m, state)
	return true
}

func issueAuthCode(jwt string) string {
	code := newToken()
	authCodeStore.Lock()
	authCodeStore.m[code] = authCodeEntry{JWT: jwt, Expires: time.Now().Add(authCodeTTL)}
	authCodeStore.Unlock()
	return code
}

func consumeAuthCode(code string) (string, bool) {
	authCodeStore.Lock()
	defer authCodeStore.Unlock()
	entry, ok := authCodeStore.m[code]
	if !ok || time.Now().After(entry.Expires) {
		delete(authCodeStore.m, code)
		return "", false
	}
	delete(authCodeStore.m, code)
	return entry.JWT, true
}

func init() {
	// Purge expired OAuth states every 5 minutes.
	go func() {
		for range time.Tick(5 * time.Minute) {
			oauthState.Lock()
			for k, v := range oauthState.m {
				if time.Now().After(v) {
					delete(oauthState.m, k)
				}
			}
			oauthState.Unlock()

			authCodeStore.Lock()
			for k, v := range authCodeStore.m {
				if time.Now().After(v.Expires) {
					delete(authCodeStore.m, k)
				}
			}
			authCodeStore.Unlock()
		}
	}()
}

// OAuthRedirect redirects the user to the OAuth provider authorization page.
// GET /api/v1/auth/oauth/:provider   (provider = "google" | "discord")
func (h *AuthHandler) OAuthRedirect(c *gin.Context) {
	provider := c.Param("provider")
	state := issueOAuthState()
	cfg := config.App
	if err := validateOAuthProviderConfig(provider, cfg); err != nil {
		serverError(c, err)
		return
	}

	var authURL string
	switch provider {
	case "google":
		params := url.Values{
			"client_id":     {cfg.GoogleClientID},
			"redirect_uri":  {oauthCallbackURL(cfg, "google")},
			"response_type": {"code"},
			"scope":         {"openid email profile"},
			"state":         {state},
			"access_type":   {"online"},
		}
		authURL = "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()

	case "discord":
		params := url.Values{
			"client_id":     {cfg.DiscordClientID},
			"redirect_uri":  {oauthCallbackURL(cfg, "discord")},
			"response_type": {"code"},
			"scope":         {"identify email"},
			"state":         {state},
		}
		authURL = "https://discord.com/api/oauth2/authorize?" + params.Encode()

	default:
		badRequest(c, "unsupported provider — use 'google' or 'discord'")
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// OAuthCallback handles the OAuth provider callback, mints a JWT, then
// redirects the frontend to /auth/callback?token=...
// GET /api/v1/auth/oauth/:provider/callback
func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if !consumeOAuthState(state) {
		c.JSON(http.StatusBadRequest, shared.Fail("INVALID_STATE", "OAuth state mismatch — possible CSRF"))
		return
	}
	if code == "" {
		badRequest(c, "authorization code is missing")
		return
	}

	var (
		oauthID       string
		email         string
		name          string
		avatar        string
		emailVerified bool
	)

	cfg := config.App
	if err := validateOAuthProviderConfig(provider, cfg); err != nil {
		serverError(c, err)
		return
	}
	switch provider {
	case "google":
		accessToken, err := exchangeGoogleCode(code, cfg)
		if err != nil {
			serverError(c, err)
			return
		}
		info, err := fetchGoogleUserInfo(accessToken)
		if err != nil {
			serverError(c, err)
			return
		}
		sub, ok := info["sub"].(string)
		if !ok || strings.TrimSpace(sub) == "" {
			serverError(c, fmt.Errorf("google userinfo missing sub"))
			return
		}
		oauthID = sub
		if e, ok := info["email"].(string); ok {
			email = strings.ToLower(e)
		}
		if v, ok := info["email_verified"].(bool); ok {
			emailVerified = v
		}
		if n, ok := info["name"].(string); ok {
			name = n
		}
		if pic, ok := info["picture"].(string); ok {
			avatar = pic
		}

	case "discord":
		accessToken, err := exchangeDiscordCode(code, cfg)
		if err != nil {
			serverError(c, err)
			return
		}
		info, err := fetchDiscordUserInfo(accessToken)
		if err != nil {
			serverError(c, err)
			return
		}
		id, ok := info["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			serverError(c, fmt.Errorf("discord userinfo missing id"))
			return
		}
		oauthID = id
		if e, ok := info["email"].(string); ok {
			email = strings.ToLower(e)
		}
		if v, ok := info["verified"].(bool); ok {
			emailVerified = v
		}
		if n, ok := info["username"].(string); ok {
			name = n
		}
		if aid, ok := info["avatar"].(string); ok && aid != "" {
			avatar = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", oauthID, aid)
		}

	default:
		badRequest(c, "unsupported provider")
		return
	}

	// Find or create user.
	user, err := h.findOrCreateOAuthUser(provider, oauthID, email, name, avatar, emailVerified)
	if err != nil {
		serverError(c, err)
		return
	}

	token, err := middleware.MakeJWT(user)
	if err != nil {
		serverError(c, err)
		return
	}

	redirectCode := issueAuthCode(token)
	redirectURL := fmt.Sprintf("%s/auth/callback?code=%s", cfg.FrontendURL, url.QueryEscape(redirectCode))
	c.Redirect(http.StatusFound, redirectURL)
}

func (h *AuthHandler) findOrCreateOAuthUser(provider, oauthID, email, name, avatar string, providerEmailVerified bool) (*models.User, error) {
	// 1. Try to find by (provider, oauth_id).
	var user models.User
	if h.db.Where("oauth_provider = ? AND oauth_id = ?", provider, oauthID).First(&user).Error == nil {
		return &user, nil
	}

	// 2. Try to find by email (link existing account).
	if email != "" && providerEmailVerified {
		if h.db.Where("email = ?", email).First(&user).Error == nil {
			// Link OAuth to existing account.
			h.db.Model(&user).Updates(map[string]any{
				"oauth_provider": provider,
				"oauth_id":       oauthID,
				"email_verified": true,
			})
			return &user, nil
		}
	}

	// 3. Create new user.
	displayName := name
	if displayName == "" {
		displayName = randomUsername()
	}
	emailPtr := (*string)(nil)
	if email != "" && providerEmailVerified {
		emailPtr = &email
	}
	user = models.User{
		ID:            newID(),
		Username:      randomUsername(),
		DisplayName:   displayName,
		Avatar:        avatar,
		Email:         emailPtr,
		EmailVerified: providerEmailVerified,
		OAuthProvider: &provider,
		OAuthID:       &oauthID,
		Metadata:      shared.JSON("{}"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	return &user, h.db.Create(&user).Error
}

// ExchangeAuthCode burns a short-lived one-time code and returns the freshly
// minted JWT. This keeps JWTs out of browser-visible URLs.
// POST /api/v1/auth/exchange
func (h *AuthHandler) ExchangeAuthCode(c *gin.Context) {
	var body struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "code is required")
		return
	}
	token, found := consumeAuthCode(strings.TrimSpace(body.Code))
	if !found {
		c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_CODE", "auth code expired or already used"))
		return
	}
	ok(c, gin.H{"token": token})
}

// ─── Wallet Binding ───────────────────────────────────────────────────────────

// WalletNonce issues a SIWE nonce for wallet binding.
// Requires an authenticated session (email/OAuth logged in first).
// GET /api/v1/auth/wallet/nonce?wallet=0x...
func (h *AuthHandler) WalletNonce(c *gin.Context) {
	user := middleware.CurrentUser(c)
	wallet := strings.ToLower(c.Query("wallet"))
	if wallet == "" {
		badRequest(c, "wallet address is required")
		return
	}

	// Ensure wallet is not already bound to another account.
	var existing models.User
	if h.db.Where("wallet_address = ?", wallet).First(&existing).Error == nil && existing.ID != user.ID {
		c.JSON(http.StatusConflict, shared.Fail("WALLET_TAKEN", "this wallet is already bound to another account"))
		return
	}

	nonce := newNonce()
	h.db.Model(user).Update("nonce", nonce)

	ok(c, gin.H{
		"nonce":   nonce,
		"wallet":  wallet,
		"message": siweMessage(wallet, nonce),
	})
}

// WalletBind binds a wallet to the current authenticated user.
// POST /api/v1/auth/wallet/bind
//
// Body: { "wallet": "0x...", "signature": "0x...", "message": "..." }
func (h *AuthHandler) WalletBind(c *gin.Context) {
	user := middleware.CurrentUser(c)

	var body struct {
		Wallet    string `json:"wallet"    binding:"required"`
		Signature string `json:"signature" binding:"required"`
		Message   string `json:"message"   binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	wallet := strings.ToLower(body.Wallet)

	// Ensure wallet is not already bound to another account.
	var existing models.User
	if h.db.Where("wallet_address = ?", wallet).First(&existing).Error == nil && existing.ID != user.ID {
		c.JSON(http.StatusConflict, shared.Fail("WALLET_TAKEN", "this wallet is already bound to another account"))
		return
	}

	if !strings.Contains(body.Message, user.Nonce) {
		c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_NONCE", "message nonce does not match — call /auth/wallet/nonce first"))
		return
	}

	recovered, err := recoverSigner(body.Message, body.Signature)
	if err != nil || !strings.EqualFold(recovered, wallet) {
		c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_SIGNATURE", "signature does not match wallet address"))
		return
	}

	// Rotate nonce and bind wallet.
	h.db.Model(user).Updates(map[string]any{
		"wallet_address": wallet,
		"nonce":          newNonce(),
	})

	ok(c, gin.H{
		"wallet_address": wallet,
		"message":        "Wallet successfully bound to your account.",
	})
}

// ─── Captcha + API Key (unchanged) ───────────────────────────────────────────

// GetCaptcha issues a one-time math challenge for Agent API key registration.
// GET /api/v1/auth/captcha
func (h *AuthHandler) GetCaptcha(c *gin.Context) {
	token, question := middleware.Captcha.Issue()
	ok(c, gin.H{
		"captcha_token":   token,
		"question":        question,
		"expires_in_secs": 600,
		"note":            "Solve the math question and include captcha_token + captcha_answer when calling POST /auth/apikey.",
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
		CaptchaToken  string `json:"captcha_token"  binding:"required"`
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

	if err := h.db.Model(user).Updates(map[string]any{
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

// RotateAPIKey generates a new API key, invalidating the previous one.
// The caller must already be authenticated (JWT or existing API Key).
// POST /api/v1/auth/apikey/rotate  (requires JWT auth)
func (h *AuthHandler) RotateAPIKey(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}
	if !user.IsAgent {
		c.JSON(http.StatusForbidden, shared.Fail("NOT_AGENT", "only agent accounts can rotate API keys — generate one first via POST /auth/apikey"))
		return
	}

	plainKey := newAPIKey()
	hash := middleware.HashAPIKey(plainKey)

	if err := h.db.Model(user).Update("api_key_hash", hash).Error; err != nil {
		serverError(c, err)
		return
	}

	ok(c, gin.H{
		"api_key": plainKey,
		"note":    "Previous key is now invalid. Store this new key safely — it will not be shown again.",
	})
}

// RevokeAPIKey removes the agent's API key and resets is_agent to false.
// DELETE /api/v1/auth/apikey  (requires JWT auth)
func (h *AuthHandler) RevokeAPIKey(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}
	if !user.IsAgent {
		c.JSON(http.StatusForbidden, shared.Fail("NOT_AGENT", "no API key to revoke — account is not an agent"))
		return
	}

	if err := h.db.Model(user).Updates(map[string]any{
		"api_key_hash": nil,
		"is_agent":     false,
	}).Error; err != nil {
		serverError(c, err)
		return
	}

	ok(c, gin.H{
		"message": "API key revoked. Agent access has been disabled. You can generate a new key via POST /auth/apikey.",
	})
}

// ─── Agent registration (one-shot, no JWT required) ─────────────────────────

// RegisterAgentChallenge issues a short-lived stateless challenge for wallet-based
// agent registration. The client signs the returned `message` with their wallet
// private key, then submits {wallet, challenge, signature} to POST /auth/register-agent.
//
// GET /api/v1/auth/register-agent/nonce?wallet=0x...
func (h *AuthHandler) RegisterAgentChallenge(c *gin.Context) {
	wallet := strings.ToLower(strings.TrimSpace(c.Query("wallet")))
	if !isValidEVMAddress(wallet) {
		badRequest(c, "wallet query param must be a 0x-prefixed 40-hex-char EVM address")
		return
	}

	// Reject early if the wallet is already bound to any account.
	var existing models.User
	if h.db.Where("wallet_address = ?", wallet).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, shared.Fail("WALLET_TAKEN", "this wallet is already registered — nothing to do"))
		return
	}

	nonce := newNonce()
	msg := siweRegisterAgentMessage(wallet, nonce)
	challenge, err := issueAgentChallenge(wallet, nonce)
	if err != nil {
		serverError(c, err)
		return
	}

	ok(c, gin.H{
		"challenge":       challenge,
		"message":         msg,
		"expires_in_secs": int(agentChallengeTTL.Seconds()),
		"instructions":    "Sign `message` with your wallet (EIP-191 personal_sign), then POST {wallet, challenge, signature} to /auth/register-agent.",
	})
}

// RegisterAgent creates a new agent account in one shot. Two auth paths are
// supported — clients pick ONE:
//
//   - **Wallet (recommended)**: {wallet, challenge, signature}
//     The challenge is the short-lived JWT returned by GET
//     /auth/register-agent/nonce. Signature proves control of the wallet's
//     private key (EIP-191 personal_sign).
//
//   - **Email/password (fallback)**: {email, password}
//     For agent developers who want email recovery. Auto-verified (no email
//     confirmation required — the registration itself is a deliberate action).
//     Username may be provided; otherwise derived from email.
//
// Response returns the plain-text `api_key` ONE TIME. Store it safely.
//
// POST /api/v1/auth/register-agent
func (h *AuthHandler) RegisterAgent(c *gin.Context) {
	var body struct {
		// Wallet path
		Wallet    string `json:"wallet"`
		Challenge string `json:"challenge"`
		Signature string `json:"signature"`
		// Email path
		Email    string `json:"email"`
		Password string `json:"password"`
		// Optional (both paths)
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	var user models.User
	switch {
	case body.Wallet != "":
		// ─── Wallet path ───────────────────────────────────────────────────
		if body.Challenge == "" || body.Signature == "" {
			badRequest(c, "wallet path requires {wallet, challenge, signature}")
			return
		}
		wallet := strings.ToLower(strings.TrimSpace(body.Wallet))
		if !isValidEVMAddress(wallet) {
			badRequest(c, "wallet must be a 0x-prefixed 40-hex-char EVM address")
			return
		}

		claims, err := verifyAgentChallenge(body.Challenge)
		if err != nil {
			c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_CHALLENGE", "challenge expired or invalid — request a new one via GET /auth/register-agent/nonce"))
			return
		}
		if !strings.EqualFold(claims.Wallet, wallet) {
			c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_CHALLENGE", "challenge wallet does not match submitted wallet"))
			return
		}

		expectedMsg := siweRegisterAgentMessage(wallet, claims.Nonce)
		recovered, err := recoverSigner(expectedMsg, body.Signature)
		if err != nil || !strings.EqualFold(recovered, wallet) {
			c.JSON(http.StatusUnauthorized, shared.Fail("INVALID_SIGNATURE", "signature does not match wallet"))
			return
		}

		// Double-check wallet not taken (race since challenge was issued).
		var existing models.User
		if h.db.Where("wallet_address = ?", wallet).First(&existing).Error == nil {
			c.JSON(http.StatusConflict, shared.Fail("WALLET_TAKEN", "this wallet is already registered"))
			return
		}

		username := body.Username
		if username == "" {
			username = deriveAgentUsername(h.db, wallet)
		}
		displayName := body.DisplayName
		if displayName == "" {
			displayName = fmt.Sprintf("Agent %s", wallet[:10])
		}

		user = models.User{
			ID:            newUserID(),
			Username:      username,
			DisplayName:   displayName,
			WalletAddress: &wallet,
			IsAgent:       true,
			EmailVerified: true, // no email to verify
			Nonce:         newNonce(),
		}

	case body.Email != "":
		// ─── Email path ────────────────────────────────────────────────────
		email := strings.ToLower(strings.TrimSpace(body.Email))
		if !strings.Contains(email, "@") {
			badRequest(c, "email is not valid")
			return
		}
		if len(body.Password) < 8 {
			badRequest(c, "password must be at least 8 characters")
			return
		}

		var existing models.User
		if h.db.Where("email = ?", email).First(&existing).Error == nil {
			c.JSON(http.StatusConflict, shared.Fail("EMAIL_TAKEN", "email already registered"))
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			serverError(c, err)
			return
		}

		username := body.Username
		if username == "" {
			username = deriveAgentUsername(h.db, email)
		}
		displayName := body.DisplayName
		if displayName == "" {
			displayName = username
		}

		user = models.User{
			ID:            newUserID(),
			Username:      username,
			DisplayName:   displayName,
			Email:         &email,
			PasswordHash:  string(hash),
			EmailVerified: true, // agent path skips email confirmation
			IsAgent:       true,
			Nonce:         newNonce(),
		}

	default:
		badRequest(c, "provide either {wallet, challenge, signature} or {email, password}")
		return
	}

	// Generate the API key and hash it for storage.
	plainKey := newAPIKey()
	hash := middleware.HashAPIKey(plainKey)
	user.APIKeyHash = &hash

	if err := h.db.Create(&user).Error; err != nil {
		// Typical failure: duplicate username collision.
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, shared.Fail("USERNAME_TAKEN", "could not generate a unique username — retry or pass `username` explicitly"))
			return
		}
		serverError(c, err)
		return
	}

	ok(c, gin.H{
		"api_key": plainKey,
		"user":    user.ToPublic(),
		"note":    "Store the api_key safely — it will not be shown again. Use it via `X-API-Key` header for all SKILL API requests.",
	})
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// recoverSigner recovers the Ethereum address that signed the given message
// using EIP-191 personal_sign format (MetaMask / wagmi compatible).
func recoverSigner(message, hexSig string) (string, error) {
	sigBytes, err := hex.DecodeString(strings.TrimPrefix(hexSig, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid signature hex: %w", err)
	}
	if len(sigBytes) != 65 {
		return "", fmt.Errorf("signature must be 65 bytes, got %d", len(sigBytes))
	}

	prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := keccak256([]byte(prefixed))

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

	uncompressed := pubKey.SerializeUncompressed()
	addrHash := keccak256(uncompressed[1:])
	addr := fmt.Sprintf("0x%s", hex.EncodeToString(addrHash[12:]))
	return strings.ToLower(addr), nil
}

func keccak256(b []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(b)
	return h.Sum(nil)
}

func siweMessage(wallet, nonce string) string {
	return fmt.Sprintf(
		"ClawLink wants you to sign in with your Ethereum account:\n%s\n\n"+
			"Bind your wallet to ClawLink\n\n"+
			"Nonce: %s\n"+
			"Chain ID: 11111110",
		wallet, nonce,
	)
}

func oauthCallbackURL(cfg *config.Config, provider string) string {
	return fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", strings.TrimRight(cfg.APIBaseURL, "/"), provider)
}

func validateOAuthProviderConfig(provider string, cfg *config.Config) error {
	switch provider {
	case "google":
		if strings.TrimSpace(cfg.GoogleClientID) == "" {
			return fmt.Errorf("google oauth is not configured: GOOGLE_CLIENT_ID is empty")
		}
		if strings.TrimSpace(cfg.GoogleClientSecret) == "" {
			return fmt.Errorf("google oauth is not configured: GOOGLE_CLIENT_SECRET is empty")
		}
	case "discord":
		if strings.TrimSpace(cfg.DiscordClientID) == "" {
			return fmt.Errorf("discord oauth is not configured: DISCORD_CLIENT_ID is empty")
		}
		if strings.TrimSpace(cfg.DiscordClientSecret) == "" {
			return fmt.Errorf("discord oauth is not configured: DISCORD_CLIENT_SECRET is empty")
		}
	}
	return nil
}

// sendVerificationEmail sends a verification link to the given email.
// In dev mode (SMTP_HOST empty) it only logs the link.
func sendVerificationEmail(email, token string) {
	cfg := config.App
	link := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", strings.TrimRight(cfg.APIBaseURL, "/"), url.QueryEscape(token))

	if cfg.SMTPHost == "" {
		log.Printf("[auth] email verification link for %s: %s", email, link)
		return
	}

	go func() {
		body := fmt.Sprintf(
			"From: ClawLink <%s>\r\nTo: %s\r\nSubject: Verify your ClawLink account\r\n\r\n"+
				"Welcome to ClawLink!\r\n\r\nVerify your email by clicking the link below:\r\n%s\r\n\r\n"+
				"This link expires in 24 hours. If you did not register, ignore this email.",
			cfg.SMTPFrom, email, link,
		)
		addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
		if err := sendSMTPMail(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom, []string{email}, []byte(body)); err != nil {
			log.Printf("[auth] failed to send verification email to %s: %v", email, err)
		} else {
			log.Printf("[auth] sent verification email to %s via %s", email, addr)
		}
	}()
}

// sendSMTPMail performs an explicit SMTP session with STARTTLS + AUTH + DATA.
// This is more reliable than smtp.SendMail for modern 587 submission servers
// that require TLS negotiation before authentication.
func sendSMTPMail(host string, port int, username, password, from string, to []string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer c.Close()

	if err := c.Hello("clawlink.app"); err != nil {
		return fmt.Errorf("smtp hello: %w", err)
	}

	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName:         host,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if username != "" || password != "" {
		auth := smtp.PlainAuth("", username, password, host)
		if ok, _ := c.Extension("AUTH"); !ok {
			return fmt.Errorf("smtp server does not advertise AUTH")
		}
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp RCPT TO %s: %w", rcpt, err)
		}
	}

	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		_ = wc.Close()
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp finalize body: %w", err)
	}

	if err := c.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

// newToken returns a 32-byte random hex string for verification tokens / OAuth state.
func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ─── OAuth provider HTTP helpers ─────────────────────────────────────────────

func exchangeGoogleCode(code string, cfg *config.Config) (string, error) {
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"code":          {code},
		"client_id":     {cfg.GoogleClientID},
		"client_secret": {cfg.GoogleClientSecret},
		"redirect_uri":  {oauthCallbackURL(cfg, "google")},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return extractAccessToken(resp.Body)
}

func fetchGoogleUserInfo(accessToken string) (map[string]any, error) {
	return fetchJSON("https://www.googleapis.com/oauth2/v3/userinfo", accessToken)
}

func exchangeDiscordCode(code string, cfg *config.Config) (string, error) {
	resp, err := http.PostForm("https://discord.com/api/oauth2/token", url.Values{
		"code":          {code},
		"client_id":     {cfg.DiscordClientID},
		"client_secret": {cfg.DiscordClientSecret},
		"redirect_uri":  {oauthCallbackURL(cfg, "discord")},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return extractAccessToken(resp.Body)
}

func fetchDiscordUserInfo(accessToken string) (map[string]any, error) {
	return fetchJSON("https://discord.com/api/users/@me", accessToken)
}

func extractAccessToken(r io.Reader) (string, error) {
	var result map[string]any
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return "", err
	}
	token, ok := result["access_token"].(string)
	if !ok || token == "" {
		// Surface the exact OAuth error from the provider instead of dumping
		// the raw map. Both Google and Discord return RFC 6749 error fields:
		//   {"error": "...", "error_description": "..."}
		errCode, _ := result["error"].(string)
		errDesc, _ := result["error_description"].(string)
		switch errCode {
		case "invalid_client":
			return "", fmt.Errorf("OAuth invalid_client — check DISCORD_CLIENT_ID/DISCORD_CLIENT_SECRET (or GOOGLE_*) in .env match the provider console. Provider said: %q", errDesc)
		case "invalid_grant":
			return "", fmt.Errorf("OAuth invalid_grant — code expired or redirect_uri mismatch. Provider said: %q", errDesc)
		case "redirect_uri_mismatch":
			return "", fmt.Errorf("OAuth redirect_uri_mismatch — the exact callback URL must be whitelisted in the provider console. Provider said: %q", errDesc)
		case "":
			return "", fmt.Errorf("no access_token in response: %v", result)
		default:
			return "", fmt.Errorf("OAuth %s: %s", errCode, errDesc)
		}
	}
	return token, nil
}

func fetchJSON(endpoint, accessToken string) (map[string]any, error) {
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]any
	return result, json.NewDecoder(resp.Body).Decode(&result)
}

// ─── Agent registration helpers ──────────────────────────────────────────────

const agentChallengeTTL = 10 * time.Minute

// agentChallengeClaims is the payload of the short-lived challenge JWT.
type agentChallengeClaims struct {
	Wallet string `json:"wallet"`
	Nonce  string `json:"nonce"`
	jwt.RegisteredClaims
}

// issueAgentChallenge returns a signed JWT that binds (wallet, nonce) with a
// short expiry. Stateless — no DB write needed.
func issueAgentChallenge(wallet, nonce string) (string, error) {
	claims := &agentChallengeClaims{
		Wallet: wallet,
		Nonce:  nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(agentChallengeTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "register-agent",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.App.JWTSecret))
}

// verifyAgentChallenge parses and validates a challenge JWT, returning the
// wallet and nonce it was issued for.
func verifyAgentChallenge(tokenStr string) (*agentChallengeClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &agentChallengeClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(config.App.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*agentChallengeClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid challenge token")
	}
	if claims.Subject != "register-agent" {
		return nil, fmt.Errorf("token is not an agent-registration challenge")
	}
	return claims, nil
}

// siweRegisterAgentMessage is distinct from siweMessage so a signature obtained
// for wallet-binding cannot be replayed against registration.
func siweRegisterAgentMessage(wallet, nonce string) string {
	return fmt.Sprintf(
		"ClawLink wants you to register a new agent account:\n%s\n\n"+
			"Register as ClawLink Agent\n\n"+
			"Nonce: %s\n"+
			"Chain ID: 11111110",
		wallet, nonce,
	)
}

// isValidEVMAddress does a format-only check (0x + 40 hex chars).
func isValidEVMAddress(addr string) bool {
	if len(addr) != 42 || !strings.HasPrefix(addr, "0x") {
		return false
	}
	_, err := hex.DecodeString(addr[2:])
	return err == nil
}

// newUserID returns a random 18-byte hex ID matching the existing user ID
// convention (see the example IDs like "8a224cb4e540bf72f24bb2ac3cee98e6809f").
func newUserID() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// deriveAgentUsername generates a unique agent_* username from the wallet
// address or email. If the first candidate is taken, it tries up to 5 suffixes.
func deriveAgentUsername(db *gorm.DB, seed string) string {
	base := "agent_"
	switch {
	case strings.HasPrefix(seed, "0x") && len(seed) >= 10:
		base += seed[2:10] // first 8 hex chars
	case strings.Contains(seed, "@"):
		local := strings.SplitN(seed, "@", 2)[0]
		// Sanitize: keep alphanumeric + underscore
		clean := make([]byte, 0, len(local))
		for i := 0; i < len(local) && i < 32; i++ {
			ch := local[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
				clean = append(clean, ch)
			}
		}
		if len(clean) == 0 {
			base += randHex(4)
		} else {
			base = "agent_" + strings.ToLower(string(clean))
		}
	default:
		base += randHex(4)
	}

	candidate := base
	for i := 0; i < 5; i++ {
		var count int64
		db.Model(&models.User{}).Where("username = ?", candidate).Count(&count)
		if count == 0 {
			return candidate
		}
		candidate = fmt.Sprintf("%s_%s", base, randHex(2))
	}
	// Last resort: always-unique timestamp suffix.
	return fmt.Sprintf("%s_%d", base, time.Now().UnixNano())
}

// randHex returns a random hex string of the given byte length.
func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
