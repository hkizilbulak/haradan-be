package paytr_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	apppaytr "github.com/hkizilbulak/haradan-be/internal/application/paytr"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainpaytr "github.com/hkizilbulak/haradan-be/internal/domain/paytr"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	paytrclient "github.com/hkizilbulak/haradan-be/internal/infrastructure/paytr"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type memCharges struct {
	byOID map[string]domainpaytr.Charge
}

func (m *memCharges) Create(_ context.Context, c domainpaytr.Charge) error {
	if m.byOID == nil {
		m.byOID = map[string]domainpaytr.Charge{}
	}
	m.byOID[c.MerchantOID] = c
	return nil
}
func (m *memCharges) FindByMerchantOID(_ context.Context, oid string) (domainpaytr.Charge, error) {
	c, ok := m.byOID[oid]
	if !ok {
		return domainpaytr.Charge{}, apperr.NotFound("Ödeme kaydı bulunamadı.")
	}
	return c, nil
}
func (m *memCharges) FindByMerchantOIDForUpdate(ctx context.Context, oid string) (domainpaytr.Charge, error) {
	return m.FindByMerchantOID(ctx, oid)
}
func (m *memCharges) FindByIDForOwner(_ context.Context, ownerID, chargeID uuid.UUID) (domainpaytr.Charge, error) {
	for _, c := range m.byOID {
		if c.ID == chargeID && c.OwnerUserID == ownerID {
			return c, nil
		}
	}
	return domainpaytr.Charge{}, fmt.Errorf("not found")
}
func (m *memCharges) Update(_ context.Context, c domainpaytr.Charge) error {
	m.byOID[c.MerchantOID] = c
	return nil
}

type stubPackages struct {
	pkg domainpackaging.Package
}

func (s stubPackages) FindByCode(_ context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error) {
	if s.pkg.Code != code {
		return domainpackaging.Package{}, fmt.Errorf("missing package")
	}
	return s.pkg, nil
}

type stubAdverts struct {
	adv domainadvert.Advert
}

func (s stubAdverts) FindByIDForOwner(_ context.Context, ownerID uuid.UUID, advertID int64) (domainadvert.Advert, error) {
	if s.adv.OwnerUserID != ownerID || s.adv.ID != advertID {
		return domainadvert.Advert{}, fmt.Errorf("not found")
	}
	return s.adv, nil
}
func (s stubAdverts) FindByID(_ context.Context, advertID int64) (domainadvert.Advert, error) {
	if s.adv.ID != advertID {
		return domainadvert.Advert{}, fmt.Errorf("not found")
	}
	return s.adv, nil
}

type stubUsers struct {
	u domainuser.User
}

func (s stubUsers) FindByID(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	if s.u.ID != id {
		return domainuser.User{}, fmt.Errorf("not found")
	}
	return s.u, nil
}

type stubPackaging struct {
	assigned int
}

func (s *stubPackaging) AssignAdvertPackage(_ context.Context, in apppackaging.AssignAdvertPackageInput) (apppackaging.AssignmentView, error) {
	s.assigned++
	if in.Source != domainpackaging.AssignmentSourcePayment {
		return apppackaging.AssignmentView{}, fmt.Errorf("expected PAYMENT source")
	}
	return apppackaging.AssignmentView{}, nil
}

type stubSubmitter struct {
	submitted int
}

func (s *stubSubmitter) SubmitAdvertForReview(_ context.Context, _ uuid.UUID, _ int64, _ int) (domainadvert.OwnerView, error) {
	s.submitted++
	return domainadvert.OwnerView{}, nil
}
func (s *stubSubmitter) ResubmitAdvertForReview(_ context.Context, _ uuid.UUID, _ int64, _ int) (domainadvert.OwnerView, error) {
	s.submitted++
	return domainadvert.OwnerView{}, nil
}

type stubGateway struct {
	token string
	salt  string
	key   string
}

