package s3storage

import (
	"fmt"
	"strings"
)

// Config holds the S3-compatible provider settings required to build a Storage
// adapter. Values come from process configuration; nothing is hardcoded.
type Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	// BasePath is an optional already-normalized prefix (no leading/trailing
	// slash). Empty means logical object keys are used as provider keys.
	BasePath string
}

func (c Config) validate() error {
	if strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("storage endpoint must not be empty")
	}
	if strings.TrimSpace(c.Region) == "" {
		return fmt.Errorf("storage region must not be empty")
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return fmt.Errorf("storage bucket must not be empty")
	}
	if strings.TrimSpace(c.AccessKey) == "" {
		return fmt.Errorf("storage access key must not be empty")
	}
	if strings.TrimSpace(c.SecretKey) == "" {
		return fmt.Errorf("storage secret key must not be empty")
	}
	if err := validateBasePath(c.BasePath); err != nil {
		return err
	}
	return nil
}

func validateBasePath(basePath string) error {
	if basePath == "" {
		return nil
	}
	if strings.Contains(basePath, `\`) {
		return fmt.Errorf("storage base path must not contain backslashes")
	}
	if strings.HasPrefix(basePath, "/") || strings.HasSuffix(basePath, "/") {
		return fmt.Errorf("storage base path must be normalized without leading or trailing slashes")
	}
	for _, part := range strings.Split(basePath, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("storage base path contains an invalid path segment")
		}
	}
	return nil
}
