package paytr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainpaytr "github.com/hkizilbulak/haradan-be/internal/domain/paytr"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	paytrclient "github.com/hkizilbulak/haradan-be/internal/infrastructure/paytr"
)

// ChargeRepository persists PayTR charges.
type ChargeRepository interface {
	Create(ctx context.Context, c domainpaytr.Charge) error
	FindByMerchantOID(ctx context.Context, merchantOID string) (domainpaytr.Charge, error)
	FindByMerchantOIDForUpdate(ctx context.Context, merchantOID string) (domainpaytr.Charge, error)
	FindByIDForOwner(ctx context.Context, ownerID, chargeID uuid.UUID) (domainpaytr.Charge, error)
	Update(ctx context.Context, c domainpaytr.Charge) error
}

// PackageCatalog loads active package pricing.
type PackageCatalog interface {
	FindByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error)
}

// AdvertAccess loads owner-scoped adverts.
type AdvertAccess interface {
	FindByIDForOwner(ctx context.Context, ownerID uuid.UUID, advertID int64) (domainadvert.Advert, error)
	FindByID(ctx context.Context, advertID int64) (domainadvert.Advert, error)
}

// UserAccess loads the payer profile.
type UserAccess interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
}

// PackagingAssigner assigns a paid package entitlement.
type PackagingAssigner interface {
	AssignAdvertPackage(ctx context.Context, in apppackaging.AssignAdvertPackageInput) (apppackaging.AssignmentView, error)
}

// AdvertSubmitter moves an advert into PENDING_REVIEW after successful charge.
type AdvertSubmitter interface {
	SubmitAdvertForReview(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error)
	ResubmitAdvertForReview(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error)
}

// TokenGateway talks to PayTR get-token / hash verification.
type TokenGateway interface {
	GetToken(ctx context.Context, in paytrclient.TokenRequest) (paytrclient.TokenResult, error)
	VerifyNotifyHash(merchantOID, status, totalAmount, hash string) bool
}

// Clock abstracts time for tests.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// PublicIPLookup resolves the machine's egress IP for local PayTR testing.
type PublicIPLookup func(ctx context.Context) (string, error)

// Config wires the checkout application service.
type Config struct {
	Charges        ChargeRepository
	Packages       PackageCatalog
	Adverts        AdvertAccess
	Users          UserAccess
	Packaging      PackagingAssigner
	Submitter      AdvertSubmitter
	Gateway        TokenGateway
	FrontendURL    string
	APIPublicURL   string // public base including /api, used for merchant_notify_url
	UserIPOverride string // optional fixed payer IP (local/dev)
	PublicIP       PublicIPLookup
	Clock          Clock
}

// Service orchestrates PayTR iframe checkout for listing packages.
type Service struct {
	charges        ChargeRepository
	packages       PackageCatalog
	adverts        AdvertAccess
	users          UserAccess
	packaging      PackagingAssigner
	submitter      AdvertSubmitter
	gateway        TokenGateway
	frontendURL    string
	apiPublicURL   string
	userIPOverride string
	publicIP       PublicIPLookup
	clock          Clock
}

// NewService constructs the PayTR application service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Charges == nil || cfg.Packages == nil || cfg.Adverts == nil || cfg.Users == nil ||
		cfg.Packaging == nil || cfg.Submitter == nil || cfg.Gateway == nil {
		return nil, fmt.Errorf("paytr service dependencies incomplete")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	publicIP := cfg.PublicIP
	if publicIP == nil {
		publicIP = lookupEgressIP
	}
	return &Service{
		charges:        cfg.Charges,
		packages:       cfg.Packages,
		adverts:        cfg.Adverts,
		users:          cfg.Users,
		packaging:      cfg.Packaging,
		submitter:      cfg.Submitter,
		gateway:        cfg.Gateway,
		frontendURL:    strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/"),
		apiPublicURL:   strings.TrimRight(strings.TrimSpace(cfg.APIPublicURL), "/"),
		userIPOverride: strings.TrimSpace(cfg.UserIPOverride),
		publicIP:       publicIP,
		clock:          clock,
	}, nil
}

// CheckoutInput starts an iframe checkout for an owner advert + package.
type CheckoutInput struct {
	OwnerUserID uuid.UUID
	AdvertID    int64
	PackageCode domainpackaging.PackageCode
	UserIP      string
}

