package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/itzraghavv/pixel-pals/backend/internal/platform"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestPresign(t *testing.T) {
	s, e := New(platform.Config{S3Endpoint: "http://localhost:9000", S3PublicURL: "http://localhost:9000/pixel-pals", S3Bucket: "pixel-pals", S3Region: "us-east-1", S3AccessKey: "minioadmin", S3SecretKey: "minioadmin"})
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.Sign(context.Background(), "image/png", 4)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(out)
	var v struct {
		URL string `json:"uploadUrl"`
	}
	_ = json.Unmarshal(raw, &v)
	u, _ := url.Parse(v.URL)
	signed := u.Query().Get("X-Amz-SignedHeaders")
	if !strings.Contains(signed, "content-length") || !strings.Contains(signed, "content-type") {
		t.Fatalf("upload constraints not signed: %s", signed)
	}
	for _, v := range []struct {
		typ  string
		size int64
	}{{"image/svg+xml", 4}, {"image/png", 2097153}, {"image/png", 0}} {
		if _, e = s.Sign(context.Background(), v.typ, v.size); e == nil {
			t.Fatal("invalid upload accepted")
		}
	}
}

// Opt-in real object storage verification, also run by CI with MinIO.
func TestMinIOUpload(t *testing.T) {
	if os.Getenv("TEST_S3_ENDPOINT") == "" {
		t.Skip("set TEST_S3_ENDPOINT to run real S3 upload test")
	}
	s, e := New(platform.Config{S3Endpoint: os.Getenv("TEST_S3_ENDPOINT"), S3PublicURL: os.Getenv("TEST_S3_ENDPOINT") + "/pixel-pals", S3Bucket: "pixel-pals", S3Region: "us-east-1", S3AccessKey: "minioadmin", S3SecretKey: "minioadmin"})
	if e != nil {
		t.Fatal(e)
	}
	result, e := s.Sign(context.Background(), "image/png", 4)
	if e != nil {
		t.Fatal(e)
	}
	m := result.(map[string]any)
	raw := m["uploadUrl"].(string)
	for _, tc := range []struct {
		body, typ string
		ok        bool
	}{{"12345", "image/png", false}, {"1234", "image/webp", false}, {"1234", "image/png", true}} {
		r, _ := http.NewRequest("PUT", raw, bytes.NewBufferString(tc.body))
		r.Header.Set("Content-Type", tc.typ)
		resp, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if (resp.StatusCode < 300) != tc.ok {
			t.Fatalf("upload status %d body %s", resp.StatusCode, body)
		}
	}
	resp, e := http.Get(m["publicUrl"].(string))
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != "1234" {
		t.Fatalf("public URL not readable: %d %q", resp.StatusCode, body)
	}
}
