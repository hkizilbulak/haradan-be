package packaging

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
)

func TestDecodeUpdatePackageNullClears(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"expectedVersion":3,"description":null,"badgeText":null,"displayPrice":null,"defaultDurationDays":null}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("PATCH", "/", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	in, err := decodeUpdatePackageInput(c, uuid.New(), domainpackaging.PackageCodeStarter)
	if err != nil {
		t.Fatal(err)
	}
	if !in.DescriptionSet || in.Description != nil {
		t.Fatalf("description clear: set=%v val=%v", in.DescriptionSet, in.Description)
	}
	if !in.BadgeTextSet || in.BadgeText != nil {
		t.Fatalf("badge clear")
	}
	if !in.DisplayPriceSet || in.DisplayPriceAmountMinor != nil {
		t.Fatalf("price clear")
	}
	if !in.DefaultDurationSet || in.DefaultDurationDays != nil {
		t.Fatalf("duration clear")
	}
}
