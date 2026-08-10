package tinifyprocessor

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

// ProfileConfig holds fit resize bounds for one transform profile.
type ProfileConfig struct {
	Width  int
	Height int
}

// Config holds Tinify adapter settings. Values come from process configuration.
type Config struct {
	APIKey      string
	BaseURL     string
	HTTPTimeout time.Duration
	Profiles    map[string]ProfileConfig
}

func (c Config) validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("tinify API key must not be empty")
	}
	if c.HTTPTimeout <= 0 {
		return fmt.Errorf("tinify HTTP timeout must be greater than zero")
	}
	if _, err := parseBaseURL(c.BaseURL); err != nil {
		return err
	}
	required := []string{
		domainmedia.ProfileDetail,
		domainmedia.ProfileHomepage,
		domainmedia.ProfileSearch,
	}
	for _, name := range required {
		p, ok := c.Profiles[name]
		if !ok {
			return fmt.Errorf("tinify profile %s is required", name)
		}
		if p.Width <= 0 || p.Height <= 0 {
			return fmt.Errorf("tinify profile %s width and height must be greater than zero", name)
		}
	}
	return nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("tinify base URL must not be empty")
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("tinify base URL is not a valid URL")
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("tinify base URL must use https")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("tinify base URL must not contain userinfo, query, or fragment")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("tinify base URL host must not be empty")
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	if path != "" {
		return nil, fmt.Errorf("tinify base URL must not include a path")
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}