func (g stubGateway) GetToken(_ context.Context, in paytrclient.TokenRequest) (paytrclient.TokenResult, error) {
	return paytrclient.TokenResult{
		Token:       g.token,
		IframeURL:   paytrclient.IframeURL(g.token),
		RequestForm: url.Values{"merchant_oid": {in.MerchantOID}},
		RawResponse: `{"status":"success","token":"` + g.token + `"}`,
	}, nil
}
func (g stubGateway) VerifyNotifyHash(merchantOID, status, totalAmount, hash string) bool {
	mac := hmac.New(sha256.New, []byte(g.key))
	_, _ = mac.Write([]byte(merchantOID + g.salt + status + totalAmount))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(hash))
}

func testSvc(t *testing.T) (*apppaytr.Service, *memCharges, *stubPackaging, *stubSubmitter, domainadvert.Advert, uuid.UUID) {
	t.Helper()
	owner := uuid.New()
	price := int64(25000)
	title := "Test ilan"
	adv := domainadvert.Advert{
		ID:          42,
		OwnerUserID: owner,
		Status:      domainadvert.StatusDraft,
		Version:     1,
		Title:       &title,
	}
	user := domainuser.User{
		ID:        owner,
		Email:     "buyer@example.com",
		FirstName: "Ali",
		LastName:  "Veli",
		Status:    domainuser.StatusActive,
	}
	charges := &memCharges{}
	packaging := &stubPackaging{}
	submitter := &stubSubmitter{}
	std := domainpackaging.PackageCode("STANDARD")
	svc, err := apppaytr.NewService(apppaytr.Config{
		Charges:      charges,
		Packages:     stubPackages{pkg: domainpackaging.Package{Code: std, DisplayName: "Standart", IsActive: true, DisplayPriceAmountMinor: &price}},
		Adverts:      stubAdverts{adv: adv},
		Users:        stubUsers{u: user},
		Packaging:    packaging,
		Submitter:    submitter,
		Gateway:      stubGateway{token: "tok123", salt: "salt", key: "key"},
		FrontendURL:  "http://localhost:8081",
		APIPublicURL: "http://localhost:8080/api",
		Clock:        fixedClock{t: time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, charges, packaging, submitter, adv, owner
}

func TestStartCheckoutCreatesPendingCharge(t *testing.T) {
	svc, charges, _, _, adv, owner := testSvc(t)
	res, err := svc.StartCheckout(context.Background(), apppaytr.CheckoutInput{
		OwnerUserID: owner,
		AdvertID:    adv.ID,
		PackageCode: domainpackaging.PackageCode("STANDARD"),
		UserIP:      "1.2.3.4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IframeURL == "" || res.MerchantOID == "" {
		t.Fatalf("missing iframe/oid: %+v", res)
	}
	if res.AmountMinor != 25000 {
		t.Fatalf("amount=%d", res.AmountMinor)
	}
	stored, ok := charges.byOID[res.MerchantOID]
	if !ok || stored.Status != domainpaytr.ChargeStatusPending {
		t.Fatalf("charge not pending: %+v", stored)
	}
}

func TestHandleNotifySuccessAssignsAndSubmits(t *testing.T) {
	svc, charges, packaging, submitter, adv, owner := testSvc(t)
	res, err := svc.StartCheckout(context.Background(), apppaytr.CheckoutInput{
		OwnerUserID: owner,
		AdvertID:    adv.ID,
		PackageCode: domainpackaging.PackageCode("STANDARD"),
		UserIP:      "1.2.3.4",
	})
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte("key"))
	_, _ = mac.Write([]byte(res.MerchantOID + "salt" + "success" + "25000"))
	hash := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	body, err := svc.HandleNotify(context.Background(), apppaytr.NotifyInput{
		MerchantOID: res.MerchantOID,
		Status:      "success",
		TotalAmount: "25000",
		Hash:        hash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if body != "OK" {
		t.Fatalf("body=%s", body)
	}
	if packaging.assigned != 1 {
		t.Fatalf("assigned=%d", packaging.assigned)
	}
	if submitter.submitted != 1 {
		t.Fatalf("submitted=%d", submitter.submitted)
	}
	if charges.byOID[res.MerchantOID].Status != domainpaytr.ChargeStatusSucceeded {
		t.Fatalf("status=%s", charges.byOID[res.MerchantOID].Status)
	}

	// Idempotent second notify
	body, err = svc.HandleNotify(context.Background(), apppaytr.NotifyInput{
		MerchantOID: res.MerchantOID,
		Status:      "success",
		TotalAmount: "25000",
		Hash:        hash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if body != "OK" || packaging.assigned != 1 {
		t.Fatalf("idempotent failed body=%s assigned=%d", body, packaging.assigned)
	}
}

func TestHandleNotifyRejectsBadHash(t *testing.T) {
	svc, _, _, _, adv, owner := testSvc(t)
	res, err := svc.StartCheckout(context.Background(), apppaytr.CheckoutInput{
		OwnerUserID: owner,
		AdvertID:    adv.ID,
		PackageCode: domainpackaging.PackageCode("STANDARD"),
		UserIP:      "1.2.3.4",
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := svc.HandleNotify(context.Background(), apppaytr.NotifyInput{
		MerchantOID: res.MerchantOID,
		Status:      "success",
		TotalAmount: "25000",
		Hash:        "bad",
	})
	if err != nil {
		t.Fatal(err)
	}
	if body != "PAYTR notification failed: bad hash" {
		t.Fatalf("body=%s", body)
	}
}

func TestHandleNotifyUnknownMerchantOIDReturnsOK(t *testing.T) {
	svc, _, _, _, _, _ := testSvc(t)
	mac := hmac.New(sha256.New, []byte("key"))
	_, _ = mac.Write([]byte("unknownoid" + "salt" + "success" + "25000"))
	hash := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	body, err := svc.HandleNotify(context.Background(), apppaytr.NotifyInput{
		MerchantOID: "unknownoid",
		Status:      "success",
		TotalAmount: "25000",
		Hash:        hash,
	})
	if err != nil {
		t.Fatalf("unknown oid must not error: %v", err)
	}
	if body != "OK" {
		t.Fatalf("body=%s", body)
	}
}

func TestHandleNotifyMissingFieldsTreatedAsBadHash(t *testing.T) {
	svc, _, _, _, _, _ := testSvc(t)
	body, err := svc.HandleNotify(context.Background(), apppaytr.NotifyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if body != "PAYTR notification failed: bad hash" {
		t.Fatalf("body=%s", body)
	}
}

func TestHandleNotifyAmountMismatch(t *testing.T) {
	svc, charges, packaging, _, adv, owner := testSvc(t)
	res, err := svc.StartCheckout(context.Background(), apppaytr.CheckoutInput{
		OwnerUserID: owner,
		AdvertID:    adv.ID,
		PackageCode: domainpackaging.PackageCode("STANDARD"),
		UserIP:      "1.2.3.4",
	})
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte("key"))
	_, _ = mac.Write([]byte(res.MerchantOID + "salt" + "success" + "999"))
	hash := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	body, err := svc.HandleNotify(context.Background(), apppaytr.NotifyInput{
		MerchantOID: res.MerchantOID,
		Status:      "success",
		TotalAmount: "999",
		Hash:        hash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if body != "OK" {
		t.Fatalf("body=%s", body)
	}
	if packaging.assigned != 0 {
		t.Fatalf("should not assign on amount mismatch")
	}
	if charges.byOID[res.MerchantOID].Status != domainpaytr.ChargeStatusFailed {
		t.Fatalf("status=%s", charges.byOID[res.MerchantOID].Status)
	}
}

type captureGateway struct {
	lastIP       string
	lastNotify   string
	rejectReason string
}

func (g *captureGateway) GetToken(_ context.Context, in paytrclient.TokenRequest) (paytrclient.TokenResult, error) {
	g.lastIP = in.UserIP
	g.lastNotify = in.MerchantNotifyURL
	if g.rejectReason != "" {
		return paytrclient.TokenResult{}, fmt.Errorf("paytr token rejected: %s", g.rejectReason)
	}
	return paytrclient.TokenResult{
		Token:       "tok",
		IframeURL:   paytrclient.IframeURL("tok"),
		RequestForm: url.Values{},
		RawResponse: `{"status":"success","token":"tok"}`,
	}, nil
}
func (g *captureGateway) VerifyNotifyHash(_, _, _, _ string) bool { return true }

func TestStartCheckoutResolvesLoopbackIP(t *testing.T) {
	owner := uuid.New()
	price := int64(25000)
	title := "Test"
	adv := domainadvert.Advert{ID: 1, OwnerUserID: owner, Status: domainadvert.StatusDraft, Version: 1, Title: &title}
	user := domainuser.User{ID: owner, Email: "a@b.com", FirstName: "A", LastName: "B", Status: domainuser.StatusActive}
	gw := &captureGateway{}
	std := domainpackaging.PackageCode("STANDARD")
	svc, err := apppaytr.NewService(apppaytr.Config{
		Charges:        &memCharges{},
		Packages:       stubPackages{pkg: domainpackaging.Package{Code: std, DisplayName: "S", IsActive: true, DisplayPriceAmountMinor: &price}},
		Adverts:        stubAdverts{adv: adv},
		Users:          stubUsers{u: user},
		Packaging:      &stubPackaging{},
		Submitter:      &stubSubmitter{},
		Gateway:        gw,
		FrontendURL:    "http://localhost:8081",
		APIPublicURL:   "http://localhost:8080/api",
		UserIPOverride: "",
		PublicIP:       func(context.Context) (string, error) { return "9.9.9.9", nil },
		Clock:          fixedClock{t: time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartCheckout(context.Background(), apppaytr.CheckoutInput{
		OwnerUserID: owner, AdvertID: 1, PackageCode: std, UserIP: "127.0.0.1",
	}); err != nil {
		t.Fatal(err)
	}
	if gw.lastIP != "9.9.9.9" {
		t.Fatalf("ip=%s", gw.lastIP)
	}
	if gw.lastNotify != "" {
		t.Fatalf("localhost notify should be omitted, got %q", gw.lastNotify)
	}
}

func TestStartCheckoutStoreInactiveMessage(t *testing.T) {
	owner := uuid.New()
	price := int64(25000)
	title := "Test"
	adv := domainadvert.Advert{ID: 1, OwnerUserID: owner, Status: domainadvert.StatusDraft, Version: 1, Title: &title}
	user := domainuser.User{ID: owner, Email: "a@b.com", FirstName: "A", LastName: "B", Status: domainuser.StatusActive}
	gw := &captureGateway{rejectReason: "Gecersiz istek veya magaza aktif degil"}
	std := domainpackaging.PackageCode("STANDARD")
	svc, err := apppaytr.NewService(apppaytr.Config{
		Charges:      &memCharges{},
		Packages:     stubPackages{pkg: domainpackaging.Package{Code: std, DisplayName: "S", IsActive: true, DisplayPriceAmountMinor: &price}},
		Adverts:      stubAdverts{adv: adv},
		Users:        stubUsers{u: user},
		Packaging:    &stubPackaging{},
		Submitter:    &stubSubmitter{},
		Gateway:      gw,
		FrontendURL:  "https://www.haradan.com",
		APIPublicURL: "https://api.haradan.com/api",
		Clock:        fixedClock{t: time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.StartCheckout(context.Background(), apppaytr.CheckoutInput{
		OwnerUserID: owner, AdvertID: 1, PackageCode: std, UserIP: "1.2.3.4",
	})
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(ae.Message, "PayTR mağazası aktif değil") {
		t.Fatalf("message=%q", ae.Message)
	}
	if gw.lastNotify == "" {
		t.Fatal("expected https notify url")
	}
}
