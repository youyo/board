package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	idproxy "github.com/youyo/idproxy"
	idpstore "github.com/youyo/idproxy/store"
)

// mcpAuthFlags は `board mcp serve --auth` 関連フラグを保持する。
//
// すべてのフラグは環境変数 BOARD_MCP_* でも設定でき、
// 明示フラグ未指定時に環境変数値が採用される（loadFromEnv 参照）。
type mcpAuthFlags struct {
	Auth                       bool
	ExternalURL                string
	OIDCIssuer                 string
	OIDCClientID               string
	OIDCClientSecret           string
	CookieSecret               string
	AllowedDomains             string
	AllowedEmails              string
	SigningKey                 string
	RefreshTokenTTL            time.Duration
	IDProxyStore               string
	IDProxyStoreDynamoDBTable  string
	IDProxyStoreDynamoDBRegion string
}

// validate は --auth 有効時の必須フィールドおよび idproxy store 設定を検証する。
func (f *mcpAuthFlags) validate() error {
	switch strings.ToLower(f.IDProxyStore) {
	case "", "memory":
		// OK
	case "dynamodb":
		if f.IDProxyStoreDynamoDBTable == "" {
			return fmt.Errorf("--idproxy-store-dynamodb-table is required when --idproxy-store=dynamodb")
		}
		if f.SigningKey == "" {
			return fmt.Errorf("--signing-key is required when --idproxy-store=dynamodb " +
				"(random signing key cannot be shared across Lambda containers)")
		}
	default:
		return fmt.Errorf("invalid --idproxy-store: %q (must be memory or dynamodb)", f.IDProxyStore)
	}

	if !f.Auth {
		return nil
	}
	if f.ExternalURL == "" {
		return fmt.Errorf("--external-url is required when --auth is enabled")
	}
	if f.OIDCIssuer == "" {
		return fmt.Errorf("--oidc-issuer is required when --auth is enabled")
	}
	if f.OIDCClientID == "" {
		return fmt.Errorf("--oidc-client-id is required when --auth is enabled")
	}
	if f.CookieSecret == "" {
		return fmt.Errorf("--cookie-secret is required when --auth is enabled")
	}
	secret, err := hex.DecodeString(f.CookieSecret)
	if err != nil {
		return fmt.Errorf("--cookie-secret: invalid hex: %w", err)
	}
	if len(secret) < 32 {
		return fmt.Errorf("--cookie-secret: must be at least 32 bytes (64 hex chars), got %d bytes", len(secret))
	}
	return nil
}

// parseSigningKey は PEM 形式の ECDSA P-256 秘密鍵をパースする。
// 空文字列の場合は新規ランダム鍵を生成する（single-instance 運用向け）。
func parseSigningKey(pemStr string) (*ecdsa.PrivateKey, error) {
	if pemStr == "" {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate signing key: %w", err)
		}
		return key, nil
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("signing-key: invalid PEM")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("signing-key: %w", err)
	}
	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("signing-key: curve must be P-256, got %s", key.Curve.Params().Name)
	}
	return key, nil
}

// buildIDProxyStore は flags から idproxy.Store を構築する。
// 呼び出し元は利用後に Close() を呼ぶ責任がある。
func buildIDProxyStore(f *mcpAuthFlags) (idproxy.Store, error) {
	switch strings.ToLower(f.IDProxyStore) {
	case "", "memory":
		return idpstore.NewMemoryStore(), nil
	case "dynamodb":
		s, err := idpstore.NewDynamoDBStore(f.IDProxyStoreDynamoDBTable, f.IDProxyStoreDynamoDBRegion)
		if err != nil {
			return nil, fmt.Errorf("failed to create idproxy dynamodb store: %w", err)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("invalid idproxy-store: %q", f.IDProxyStore)
	}
}

// buildAuthConfig は mcpAuthFlags から idproxy.Config を構築する。
// 返される idproxy.Config.Store は利用後に Close() を呼ぶ必要がある。
func buildAuthConfig(f *mcpAuthFlags) (idproxy.Config, error) {
	cookieSecret, err := hex.DecodeString(f.CookieSecret)
	if err != nil {
		return idproxy.Config{}, fmt.Errorf("cookie-secret: invalid hex: %w", err)
	}
	if len(cookieSecret) < 32 {
		return idproxy.Config{}, fmt.Errorf("cookie-secret: must be at least 32 bytes (64 hex chars), got %d bytes", len(cookieSecret))
	}

	signingKey, err := parseSigningKey(f.SigningKey)
	if err != nil {
		return idproxy.Config{}, err
	}

	store, err := buildIDProxyStore(f)
	if err != nil {
		return idproxy.Config{}, err
	}

	splitCSV := func(v string) []string {
		if v == "" {
			return nil
		}
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}

	return idproxy.Config{
		Providers: []idproxy.OIDCProvider{
			{
				Issuer:       f.OIDCIssuer,
				ClientID:     f.OIDCClientID,
				ClientSecret: f.OIDCClientSecret,
			},
		},
		AllowedDomains:  splitCSV(f.AllowedDomains),
		AllowedEmails:   splitCSV(f.AllowedEmails),
		ExternalURL:     f.ExternalURL,
		CookieSecret:    cookieSecret,
		Store:           store,
		RefreshTokenTTL: f.RefreshTokenTTL,
		OAuth: &idproxy.OAuthConfig{
			SigningKey: signingKey,
		},
	}, nil
}
