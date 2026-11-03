package storage

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/itzraghavv/pixel-pals/backend/internal/platform"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"net/url"
	"strings"
	"time"
)

type S3 struct {
	Client            *s3.PresignClient
	Bucket, PublicURL string
}

func New(c platform.Config) (*S3, error) {
	if c.S3Bucket == "" {
		return nil, nil
	}
	for _, raw := range []string{c.S3Endpoint, c.S3PublicURL} {
		u, e := url.Parse(raw)
		if e != nil || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
			return nil, service.Fail(500, "INVALID_STORAGE_CONFIG", "S3_ENDPOINT and S3_PUBLIC_URL must be HTTP URLs")
		}
	}
	if c.S3AccessKey == "" || c.S3SecretKey == "" {
		return nil, service.Fail(500, "INVALID_STORAGE_CONFIG", "S3 credentials are required")
	}
	cfg := aws.Config{Region: c.S3Region, Credentials: credentials.NewStaticCredentialsProvider(c.S3AccessKey, c.S3SecretKey, "")}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(c.S3Endpoint)
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})
	return &S3{s3.NewPresignClient(client), c.S3Bucket, strings.TrimRight(c.S3PublicURL, "/")}, nil
}
func (s *S3) Sign(ctx context.Context, contentType string, size int64) (any, error) {
	if size < 1 || size > 2<<20 || contentType != "image/png" && contentType != "image/webp" {
		return nil, service.Fail(400, "INVALID_UPLOAD", "Expected PNG or WebP, at most 2 MB")
	}
	ext := ".png"
	if contentType == "image/webp" {
		ext = ".webp"
	}
	key := "uploads/" + service.ID() + ext
	signed, e := s.Client.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key), ContentType: aws.String(contentType), ContentLength: aws.Int64(size)}, func(o *s3.PresignOptions) { o.Expires = 5 * time.Minute })
	if e != nil {
		return nil, e
	}
	return map[string]any{"uploadUrl": signed.URL, "publicUrl": s.PublicURL + "/" + key, "headers": signed.SignedHeader, "expiresIn": 300}, nil
}