// CheckoutResult is returned to the FE for iframe embedding.
type CheckoutResult struct {
	ChargeID     uuid.UUID
	MerchantOID  string
	IframeToken  string
	IframeURL    string
	AmountMinor  int64
	CurrencyCode string
	PackageCode  string
	AdvertID     int64
	Status       domainpaytr.ChargeStatus
}

// StartCheckout creates a PENDING charge and fetches a PayTR iframe token.
func (s *Service) StartCheckout(ctx context.Context, in CheckoutInput) (CheckoutResult, error) {
	if !in.PackageCode.Valid() {
		return CheckoutResult{}, apperr.Validation("Geçersiz paket kodu.")
	}
	ip, err := s.resolvePayerIP(ctx, in.UserIP)
	if err != nil {
		return CheckoutResult{}, err
	}

	advert, err := s.adverts.FindByIDForOwner(ctx, in.OwnerUserID, in.AdvertID)
	if err != nil {
		return CheckoutResult{}, err
	}
	if advert.IsDeleted() {
		return CheckoutResult{}, apperr.NotFound("İlan bulunamadı.")
	}
	if !domainadvert.CanOwnerEditDetails(advert.Status) {
		return CheckoutResult{}, apperr.InvalidState("Bu ilan durumunda ödeme başlatılamaz.")
	}

	pkg, err := s.packages.FindByCode(ctx, in.PackageCode)
	if err != nil {
		return CheckoutResult{}, err
	}
	if !pkg.IsActive {
		return CheckoutResult{}, apperr.InvalidState("Seçilen paket aktif değil.")
	}
	if pkg.DisplayPriceAmountMinor == nil || *pkg.DisplayPriceAmountMinor <= 0 {
		return CheckoutResult{}, apperr.InvalidState("Paket ücreti tanımlı değil.")
	}
	amount := *pkg.DisplayPriceAmountMinor

	user, err := s.users.FindByID(ctx, in.OwnerUserID)
	if err != nil {
		return CheckoutResult{}, err
	}
	if !user.IsActive() {
		return CheckoutResult{}, apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
	}

	now := s.clock.Now()
	merchantOID := "hrd" + strings.ReplaceAll(uuid.NewString(), "-", "")
	charge := domainpaytr.Charge{
		ID:           uuid.New(),
		MerchantOID:  merchantOID,
		AdvertID:     in.AdvertID,
		OwnerUserID:  in.OwnerUserID,
		PackageCode:  in.PackageCode,
		AmountMinor:  amount,
		CurrencyCode: "TRY",
		Status:       domainpaytr.ChargeStatusPending,
		UserIP:       &ip,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.charges.Create(ctx, charge); err != nil {
		return CheckoutResult{}, err
	}

	okURL := s.frontendURL + "/post/payment-result?status=ok&advertId=" + fmt.Sprintf("%d", in.AdvertID) + "&merchantOid=" + merchantOID
	failURL := s.frontendURL + "/post/payment-result?status=fail&advertId=" + fmt.Sprintf("%d", in.AdvertID) + "&merchantOid=" + merchantOID
	// Legacy configured notify URL in the PayTR panel only. Sending a localhost
	// notify URL on get-token is useless (PayTR cannot reach it) and can make
	// the request look invalid — only forward publicly reachable HTTPS bases.
	notifyURL := ""
	if isPublicAPIBase(s.apiPublicURL) {
		notifyURL = s.apiPublicURL + "/v1/paytr/notify"
	}

	phone := ""
	if user.Phone != nil {
		phone = strings.TrimSpace(*user.Phone)
	}
	if phone == "" {
		phone = "0000000000"
	}
	address := "Turkiye"
	if advert.Address != nil && strings.TrimSpace(*advert.Address) != "" {
		address = strings.TrimSpace(*advert.Address)
	}
	fullName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if fullName == "" {
		fullName = "Haradan Kullanici"
	}

	tokenRes, err := s.gateway.GetToken(ctx, paytrclient.TokenRequest{
		MerchantOID:       merchantOID,
		UserIP:            ip,
		Email:             user.Email,
		PaymentAmount:     paytrclient.AmountMinorString(amount),
		UserName:          fullName,
		UserAddress:       address,
		UserPhone:         phone,
		MerchantOKURL:     okURL,
		MerchantFailURL:   failURL,
		MerchantNotifyURL: notifyURL,
		BasketTitle:       pkg.DisplayName + " ilan paketi",
		AmountMinor:       amount,
	})
	if err != nil {
		fail := "TOKEN_FAILED"
		msg := err.Error()
		charge.Status = domainpaytr.ChargeStatusFailed
		charge.FailReasonCode = &fail
		charge.FailReasonMsg = &msg
		charge.UpdatedAt = s.clock.Now()
		_ = s.charges.Update(ctx, charge)
		return CheckoutResult{}, apperr.DependencyUnavailable(checkoutUnavailableMessage(err))
	}

	reqJSON := tokenRes.RequestForm.Encode()
	respJSON := tokenRes.RawResponse
	token := tokenRes.Token
	charge.IframeToken = &token
	charge.TokenRequestJSON = &reqJSON
	charge.TokenResponseJSON = &respJSON
	charge.UpdatedAt = s.clock.Now()
	if err := s.charges.Update(ctx, charge); err != nil {
		return CheckoutResult{}, err
	}

	return CheckoutResult{
		ChargeID:     charge.ID,
		MerchantOID:  merchantOID,
		IframeToken:  tokenRes.Token,
		IframeURL:    tokenRes.IframeURL,
		AmountMinor:  amount,
		CurrencyCode: "TRY",
		PackageCode:  string(in.PackageCode),
		AdvertID:     in.AdvertID,
		Status:       domainpaytr.ChargeStatusPending,
	}, nil
}

// NotifyInput is the PayTR server-to-server callback payload.
type NotifyInput struct {
	MerchantOID      string
	Status           string
	TotalAmount      string
	Hash             string
	FailedReasonCode string
	FailedReasonMsg  string
	RawPayloadJSON   string
}

// HandleNotify processes PayTR notification.
// Response bodies match legacy PaymentService.notify:
//   - "OK" on accepted success/failure processing (and unknown merchant_oid)
//   - "PAYTR notification failed: bad hash" when HMAC does not match
// Side-effect failures return "ERR" so PayTR retries (safer than legacy which
// marked payment ACTIVE before doping and could NPE on missing payment).
func (s *Service) HandleNotify(ctx context.Context, in NotifyInput) (string, error) {
	const (
		notifyOK      = "OK"
		notifyBadHash = "PAYTR notification failed: bad hash"
		notifyRetry   = "ERR"
	)

	merchantOID := strings.TrimSpace(in.MerchantOID)
	status := strings.TrimSpace(in.Status)
	totalAmount := strings.TrimSpace(in.TotalAmount)
	hash := strings.TrimSpace(in.Hash)

	if merchantOID == "" || status == "" || totalAmount == "" || hash == "" {
		return notifyBadHash, nil
	}
	if !s.gateway.VerifyNotifyHash(merchantOID, status, totalAmount, hash) {
		return notifyBadHash, nil
	}

	charge, err := s.charges.FindByMerchantOIDForUpdate(ctx, merchantOID)
	if err != nil {
		// Legacy updatePaymentLog no-ops when payment is missing; acknowledge OK
		// so PayTR stops retrying callbacks for unknown/expired oids.
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return notifyOK, nil
		}
		return notifyRetry, err
	}
	if charge.Status == domainpaytr.ChargeStatusSucceeded {
		return notifyOK, nil
	}

	raw := strings.TrimSpace(in.RawPayloadJSON)
	if raw != "" {
		charge.NotifyPayloadJSON = &raw
	}
	now := s.clock.Now()

	if !strings.EqualFold(status, "success") {
		code := strings.TrimSpace(in.FailedReasonCode)
		msg := strings.TrimSpace(in.FailedReasonMsg)
		charge.Status = domainpaytr.ChargeStatusFailed
		if code != "" {
			charge.FailReasonCode = &code
		}
		if msg != "" {
			charge.FailReasonMsg = &msg
		}
		charge.UpdatedAt = now
		_ = s.charges.Update(ctx, charge)
		return notifyOK, nil
	}

	expectedAmount := paytrclient.AmountMinorString(charge.AmountMinor)
	if totalAmount != expectedAmount {
		code := "AMOUNT_MISMATCH"
		msg := "total_amount does not match charge"
		charge.Status = domainpaytr.ChargeStatusFailed
		charge.FailReasonCode = &code
		charge.FailReasonMsg = &msg
		charge.UpdatedAt = now
		_ = s.charges.Update(ctx, charge)
		return notifyOK, nil
	}

	// Side effects before marking SUCCEEDED (legacy marked ACTIVE first; we
	// only commit success after package assign + submit-for-review).
	if _, err := s.packaging.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: charge.OwnerUserID,
		AdvertID:    charge.AdvertID,
		PackageCode: charge.PackageCode,
		Source:      domainpackaging.AssignmentSourcePayment,
	}); err != nil {
		return notifyRetry, err
	}

	advert, err := s.adverts.FindByID(ctx, charge.AdvertID)
	if err != nil {
		return notifyRetry, err
	}
	switch advert.Status {
	case domainadvert.StatusDraft:
		if _, err := s.submitter.SubmitAdvertForReview(ctx, charge.OwnerUserID, charge.AdvertID, advert.Version); err != nil {
			return notifyRetry, err
		}
	case domainadvert.StatusChangesRequested:
		if _, err := s.submitter.ResubmitAdvertForReview(ctx, charge.OwnerUserID, charge.AdvertID, advert.Version); err != nil {
			return notifyRetry, err
		}
	case domainadvert.StatusPendingReview:
		// Already submitted (retry) — ok.
	default:
		// Leave as-is; entitlement was assigned.
	}

	paidAt := now
	submittedAt := now
	charge.Status = domainpaytr.ChargeStatusSucceeded
	charge.PaidAt = &paidAt
	charge.AdvertSubmittedAt = &submittedAt
	charge.UpdatedAt = now
	if err := s.charges.Update(ctx, charge); err != nil {
		return notifyRetry, err
	}
	return notifyOK, nil
}

