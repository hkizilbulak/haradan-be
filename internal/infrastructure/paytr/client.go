package paytrclient

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTokenURL = "https://www.paytr.com/odeme/api/get-token"
	iframeBaseURL   = "https://www.paytr.com/odeme/guvenli/"
	maxErrorBytes   = 4 << 10
)

// Config holds PayTR merchant credentials and runtime flags.
type Config struct {
	MerchantID     string
	MerchantKey    string
	MerchantSalt   string
	TokenURL       string
	HTTPTimeout    time.Duration
	TestMode       bool
	DebugOn        bool
	NoInstallment  bool
	MaxInstallment string
	TimeoutLimit   string
	Currency       string
}

// TokenRequest is the payload for get-token.
type TokenRequest struct {
	MerchantOID       string
	UserIP            string
	Email             string
	PaymentAmount     string // minor units as decimal digits, e.g. "25000"
	UserName          string
	UserAddress       string
	UserPhone         string
	MerchantOKURL     string
	MerchantFailURL   string
	MerchantNotifyURL string
	BasketTitle       string
	AmountMinor       int64
}

// TokenResult is a successful get-token response.
type TokenResult struct {
	Token       string
	IframeURL   string
	RequestForm url.Values
	RawResponse string
}

// Client talks to PayTR get-token and verifies notify hashes.
type Client struct {
	cfg  Config
	http *http.Client
}

// New builds a Client. It performs no network I/O.
func New(cfg Config) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tokenURL := strings.TrimSpace(cfg.TokenURL)
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	cfg.TokenURL = tokenURL
	if cfg.Currency == "" {
		cfg.Currency = "TL"
	}
	if cfg.TimeoutLimit == "" {
		cfg.TimeoutLimit = "30"
	}
	if cfg.MaxInstallment == "" {
		cfg.MaxInstallment = "0"
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c Config) validate() error {
	if strings.TrimSpace(c.MerchantID) == "" {
		return fmt.Errorf("paytr merchant id must not be empty")
	}
	if strings.TrimSpace(c.MerchantKey) == "" {
		return fmt.Errorf("paytr merchant key must not be empty")
	}
	if strings.TrimSpace(c.MerchantSalt) == "" {
		return fmt.Errorf("paytr merchant salt must not be empty")
	}
	return nil
}

// GetToken requests an iframe token from PayTR.
func (c *Client) GetToken(ctx context.Context, in TokenRequest) (TokenResult, error) {
	if err := ctx.Err(); err != nil {
		return TokenResult{}, err
	}
	basket, err := encodeBasket(in.BasketTitle, in.AmountMinor)
	if err != nil {
		return TokenResult{}, err
	}
	noInstallment := "1"
	if !c.cfg.NoInstallment {
		noInstallment = "0"
	}
	testMode := "0"
	if c.cfg.TestMode {
		testMode = "1"
	}
	debugOn := "0"
	if c.cfg.DebugOn {
		debugOn = "1"
	}

	paytrToken := c.hash(
		c.cfg.MerchantID + in.UserIP + in.MerchantOID + in.Email + in.PaymentAmount +
			basket + noInstallment + c.cfg.MaxInstallment + c.cfg.Currency + testMode + c.cfg.MerchantSalt,
	)

	form := url.Values{}
	form.Set("merchant_id", c.cfg.MerchantID)
	form.Set("user_ip", in.UserIP)
	form.Set("merchant_oid", in.MerchantOID)
	form.Set("email", in.Email)
	form.Set("payment_amount", in.PaymentAmount)
	form.Set("payment_type", "card")
	form.Set("paytr_token", paytrToken)
	form.Set("user_basket", basket)
	form.Set("debug_on", debugOn)
	form.Set("no_installment", noInstallment)
	form.Set("max_installment", c.cfg.MaxInstallment)
	form.Set("user_name", in.UserName)
	form.Set("user_address", in.UserAddress)
	form.Set("user_phone", in.UserPhone)
	form.Set("merchant_ok_url", in.MerchantOKURL)
	form.Set("merchant_fail_url", in.MerchantFailURL)
	form.Set("timeout_limit", c.cfg.TimeoutLimit)
	form.Set("currency", c.cfg.Currency)
	form.Set("test_mode", testMode)
	if strings.TrimSpace(in.MerchantNotifyURL) != "" {
		form.Set("merchant_notify_url", in.MerchantNotifyURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResult{}, fmt.Errorf("paytr token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return TokenResult{}, fmt.Errorf("paytr token call: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
	if err != nil {
		return TokenResult{}, fmt.Errorf("paytr token read: %w", err)
	}
	var parsed struct {
		Status string `json:"status"`
		Token  string `json:"token"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return TokenResult{}, fmt.Errorf("paytr token decode: %w", err)
	}
	if !strings.EqualFold(parsed.Status, "success") || strings.TrimSpace(parsed.Token) == "" {
		reason := strings.TrimSpace(parsed.Reason)
		if reason == "" {
			reason = string(raw)
		}
		return TokenResult{}, fmt.Errorf("paytr token rejected: %s", reason)
	}
	token := strings.TrimSpace(parsed.Token)
	return TokenResult{
		Token:       token,
		IframeURL:   iframeBaseURL + token,
		RequestForm: form,
		RawResponse: string(raw),
	}, nil
}

// VerifyNotifyHash validates the PayTR server-to-server callback hash.
func (c *Client) VerifyNotifyHash(merchantOID, status, totalAmount, hash string) bool {
	expected := c.hash(merchantOID + c.cfg.MerchantSalt + status + totalAmount)
	return hmac.Equal([]byte(expected), []byte(hash))
}

// IframeURL builds the hosted payment page URL for a token.
func IframeURL(token string) string {
	return iframeBaseURL + strings.TrimSpace(token)
}

func (c *Client) hash(payload string) string {
	mac := hmac.New(sha256.New, []byte(c.cfg.MerchantKey))
	_, _ = mac.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func encodeBasket(title string, amountMinor int64) (string, error) {
	name := strings.TrimSpace(title)
	if name == "" {
		name = "Ilan paketi"
	}
	price := formatMinorAsDecimal(amountMinor)
	basket := [][]any{{name, price, 1}}
	raw, err := json.Marshal(basket)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// formatMinorAsDecimal turns 25000 into "250.00".
func formatMinorAsDecimal(minor int64) string {
	if minor < 0 {
		minor = 0
	}
	whole := minor / 100
	frac := minor % 100
	return strconv.FormatInt(whole, 10) + "." + fmt.Sprintf("%02d", frac)
}

// AmountMinorString returns the PayTR payment_amount field (minor units, no decimal).
func AmountMinorString(minor int64) string {
	if minor < 0 {
		minor = 0
	}
	return strconv.FormatInt(minor, 10)
}
