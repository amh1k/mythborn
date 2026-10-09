package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
)

type SupabaseAuthenticator struct {
	db         contracts.Database
	issuer     string
	jwksURL    string
	client     *http.Client
	cacheTTL   time.Duration
	mu         sync.RWMutex
	keys       map[string]jwkKey
	keysExpire time.Time
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Subject   string          `json:"sub"`
	Issuer    string          `json:"iss"`
	Audience  json.RawMessage `json:"aud"`
	ExpiresAt int64           `json:"exp"`
	NotBefore int64           `json:"nbf"`
}

func NewSupabaseAuthenticator(db contracts.Database, projectURL string) (*SupabaseAuthenticator, error) {
	projectURL = strings.TrimRight(strings.TrimSpace(projectURL), "/")
	if db == nil || projectURL == "" {
		return nil, fmt.Errorf("database and SUPABASE_URL are required for authentication")
	}
	issuer := projectURL + "/auth/v1"
	return &SupabaseAuthenticator{
		db: db, issuer: issuer,
		jwksURL: issuer + "/.well-known/jwks.json",
		client:  &http.Client{Timeout: 5 * time.Second}, cacheTTL: 10 * time.Minute,
		keys: make(map[string]jwkKey),
	}, nil
}

func (a *SupabaseAuthenticator) Authenticate(ctx context.Context, raw string) (Principal, error) {
	header, claims, signingInput, sig, err := parseJWT(raw)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}
	key, err := a.signingKey(ctx, header.Kid)
	if err != nil || !verifySignature(header.Alg, key, signingInput, sig) {
		return Principal{}, ErrUnauthorized
	}
	if claims.Issuer != a.issuer || claims.Subject == "" || !validAudience(claims.Audience, "authenticated") {
		return Principal{}, ErrUnauthorized
	}
	now := time.Now()
	if claims.ExpiresAt == 0 || !now.Before(time.Unix(claims.ExpiresAt, 0)) || (claims.NotBefore != 0 && now.Add(30*time.Second).Before(time.Unix(claims.NotBefore, 0))) {
		return Principal{}, ErrUnauthorized
	}

	// Provision a default user row on first authenticated use. Public tokens
	// never supply or alter the account-level system role.
	if _, err := a.db.Exec(ctx, `INSERT INTO accounts(id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, claims.Subject); err != nil {
		return Principal{}, fmt.Errorf("ensure account: %w", err)
	}
	var systemRole, status string
	if err := a.db.QueryRow(ctx, `SELECT system_role, status FROM accounts WHERE id=$1`, claims.Subject).Scan(&systemRole, &status); err != nil {
		return Principal{}, ErrUnauthorized
	}
	if status != "active" || (systemRole != string(domain.SystemRoleUser) && systemRole != string(domain.SystemRoleAdmin)) {
		return Principal{}, ErrUnauthorized
	}
	return Principal{AccountID: domain.ID(claims.Subject), SystemRole: domain.SystemRole(systemRole)}, nil
}

func parseJWT(raw string) (jwtHeader, jwtClaims, []byte, []byte, error) {
	var header jwtHeader
	var claims jwtClaims
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return header, claims, nil, nil, errors.New("malformed jwt")
	}
	decode := func(part string, dest any) error {
		data, err := base64.RawURLEncoding.DecodeString(part)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, dest)
	}
	if err := decode(parts[0], &header); err != nil || header.Kid == "" || header.Alg == "" {
		return header, claims, nil, nil, errors.New("invalid jwt header")
	}
	if err := decode(parts[1], &claims); err != nil {
		return header, claims, nil, nil, errors.New("invalid jwt claims")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return header, claims, nil, nil, err
	}
	return header, claims, []byte(parts[0] + "." + parts[1]), sig, nil
}

func (a *SupabaseAuthenticator) signingKey(ctx context.Context, kid string) (jwkKey, error) {
	a.mu.RLock()
	key, found := a.keys[kid]
	valid := time.Now().Before(a.keysExpire)
	a.mu.RUnlock()
	if found && valid {
		return key, nil
	}
	if err := a.refreshKeys(ctx); err != nil {
		return jwkKey{}, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	key, found = a.keys[kid]
	if !found {
		return jwkKey{}, errors.New("signing key not found")
	}
	return key, nil
}

func (a *SupabaseAuthenticator) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS returned status %d", resp.StatusCode)
	}
	var document jwksDocument
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&document); err != nil {
		return err
	}
	keys := make(map[string]jwkKey, len(document.Keys))
	for _, key := range document.Keys {
		if key.Kid != "" && (key.Use == "" || key.Use == "sig") {
			keys[key.Kid] = key
		}
	}
	if len(keys) == 0 {
		return errors.New("JWKS contains no signing keys")
	}
	a.mu.Lock()
	a.keys = keys
	a.keysExpire = time.Now().Add(a.cacheTTL)
	a.mu.Unlock()
	return nil
}

func verifySignature(alg string, jwk jwkKey, message, signature []byte) bool {
	if jwk.Alg != "" && jwk.Alg != alg {
		return false
	}
	switch alg {
	case "RS256":
		if jwk.Kty != "RSA" {
			return false
		}
		pub, err := rsaPublicKey(jwk)
		if err != nil {
			return false
		}
		digest := sha256.Sum256(message)
		return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], signature) == nil
	case "ES256", "ES384", "ES512":
		if jwk.Kty != "EC" {
			return false
		}
		pub, err := ecPublicKey(jwk)
		if err != nil {
			return false
		}
		var digest []byte
		switch alg {
		case "ES256":
			sum := sha256.Sum256(message)
			digest = sum[:]
		case "ES384":
			sum := sha512.Sum384(message)
			digest = sum[:]
		case "ES512":
			sum := sha512.Sum512(message)
			digest = sum[:]
		}
		size := (pub.Curve.Params().BitSize + 7) / 8
		if len(signature) != size*2 {
			return false
		}
		r := new(big.Int).SetBytes(signature[:size])
		s := new(big.Int).SetBytes(signature[size:])
		return ecdsa.Verify(pub, digest, r, s)
	case "EdDSA":
		if jwk.Kty != "OKP" || jwk.Crv != "Ed25519" {
			return false
		}
		x, err := base64.RawURLEncoding.DecodeString(jwk.X)
		return err == nil && ed25519.Verify(ed25519.PublicKey(x), message, signature)
	default:
		return false
	}
}

func rsaPublicKey(key jwkKey) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil || len(e) == 0 || len(e) > 4 {
		return nil, errors.New("invalid RSA exponent")
	}
	exponent := 0
	for _, b := range e {
		exponent = exponent<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}, nil
}

func ecPublicKey(key jwkKey) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch key.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, errors.New("unsupported EC curve")
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil {
		return nil, err
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(key.Y)
	if err != nil {
		return nil, err
	}
	x, y := new(big.Int).SetBytes(xBytes), new(big.Int).SetBytes(yBytes)
	if !curve.IsOnCurve(x, y) {
		return nil, errors.New("EC point is not on curve")
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

func validAudience(raw json.RawMessage, expected string) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == expected
	}
	var multiple []string
	if json.Unmarshal(raw, &multiple) == nil {
		for _, value := range multiple {
			if value == expected {
				return true
			}
		}
	}
	return false
}
