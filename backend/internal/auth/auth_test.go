package auth

import (
	"encoding/hex"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/scrypt"
	"strings"
	"testing"
	"time"
)

func TestPasswords(t *testing.T) {
	h, e := Hash("a sufficiently long password")
	if e != nil {
		t.Fatal(e)
	}
	if ok, legacy := Verify("a sufficiently long password", h); !ok || legacy {
		t.Fatal("new password failed")
	}
	if ok, _ := Verify("wrong", h); ok {
		t.Fatal("wrong password accepted")
	}
	salt := "0123456789abcdef0123456789abcdef"
	key, _ := scrypt.Key([]byte("old password"), []byte(salt), 16384, 8, 1, 64)
	old := salt + "." + hex.EncodeToString(key)
	if ok, legacy := Verify("old password", old); !ok || !legacy {
		t.Fatal("legacy failed")
	}
	for _, bad := range []string{"", "bad.hash", "$argon2id$v=19$m=999999999,t=2,p=1$AAAA$BBBB"} {
		if ok, _ := Verify("password", bad); ok {
			t.Fatal("malformed hash accepted")
		}
	}
}
func TestTokens(t *testing.T) {
	tokens := Tokens{[]byte(strings.Repeat("s", 32))}
	raw, e := tokens.Issue("user-1", "admin")
	if e != nil {
		t.Fatal(e)
	}
	c, e := tokens.Verify(raw)
	if e != nil || c.Subject != "user-1" || c.UserID != "user-1" || c.Role != "admin" || c.ExpiresAt.Sub(c.IssuedAt.Time) != 15*time.Minute {
		t.Fatalf("claims: %+v %v", c, e)
	}
	for name, claims := range map[string]Claims{"expired": {Role: "user", RegisteredClaims: jwt.RegisteredClaims{Subject: "u", IssuedAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))}}, "no expiry": {Role: "user", RegisteredClaims: jwt.RegisteredClaims{Subject: "u", IssuedAt: jwt.NewNumericDate(time.Now())}}} {
		t.Run(name, func(t *testing.T) {
			raw, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tokens.Secret)
			if _, e := tokens.Verify(raw); e == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodNone, jwt.SigningMethodHS384} {
		raw, _ := jwt.NewWithClaims(method, c).SignedString(jwt.UnsafeAllowNoneSignatureType)
		if method != jwt.SigningMethodNone {
			raw, _ = jwt.NewWithClaims(method, c).SignedString(tokens.Secret)
		}
		if _, e := tokens.Verify(raw); e == nil {
			t.Fatal("wrong algorithm accepted")
		}
	}
}
