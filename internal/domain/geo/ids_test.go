package geo_test

import (
	"testing"

	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
)

func TestStableIDsAreDeterministicAndDistinct(t *testing.T) {
	p34 := domaingeo.StableProvinceID(34)
	if p34 != domaingeo.StableProvinceID(34) {
		t.Fatal("province UUID must be stable")
	}
	if p34 == domaingeo.StableProvinceID(6) {
		t.Fatal("different plates must not share an id")
	}
	d := domaingeo.StableDistrictID(1103)
	if d != domaingeo.StableDistrictID(1103) {
		t.Fatal("district UUID must be stable")
	}
	if d == p34 {
		t.Fatal("province and district ids must not collide")
	}
}
