package worker

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEffectiveJobTimeoutUsesPayloadCappedByMax(t *testing.T) {
	defaultTimeout := 60 * time.Second
	maxCeiling := 2 * time.Hour

	payload, _ := json.Marshal(map[string]int{"timeoutSeconds": 1800})
	got := effectiveJobTimeout(defaultTimeout, maxCeiling, payload)
	if got != 1800*time.Second {
		t.Fatalf("expected 1800s from payload, got %v", got)
	}

	payload, _ = json.Marshal(map[string]int{"timeoutSeconds": 7200})
	got = effectiveJobTimeout(defaultTimeout, maxCeiling, payload)
	if got != maxCeiling {
		t.Fatalf("expected clamp to maxCeiling, got %v", got)
	}
}

func TestEffectiveJobTimeoutFallsBackToDefault(t *testing.T) {
	defaultTimeout := 60 * time.Second
	maxCeiling := 2 * time.Hour

	cases := [][]byte{
		nil,
		[]byte(`{}`),
		[]byte(`{"timeoutSeconds":0}`),
		[]byte(`{"timeoutSeconds":-1}`),
		[]byte(`not-json`),
	}
	for _, payload := range cases {
		got := effectiveJobTimeout(defaultTimeout, maxCeiling, payload)
		if got != defaultTimeout {
			t.Fatalf("payload=%s: got %v want %v", payload, got, defaultTimeout)
		}
	}
}

func TestEffectiveJobTimeoutNeverNonPositive(t *testing.T) {
	got := effectiveJobTimeout(0, 0, nil)
	if got <= 0 {
		t.Fatalf("expected positive fallback, got %v", got)
	}
	got = effectiveJobTimeout(0, 5*time.Second, nil)
	if got != 5*time.Second {
		t.Fatalf("expected maxCeiling fallback, got %v", got)
	}
}
