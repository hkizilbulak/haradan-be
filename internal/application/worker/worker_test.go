package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

func TestBackoffExponentialAndCap(t *testing.T) {
	t.Parallel()
	b := Backoff{
		Base:    5 * time.Second,
		Max:     40 * time.Second,
		Float63: func() float64 { return 1 }, // jitter multiplier 1.0 → full delay
	}
	d1 := b.Delay(1)
	d2 := b.Delay(2)
	d3 := b.Delay(3)
	if d1 != 5*time.Second {
		t.Fatalf("d1=%v", d1)
	}
	if d2 != 10*time.Second {
		t.Fatalf("d2=%v", d2)
	}
	if d3 != 20*time.Second {
		t.Fatalf("d3=%v", d3)
	}
	dBig := b.Delay(100)
	if dBig != 40*time.Second {
		t.Fatalf("cap=%v", dBig)
	}
}

func TestClassifyErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		down bool
		kind outcomeKind
	}{
		{"nil", nil, false, outcomeSuccess},
		{"validation", apperr.Validation("bad"), false, outcomePermanentFail},
		{"badrequest", apperr.BadRequest(apperr.CodeValidation, "bad"), false, outcomePermanentFail},
		{"notfound", apperr.NotFound("missing"), false, outcomePermanentFail},
		{"invalidstate", apperr.InvalidState("state"), false, outcomePermanentFail},
		{"dep", apperr.DependencyUnavailable("down"), false, outcomeTransientRetry},
		{"internal", apperr.Internal(errors.New("x")), false, outcomeTransientRetry},
		{"deadline", context.DeadlineExceeded, false, outcomeTransientRetry},
		{"shutdown cancel", context.Canceled, true, outcomeShutdownRequeue},
		{"shutdown deadline", context.DeadlineExceeded, true, outcomeShutdownRequeue},
		{"plain cancel", context.Canceled, false, outcomeTransientRetry},
		{"unknown", errors.New("raw"), false, outcomeTransientRetry},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyProcessError(tc.err, tc.down)
			if got.Kind != tc.kind {
				t.Fatalf("kind=%v want %v last=%q", got.Kind, tc.kind, got.LastError)
			}
			if tc.err != nil && got.LastError == "" {
				t.Fatal("last error empty")
			}
			if got.LastError == "raw" {
				t.Fatal("raw error leaked")
			}
			if !tc.down && errors.Is(tc.err, context.Canceled) && got.LastError == safeShutdownErrorMessage {
				t.Fatal("non-shutdown cancel should not use shutdown message")
			}
		})
	}
}

func TestSanitizeLastErrorUTF8(t *testing.T) {
	t.Parallel()
	long := ""
	for len(long) < maxLastErrorLen+10 {
		long += "ğ"
	}
	out := sanitizeLastError(long, "fallback")
	if len(out) > maxLastErrorLen {
		t.Fatalf("len=%d", len(out))
	}
	if out == "" {
		t.Fatal("empty")
	}
}

func TestParseMediaJob(t *testing.T) {
	t.Parallel()
	assetID := uuid.New()
	validValidate, _ := json.Marshal(map[string]string{"assetId": assetID.String()})
	p, err := parseMediaJob(domainmedia.JobValidateAndNormalize, validValidate)
	if err != nil || p.AssetID != assetID {
		t.Fatalf("validate parse: %+v err=%v", p, err)
	}

	validVariant, _ := json.Marshal(map[string]string{
		"assetId": assetID.String(), "transformProfile": domainmedia.ProfileDetail,
	})
	p, err = parseMediaJob(domainmedia.JobGenerateVariant, validVariant)
	if err != nil || p.Profile != domainmedia.ProfileDetail {
		t.Fatalf("variant parse: %+v err=%v", p, err)
	}

	badCases := []struct {
		name    string
		jobType domainmedia.JobType
		raw     string
	}{
		{"missing asset", domainmedia.JobValidateAndNormalize, `{}`},
		{"invalid uuid", domainmedia.JobValidateAndNormalize, `{"assetId":"nope"}`},
		{"unknown field", domainmedia.JobValidateAndNormalize, `{"assetId":"` + assetID.String() + `","extra":1}`},
		{"missing profile", domainmedia.JobGenerateVariant, `{"assetId":"` + assetID.String() + `"}`},
		{"unknown profile", domainmedia.JobGenerateVariant, `{"assetId":"` + assetID.String() + `","transformProfile":"X"}`},
		{"malformed", domainmedia.JobValidateAndNormalize, `{`},
		{"trailing json", domainmedia.JobValidateAndNormalize, `{"assetId":"` + assetID.String() + `"}{"x":1}`},
		{"unsupported", domainmedia.JobDeleteObjects, `{"assetId":"` + assetID.String() + `"}`},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseMediaJob(tc.jobType, json.RawMessage(tc.raw))
			if err == nil {
				t.Fatal("expected error")
			}
			if ae, ok := apperr.As(err); !ok || (ae.Kind != apperr.KindValidation && ae.Kind != apperr.KindBadRequest) {
				t.Fatalf("want permanent validation-class, got %v", err)
			}
		})
	}
}
