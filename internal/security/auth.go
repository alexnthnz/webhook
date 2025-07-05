package security

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// Claims represents JWT claims for webhook service
type Claims struct {
	CustomerID string   `json:"customer_id"`
	Scopes     []string `json:"scopes"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token creation and validation
type JWTManager struct {
	secret     string
	issuer     string
	audience   string
	expiration time.Duration
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secret, issuer, audience string, expiration time.Duration) *JWTManager {
	return &JWTManager{
		secret:     secret,
		issuer:     issuer,
		audience:   audience,
		expiration: expiration,
	}
}

// GenerateToken generates a JWT token for the given customer
func (j *JWTManager) GenerateToken(customerID string, scopes []string) (string, error) {
	now := time.Now()
	claims := &Claims{
		CustomerID: customerID,
		Scopes:     scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Audience:  jwt.ClaimStrings{j.audience},
			Subject:   customerID,
			ExpiresAt: jwt.NewNumericDate(now.Add(j.expiration)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secret))
}

// ValidateToken validates a JWT token and returns the claims
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Additional validation
	if claims.Issuer != j.issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	// Check if audience matches
	validAudience := false
	for _, aud := range claims.Audience {
		if aud == j.audience {
			validAudience = true
			break
		}
	}
	if !validAudience {
		return nil, fmt.Errorf("invalid audience")
	}

	return claims, nil
}

// ExtractTokenFromHeader extracts JWT token from Authorization header
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is required")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return parts[1], nil
}

// OAuth2Config holds OAuth2 configuration
type OAuth2Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	AuthURL      string
	TokenURL     string
}

// OAuth2Manager handles OAuth2 authentication
type OAuth2Manager struct {
	config *oauth2.Config
}

// NewOAuth2Manager creates a new OAuth2 manager
func NewOAuth2Manager(cfg OAuth2Config) *OAuth2Manager {
	return &OAuth2Manager{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.AuthURL,
				TokenURL: cfg.TokenURL,
			},
		},
	}
}

// GetAuthURL returns the OAuth2 authorization URL
func (o *OAuth2Manager) GetAuthURL(state string) string {
	return o.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCode exchanges authorization code for access token
func (o *OAuth2Manager) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return o.config.Exchange(ctx, code)
}

// RefreshToken refreshes an OAuth2 token
func (o *OAuth2Manager) RefreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error) {
	tokenSource := o.config.TokenSource(ctx, token)
	return tokenSource.Token()
}

// ValidateToken validates an OAuth2 token
func (o *OAuth2Manager) ValidateToken(ctx context.Context, token *oauth2.Token) error {
	if !token.Valid() {
		return fmt.Errorf("token is expired or invalid")
	}
	return nil
}

// AuthMiddleware provides authentication middleware for HTTP handlers
type AuthMiddleware struct {
	jwtManager    *JWTManager
	oauth2Manager *OAuth2Manager
	skipPaths     []string
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(jwtManager *JWTManager, oauth2Manager *OAuth2Manager, skipPaths []string) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager:    jwtManager,
		oauth2Manager: oauth2Manager,
		skipPaths:     skipPaths,
	}
}

// Middleware returns the HTTP middleware function
func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if path should be skipped
		for _, path := range a.skipPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Extract token from header
		authHeader := r.Header.Get("Authorization")
		token, err := ExtractTokenFromHeader(authHeader)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Validate JWT token
		claims, err := a.jwtManager.ValidateToken(token)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), "claims", claims)
		ctx = context.WithValue(ctx, "customer_id", claims.CustomerID)
		ctx = context.WithValue(ctx, "scopes", claims.Scopes)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetClaimsFromContext extracts claims from request context
func GetClaimsFromContext(ctx context.Context) (*Claims, error) {
	claims, ok := ctx.Value("claims").(*Claims)
	if !ok {
		return nil, fmt.Errorf("no claims found in context")
	}
	return claims, nil
}

// GetCustomerIDFromContext extracts customer ID from request context
func GetCustomerIDFromContext(ctx context.Context) (string, error) {
	customerID, ok := ctx.Value("customer_id").(string)
	if !ok {
		return "", fmt.Errorf("no customer ID found in context")
	}
	return customerID, nil
}

// GetScopesFromContext extracts scopes from request context
func GetScopesFromContext(ctx context.Context) ([]string, error) {
	scopes, ok := ctx.Value("scopes").([]string)
	if !ok {
		return nil, fmt.Errorf("no scopes found in context")
	}
	return scopes, nil
}

// HasScope checks if the request context has a specific scope
func HasScope(ctx context.Context, requiredScope string) bool {
	scopes, err := GetScopesFromContext(ctx)
	if err != nil {
		return false
	}

	for _, scope := range scopes {
		if scope == requiredScope {
			return true
		}
	}
	return false
}

// RequireScope middleware that requires a specific scope
func RequireScope(requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !HasScope(r.Context(), requiredScope) {
				http.Error(w, "Forbidden: insufficient scope", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// APIKeyManager handles API key authentication
type APIKeyManager struct {
	keys map[string]string // API key -> customer ID
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		keys: make(map[string]string),
	}
}

// AddAPIKey adds an API key for a customer
func (a *APIKeyManager) AddAPIKey(apiKey, customerID string) {
	a.keys[apiKey] = customerID
}

// ValidateAPIKey validates an API key and returns the customer ID
func (a *APIKeyManager) ValidateAPIKey(apiKey string) (string, error) {
	customerID, exists := a.keys[apiKey]
	if !exists {
		return "", fmt.Errorf("invalid API key")
	}
	return customerID, nil
}

// APIKeyMiddleware provides API key authentication middleware
func (a *APIKeyManager) APIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "API key is required", http.StatusUnauthorized)
			return
		}

		customerID, err := a.ValidateAPIKey(apiKey)
		if err != nil {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// Add customer ID to request context
		ctx := context.WithValue(r.Context(), "customer_id", customerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request is allowed for the given key
func (r *RateLimiter) Allow(key string) bool {
	now := time.Now()

	// Get existing requests for this key
	requests, exists := r.requests[key]
	if !exists {
		r.requests[key] = []time.Time{now}
		return true
	}

	// Remove old requests outside the window
	var validRequests []time.Time
	for _, req := range requests {
		if now.Sub(req) < r.window {
			validRequests = append(validRequests, req)
		}
	}

	// Check if we're under the limit
	if len(validRequests) >= r.limit {
		return false
	}

	// Add current request
	validRequests = append(validRequests, now)
	r.requests[key] = validRequests

	return true
}

// RateLimitMiddleware provides rate limiting middleware
func (rl *RateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use IP address as the key (in production, you might want to use customer ID)
		key := r.RemoteAddr

		if !rl.Allow(key) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
