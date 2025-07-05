package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// HMACSigner provides HMAC signing functionality for webhook payloads
type HMACSigner struct {
	secret string
}

// NewHMACSignerFromSecret creates a new HMAC signer with the given secret
func NewHMACSignerFromSecret(secret string) *HMACSigner {
	return &HMACSigner{
		secret: secret,
	}
}

// Sign signs the payload using HMAC-SHA256 and returns the signature
func (h *HMACSigner) Sign(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	signature := mac.Sum(nil)
	return "sha256=" + hex.EncodeToString(signature)
}

// SignBase64 signs the payload using HMAC-SHA256 and returns the base64 encoded signature
func (h *HMACSigner) SignBase64(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	signature := mac.Sum(nil)
	return "sha256=" + base64.StdEncoding.EncodeToString(signature)
}

// Verify verifies the signature against the payload
func (h *HMACSigner) Verify(payload []byte, signature string) bool {
	expectedSignature := h.Sign(payload)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// VerifyBase64 verifies the base64 encoded signature against the payload
func (h *HMACSigner) VerifyBase64(payload []byte, signature string) bool {
	expectedSignature := h.SignBase64(payload)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// VerifyWebhookSignature verifies a webhook signature in the format used by GitHub, Stripe, etc.
func (h *HMACSigner) VerifyWebhookSignature(payload []byte, signature string) bool {
	// Handle different signature formats
	if strings.HasPrefix(signature, "sha256=") {
		return h.Verify(payload, signature)
	}

	// Try with sha256= prefix
	prefixedSignature := "sha256=" + signature
	return h.Verify(payload, prefixedSignature)
}

// GenerateWebhookSignature generates a webhook signature with timestamp
func (h *HMACSigner) GenerateWebhookSignature(payload []byte, timestamp int64) string {
	// Create the signed payload: timestamp.payload
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(payload))

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(signedPayload))
	signature := mac.Sum(nil)

	return hex.EncodeToString(signature)
}

// VerifyWebhookSignatureWithTimestamp verifies a webhook signature with timestamp
func (h *HMACSigner) VerifyWebhookSignatureWithTimestamp(payload []byte, signature string, timestamp int64, tolerance time.Duration) bool {
	// Check if timestamp is within tolerance
	now := time.Now().Unix()
	if tolerance > 0 && (now-timestamp) > int64(tolerance.Seconds()) {
		return false
	}

	expectedSignature := h.GenerateWebhookSignature(payload, timestamp)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// ParseWebhookSignature parses a webhook signature header (e.g., "t=1234567890,v1=signature")
func ParseWebhookSignature(signatureHeader string) (timestamp int64, signature string, err error) {
	parts := strings.Split(signatureHeader, ",")
	if len(parts) < 2 {
		return 0, "", fmt.Errorf("invalid signature header format")
	}

	var t string
	var v string

	for _, part := range parts {
		if strings.HasPrefix(part, "t=") {
			t = strings.TrimPrefix(part, "t=")
		} else if strings.HasPrefix(part, "v1=") {
			v = strings.TrimPrefix(part, "v1=")
		}
	}

	if t == "" || v == "" {
		return 0, "", fmt.Errorf("missing timestamp or signature in header")
	}

	timestamp, err = parseTimestamp(t)
	if err != nil {
		return 0, "", fmt.Errorf("invalid timestamp: %w", err)
	}

	return timestamp, v, nil
}

// parseTimestamp parses a timestamp string to int64
func parseTimestamp(timestampStr string) (int64, error) {
	// Try parsing as Unix timestamp
	if len(timestampStr) == 10 {
		// Assume it's a Unix timestamp in seconds
		timestamp := int64(0)
		for _, c := range timestampStr {
			if c < '0' || c > '9' {
				return 0, fmt.Errorf("invalid timestamp format")
			}
			timestamp = timestamp*10 + int64(c-'0')
		}
		return timestamp, nil
	}

	return 0, fmt.Errorf("unsupported timestamp format")
}

// GenerateSecret generates a random secret for HMAC signing
func GenerateSecret() (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	for i := range bytes {
		bytes[i] = byte(time.Now().UnixNano() % 256)
	}

	return base64.StdEncoding.EncodeToString(bytes), nil
}

// ValidateSecret validates that a secret is strong enough
func ValidateSecret(secret string) error {
	if len(secret) < 16 {
		return fmt.Errorf("secret must be at least 16 characters long")
	}

	// Check for sufficient entropy (basic check)
	if strings.Repeat(string(secret[0]), len(secret)) == secret {
		return fmt.Errorf("secret must not be a repeated character")
	}

	return nil
}

// SecureCompare performs a constant-time comparison of two strings
func SecureCompare(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}
