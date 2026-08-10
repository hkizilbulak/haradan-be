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

// CreateAdminPackage implements packaging admin create.
func (s *Server) CreateAdminPackage(c *gin.Context) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.CreateAdminPackage(c)
}

// ReorderPackages implements packaging admin reorder.
func (s *Server) ReorderPackages(c *gin.Context) {
	if s.packaging == nil {
		respondNotImplemented(c)
		return
	}
	s.packaging.ReorderPackages(c)
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

// ListAdminProviderEmailTemplates implements provider email template list.
func (s *Server) ListAdminProviderEmailTemplates(c *gin.Context) {
	if s.emailtpl == nil {
		s.respondDependencyUnavailable(c, "E-posta sağlayıcı yapılandırılmamış.")
		return
	}
	s.emailtpl.ListAdminProviderEmailTemplates(c)
}

// GetAdminProviderEmailTemplateVariables implements provider template variables.
func (s *Server) GetAdminProviderEmailTemplateVariables(c *gin.Context, templateId generated.ProviderEmailTemplateIdPath) {
	if s.emailtpl == nil {
		s.respondDependencyUnavailable(c, "E-posta sağlayıcı yapılandırılmamış.")
		return
	}
	s.emailtpl.GetAdminProviderEmailTemplateVariables(c, templateId)
}

// ListAdminJobs implements job admin list.
func (s *Server) ListAdminJobs(c *gin.Context) {
	if s.jobadmin == nil {
		s.respondDependencyUnavailable(c, "İş tanımı servisi henüz hazır değil.")
		return
	}
	s.jobadmin.ListAdminJobs(c)
}

// GetAdminJob implements job admin get.
func (s *Server) GetAdminJob(c *gin.Context, jobId generated.JobIdPath) {
	if s.jobadmin == nil {
		s.respondDependencyUnavailable(c, "İş tanımı servisi henüz hazır değil.")
		return
	}
	s.jobadmin.GetAdminJob(c, jobId)
}

// UpdateAdminJob implements job admin update.
func (s *Server) UpdateAdminJob(c *gin.Context, jobId generated.JobIdPath) {
	if s.jobadmin == nil {
		s.respondDependencyUnavailable(c, "İş tanımı servisi henüz hazır değil.")
		return
	}
	s.jobadmin.UpdateAdminJob(c, jobId)
}

// RunAdminJob implements job admin manual run.
func (s *Server) RunAdminJob(c *gin.Context, jobId generated.JobIdPath) {
	if s.jobadmin == nil {
		s.respondDependencyUnavailable(c, "İş tanımı servisi henüz hazır değil.")
		return
	}
	s.jobadmin.RunAdminJob(c, jobId)
}

// ListAdminJobHistory implements job admin history.
func (s *Server) ListAdminJobHistory(c *gin.Context, jobId generated.JobIdPath, params generated.ListAdminJobHistoryParams) {
	if s.jobadmin == nil {
		s.respondDependencyUnavailable(c, "İş tanımı servisi henüz hazır değil.")
		return
	}
	s.jobadmin.ListAdminJobHistory(c, jobId, params)
}
