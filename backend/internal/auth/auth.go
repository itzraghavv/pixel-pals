package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/scrypt"
)

func Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	key := argon2.IDKey([]byte(password), salt, 2, 64*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=2,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify accepts our bounded Argon2id profile and the Node scrypt defaults used by v0.
// v0 stores a hex salt string followed by a dot and a 64-byte hex digest.
func Verify(password, encoded string) (valid, legacy bool) {
	if strings.HasPrefix(encoded, "$argon2id$") {
		parts := strings.Split(encoded, "$")
		if len(parts) != 6 || parts[2] != "v=19" || parts[3] != "m=65536,t=2,p=1" {
			return false, false
		}
		salt, e := base64.RawStdEncoding.DecodeString(parts[4])
		if e != nil || len(salt) != 16 {
			return false, false
		}
		want, e := base64.RawStdEncoding.DecodeString(parts[5])
		if e != nil || len(want) != 32 {
			return false, false
		}
		got := argon2.IDKey([]byte(password), salt, 2, 64*1024, 1, 32)
		return subtle.ConstantTimeCompare(got, want) == 1, false
	}
	parts := strings.Split(encoded, ".")
	if len(parts) != 2 || len(parts[0]) < 16 || len(parts[0]) > 128 {
		return false, false
	}
	want, e := hex.DecodeString(parts[1])
	if e != nil || len(want) != 64 {
		return false, false
	}
	got, e := scrypt.Key([]byte(password), []byte(parts[0]), 16384, 8, 1, 64)
	return e == nil && subtle.ConstantTimeCompare(got, want) == 1, true
}

type Claims struct {
	Role   string `json:"role"`
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}
type Tokens struct{ Secret []byte }

func (t Tokens) Issue(id, role string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{role, id, jwt.RegisteredClaims{Subject: id, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute))}}).SignedString(t.Secret)
}
func (t Tokens) Verify(raw string) (*Claims, error) {
	claims := new(Claims)
	token, e := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return t.Secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if e != nil || !token.Valid || claims.Subject == "" || claims.IssuedAt == nil || claims.Role != "admin" && claims.Role != "user" {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
func Opaque() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func Digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
