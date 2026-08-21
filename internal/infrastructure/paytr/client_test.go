package paytrclient_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	paytrclient "github.com/hkizilbulak/haradan-be/internal/infrastructure/paytr"
)

func TestAmountMinorString(t *testing.T) {
	if got := paytrclient.AmountMinorString(25000); got != "25000" {
		t.Fatalf("amount=%s", got)
	}
}

func TestVerifyNotifyHash(t *testing.T) {
	c, err := paytrclient.New(paytrclient.Config{
		MerchantID:   "mid",
		MerchantKey:  "merchant-key",
		MerchantSalt: "merchant-salt",
		HTTPTimeout:  time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := "oid123" + "merchant-salt" + "success" + "25000"
	mac := hmac.New(sha256.New, []byte("merchant-key"))
	_, _ = mac.Write([]byte(payload))
	hash := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !c.VerifyNotifyHash("oid123", "success", "25000", hash) {
		t.Fatal("expected valid hash")
	}
	if c.VerifyNotifyHash("oid123", "success", "25000", "bad") {
		t.Fatal("expected reject")
	}
}

func TestIframeURL(t *testing.T) {
	if got := paytrclient.IframeURL("tok"); got != "https://www.paytr.com/odeme/guvenli/tok" {
		t.Fatalf("url=%s", got)
	}
}
