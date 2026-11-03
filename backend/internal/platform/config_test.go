package platform

import (
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://pals:pals@localhost/pals")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("APP_ENV", "production")
	t.Setenv("ALLOW_ADMIN_SIGNUP", "false")
	if _, e := Load(); e != nil {
		t.Fatal(e)
	}
	for _, v := range []struct{ k, value string }{{"JWT_SECRET", "short"}, {"HTTP_PORT", "oops"}, {"DATABASE_URL", ""}, {"REDIS_URL", "oops"}, {"ALLOW_ADMIN_SIGNUP", "true"}, {"COMPAT_V0", "oops"}, {"CORS_ORIGINS", "*"}} {
		t.Run(v.k, func(t *testing.T) {
			t.Setenv(v.k, v.value)
			if _, e := Load(); e == nil {
				t.Fatalf("accepted %s=%q", v.k, v.value)
			}
		})
	}
}