// GetChargeForOwner returns charge status for FE polling.
func (s *Service) GetChargeForOwner(ctx context.Context, ownerID uuid.UUID, merchantOID string) (domainpaytr.Charge, error) {
	c, err := s.charges.FindByMerchantOID(ctx, strings.TrimSpace(merchantOID))
	if err != nil {
		return domainpaytr.Charge{}, err
	}
	if c.OwnerUserID != ownerID {
		return domainpaytr.Charge{}, apperr.Forbidden(apperr.CodeForbidden, "Bu ödeme kaydına erişim yok.")
	}
	return c, nil
}

// NotifyPayloadMap builds a JSON string from form values for audit storage.
func NotifyPayloadMap(values map[string]string) string {
	raw, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func (s *Service) resolvePayerIP(ctx context.Context, raw string) (string, error) {
	ip := strings.TrimSpace(raw)
	if s.userIPOverride != "" {
		return s.userIPOverride, nil
	}
	if ip == "" {
		return "", apperr.Validation("İstemci IP adresi gerekli.")
	}
	if !isLoopbackIP(ip) {
		return ip, nil
	}
	// PayTR rejects loopback IPs; mirror legacy Utils.getExternalIp(localhost).
	pub, err := s.publicIP(ctx)
	if err != nil || strings.TrimSpace(pub) == "" {
		return "", apperr.DependencyUnavailable("Ödeme için dış IP adresi alınamadı. PAYTR_USER_IP tanımlayın.")
	}
	return strings.TrimSpace(pub), nil
}

func checkoutUnavailableMessage(err error) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "magaza aktif") || strings.Contains(msg, "gecersiz istek") || strings.Contains(msg, "geçersiz istek") {
		return "PayTR mağazası aktif değil veya kimlik bilgileri geçersiz. Mağaza panelini kontrol edin."
	}
	return "Ödeme servisi şu anda kullanılamıyor."
}

func isPublicAPIBase(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := u.Hostname()
	return host != "" && !isLoopbackIP(host)
}

func isLoopbackIP(raw string) bool {
	host := strings.TrimSpace(raw)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

var (
	egressIPMu    sync.Mutex
	egressIPCache string
	egressIPAt    time.Time
)

func lookupEgressIP(ctx context.Context) (string, error) {
	egressIPMu.Lock()
	cached := egressIPCache
	at := egressIPAt
	egressIPMu.Unlock()
	if cached != "" && time.Since(at) < 30*time.Minute {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://checkip.amazonaws.com", nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("egress ip lookup HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("egress ip lookup returned invalid value")
	}
	egressIPMu.Lock()
	egressIPCache = ip
	egressIPAt = time.Now()
	egressIPMu.Unlock()
	return ip, nil
}
