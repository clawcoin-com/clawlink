package handlers

import (
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

	// Redirect frontend to the callback page with the JWT.
	frontendURL := config.App.FrontendURL
	redirectURL := fmt.Sprintf("%s/auth/callback?token=%s", frontendURL, url.QueryEscape(jwt))
	c.Redirect(http.StatusFound, redirectURL)
}

// ─── OAuth ────────────────────────────────────────────────────────────────────

// oauthState is an in-memory CSRF state store (mirrors captcha pattern).
var oauthState = struct {
	sync.Mutex
	m map[string]time.Time
}{m: make(map[string]time.Time)}

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
		oauthID string
		email   string
		name    string
		avatar  string
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
		oauthID = info["sub"].(string)
		if e, ok := info["email"].(string); ok {
			email = strings.ToLower(e)
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
		oauthID = info["id"].(string)
		if e, ok := info["email"].(string); ok {
			email = strings.ToLower(e)
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
	user, err := h.findOrCreateOAuthUser(provider, oauthID, email, name, avatar)
	if err != nil {
		serverError(c, err)
		return
	}

	token, err := middleware.MakeJWT(user)
	if err != nil {
		serverError(c, err)
		return
	}

	redirectURL := fmt.Sprintf("%s/auth/callback?token=%s", cfg.FrontendURL, url.QueryEscape(token))
	c.Redirect(http.StatusFound, redirectURL)
}

func (h *AuthHandler) findOrCreateOAuthUser(provider, oauthID, email, name, avatar string) (*models.User, error) {
	// 1. Try to find by (provider, oauth_id).
	var user models.User
	if h.db.Where("oauth_provider = ? AND oauth_id = ?", provider, oauthID).First(&user).Error == nil {
		return &user, nil
	}

	// 2. Try to find by email (link existing account).
	if email != "" {
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
	if email != "" {
		emailPtr = &email
	}
	user = models.User{
		ID:            newID(),
		Username:      randomUsername(),
		DisplayName:   displayName,
		Avatar:        avatar,
		Email:         emailPtr,
		EmailVerified: true, // OAuth emails are pre-verified by the provider
		OAuthProvider: provider,
		OAuthID:       oauthID,
		Metadata:      shared.JSON("{}"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	return &user, h.db.Create(&user).Error
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
		auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
		if err := smtp.SendMail(addr, auth, cfg.SMTPFrom, []string{email}, []byte(body)); err != nil {
			log.Printf("[auth] failed to send verification email to %s: %v", email, err)
		}
	}()
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
		return "", fmt.Errorf("no access_token in response: %v", result)
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
