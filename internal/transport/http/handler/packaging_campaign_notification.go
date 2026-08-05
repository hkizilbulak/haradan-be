package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

func (s *Server) ListPublicPackages(c *gin.Context) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.ListPublicPackages(c)
}

// ListAdminPackages implements packaging admin list.
func (s *Server) ListAdminPackages(c *gin.Context) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.ListAdminPackages(c)
}

// GetAdminPackage implements packaging admin get-by-code.
func (s *Server) GetAdminPackage(c *gin.Context, packageCode generated.PackageCodePath) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.GetAdminPackage(c, packageCode)
}

// UpdateAdminPackage implements packaging admin update.
func (s *Server) UpdateAdminPackage(c *gin.Context, packageCode generated.PackageCodePath) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.UpdateAdminPackage(c, packageCode)
}

// GetAdminAdvertPackage implements admin advert package get.
func (s *Server) GetAdminAdvertPackage(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.GetAdminAdvertPackage(c, advertId)
}

// AssignAdminAdvertPackage implements admin advert package assign.
func (s *Server) AssignAdminAdvertPackage(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.AssignAdminAdvertPackage(c, advertId)
}

// ListAdminAdvertPackageHistory implements admin advert package history.
func (s *Server) ListAdminAdvertPackageHistory(
	c *gin.Context,
	advertId generated.AdvertIdPath,
	params generated.ListAdminAdvertPackageHistoryParams,
) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.ListAdminAdvertPackageHistory(c, advertId, params)
}

// CancelAdminAdvertPackage implements admin advert package cancel.
func (s *Server) CancelAdminAdvertPackage(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.CancelAdminAdvertPackage(c, advertId)
}

// ActivateAdvertUrgent implements URGENT activate.
func (s *Server) ActivateAdvertUrgent(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.ActivateAdvertUrgent(c, advertId)
}

// DeactivateAdvertUrgent implements URGENT deactivate.
func (s *Server) DeactivateAdvertUrgent(c *gin.Context, advertId generated.AdvertIdPath) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.DeactivateAdvertUrgent(c, advertId)
}

// ListAdminCampaigns implements campaign admin list.
func (s *Server) ListAdminCampaigns(c *gin.Context, params generated.ListAdminCampaignsParams) {
	if s.campaign == nil {
		respondNotImplemented(c)
		return
	}
	s.campaign.ListAdminCampaigns(c, params)
}

// CreateAdminCampaign implements campaign admin create.
func (s *Server) CreateAdminCampaign(c *gin.Context) {
	if s.campaign == nil {
		respondNotImplemented(c)
		return
	}
	s.campaign.CreateAdminCampaign(c)
}

// GetAdminCampaign implements campaign admin get.
func (s *Server) GetAdminCampaign(c *gin.Context, campaignId generated.CampaignIdPath) {
	if s.campaign == nil {
		respondNotImplemented(c)
		return
	}
	s.campaign.GetAdminCampaign(c, campaignId)
}

// UpdateAdminCampaign implements campaign admin update.
func (s *Server) UpdateAdminCampaign(c *gin.Context, campaignId generated.CampaignIdPath) {
	if s.campaign == nil {
		respondNotImplemented(c)
		return
	}
	s.campaign.UpdateAdminCampaign(c, campaignId)
}

// ListAdminNotificationTemplates implements notification template admin list.
func (s *Server) ListAdminNotificationTemplates(c *gin.Context) {
	if s.notification == nil {
		respondNotImplemented(c)
		return
	}
	s.notification.ListAdminNotificationTemplates(c)
}

// GetAdminNotificationTemplate implements notification template admin get.
func (s *Server) GetAdminNotificationTemplate(c *gin.Context, eventType generated.NotificationEventTypePath) {
	if s.notification == nil {
		respondNotImplemented(c)
		return
	}
	s.notification.GetAdminNotificationTemplate(c, eventType)
}

// UpdateAdminNotificationTemplate implements notification template admin update.
func (s *Server) UpdateAdminNotificationTemplate(c *gin.Context, eventType generated.NotificationEventTypePath) {
	if s.notification == nil {
		respondNotImplemented(c)
		return
	}
	s.notification.UpdateAdminNotificationTemplate(c, eventType)
}
