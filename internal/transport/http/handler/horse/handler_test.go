package horse

import (
	"context"
	"testing"
	"time"
)

func TestLiveTJKResolution(t *testing.T) {
	h := &Handler{}

	testCases := []string{"HAZARFEN", "AĞA KARACA", "TURBO"}
	for _, name := range testCases {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		id, url := h.resolveHorseAtID(ctx, name)
		cancel()
		t.Logf("Horse '%s' => id: '%s', url: '%s'", name, id, url)
		if id == "" {
			t.Errorf("expected id for '%s', got empty", name)
		}
	}
}
