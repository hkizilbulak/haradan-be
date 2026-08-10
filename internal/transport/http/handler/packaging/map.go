package packaging

import (
	"encoding/json"

	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

func mapPackageAdminView(p domainpackaging.Package) generated.PackageAdminView {
	return generated.PackageAdminView{
		Id:                  p.ID,
		Code:                generated.PackageCode(p.Code),
		DisplayName:         p.DisplayName,
		Description:         p.Description,
		BadgeText:           p.BadgeText,
		Benefits:            mapBenefits(p.BenefitsJSON),
		DisplayPrice:        mapDisplayPrice(p.DisplayPriceAmountMinor, p.CurrencyCode),
		CurrencyCode:        p.CurrencyCode,
		DefaultDurationDays: p.DefaultDurationDays,
		AllowsUrgent:        p.AllowsUrgent,
		ShowcaseEligible:    p.ShowcaseEligible,
		SearchPriority:      p.SearchPriority,
		BroadcastOnPublish:  p.BroadcastOnPublish,
		IsActive:            p.IsActive,
		SortOrder:           p.SortOrder,
		Version:             p.Version,
		CreatedAt:           p.CreatedAt,
		UpdatedAt:           p.UpdatedAt,
	}
}

func mapPublicPackage(p domainpackaging.Package) generated.PublicPackage {
	return generated.PublicPackage{
		Code:                generated.PackageCode(p.Code),
		DisplayName:         p.DisplayName,
		Description:         p.Description,
		BadgeText:           p.BadgeText,
		Benefits:            mapBenefits(p.BenefitsJSON),
		DisplayPrice:        mapDisplayPrice(p.DisplayPriceAmountMinor, p.CurrencyCode),
		DefaultDurationDays: p.DefaultDurationDays,
		AllowsUrgent:        p.AllowsUrgent,
		ShowcaseEligible:    p.ShowcaseEligible,
		SearchPriority:      p.SearchPriority,
		SortOrder:           p.SortOrder,
	}
}

func mapBenefits(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

func mapDisplayPrice(amount *int64, currency string) *generated.Money {
	if amount == nil {
		return nil
	}
	return &generated.Money{AmountMinor: int(*amount), Currency: currency}
}

func mapAssignmentView(v apppackaging.AssignmentView) generated.AdvertPackageAssignmentView {
	a := v.Assignment
	return generated.AdvertPackageAssignmentView{
		Id:               a.ID,
		AdvertId:         a.AdvertID,
		PackageCode:      generated.PackageCode(v.Package.Code),
		Status:           generated.PackageAssignmentStatus(a.Status),
		StartsAt:         a.StartsAt,
		EndsAt:           a.EndsAt,
		AssignedByUserId: a.AssignedByUserID,
		AssignedAt:       a.AssignedAt,
		SupersededAt:     a.SupersededAt,
		ExpiredAt:        a.ExpiredAt,
		CancelledAt:      a.CancelledAt,
		Reason:           a.Reason,
		Source:           generated.PackageAssignmentSource(a.Source),
		Version:          a.Version,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

func mapHistoryItem(v apppackaging.AssignmentView) generated.AdvertPackageHistoryItem {
	a := v.Assignment
	return generated.AdvertPackageHistoryItem{
		Id:               a.ID,
		AdvertId:         a.AdvertID,
		PackageCode:      generated.PackageCode(v.Package.Code),
		Status:           generated.PackageAssignmentStatus(a.Status),
		StartsAt:         a.StartsAt,
		EndsAt:           a.EndsAt,
		AssignedByUserId: a.AssignedByUserID,
		AssignedAt:       a.AssignedAt,
		SupersededAt:     a.SupersededAt,
		ExpiredAt:        a.ExpiredAt,
		CancelledAt:      a.CancelledAt,
		Reason:           a.Reason,
		Source:           generated.PackageAssignmentSource(a.Source),
		Version:          a.Version,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

func mapUrgentView(a domainpackaging.AdvertFeatureActivation) generated.AdvertUrgentActivationView {
	return generated.AdvertUrgentActivationView{
		Id:                  a.ID,
		AdvertId:            a.AdvertID,
		PackageAssignmentId: a.PackageAssignmentID,
		FeatureCode:         generated.AdvertUrgentActivationViewFeatureCode(a.FeatureCode),
		Status:              generated.AdvertUrgentActivationViewStatus(a.Status),
		ActivatedByUserId:   a.ActivatedByUserID,
		ActivatedAt:         a.ActivatedAt,
		DeactivatedAt:       a.DeactivatedAt,
		Reason:              a.Reason,
		ActivationVersion:   a.ActivationVersion,
		CreatedAt:           a.CreatedAt,
		UpdatedAt:           a.UpdatedAt,
	}
}
