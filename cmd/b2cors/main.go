// Command b2cors applies browser CORS rules to the configured S3/B2 bucket.
// Usage (from haradan-be with .env loaded): go run ./cmd/b2cors
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/joho/godotenv"

	"github.com/hkizilbulak/haradan-be/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "b2cors: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.StorageProvider != config.StorageProviderB2 {
		return fmt.Errorf("STORAGE_PROVIDER must be b2 (got %q)", cfg.StorageProvider)
	}

	origins := uniqueOrigins(cfg.CORSAllowedOrigins)
	for _, host := range []string{"localhost", "127.0.0.1"} {
		for _, port := range []string{"8081", "8083", "3000", "3001", "19006"} {
			origins = append(origins, "http://"+host+":"+port)
		}
	}
	origins = uniqueOrigins(origins)
	if len(origins) == 0 {
		return fmt.Errorf("no CORS origins to allow")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(strings.TrimSpace(cfg.S3Region)),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				strings.TrimSpace(cfg.S3AccessKey),
				strings.TrimSpace(cfg.S3SecretKey),
				"",
			),
		),
	)
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(strings.TrimSpace(cfg.S3Endpoint))
		o.UsePathStyle = true
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(cfg.S3Bucket),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: []types.CORSRule{{
				AllowedOrigins: origins,
				AllowedMethods: []string{"GET", "PUT", "HEAD"},
				AllowedHeaders: []string{"*"},
				ExposeHeaders:  []string{"ETag", "Content-Type", "Content-Length"},
				MaxAgeSeconds:  aws.Int32(3600),
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("put bucket cors: %w", err)
	}

	out, err := client.GetBucketCors(ctx, &s3.GetBucketCorsInput{
		Bucket: aws.String(cfg.S3Bucket),
	})
	if err != nil {
		return fmt.Errorf("get bucket cors: %w", err)
	}
	fmt.Printf("bucket=%s origins=%d methods=%v\n", cfg.S3Bucket, len(out.CORSRules[0].AllowedOrigins), out.CORSRules[0].AllowedMethods)
	return nil
}

func uniqueOrigins(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
