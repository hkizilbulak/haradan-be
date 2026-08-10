package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware"
)

// NotImplementedServer temporarily satisfies generated.ServerInterface with HTTP 501.
type NotImplementedServer struct{}

func respondNotImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, generated.ErrorResponse{
		Code:    generated.DomainErrorCodeINTERNALERROR,
		Message: "Bu işlem henüz uygulanmadı.",
		TraceId: middleware.RequestIDFromContext(c.Request.Context()),
	})
}

func (NotImplementedServer) GetHealth(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListBannersAdmin(c *gin.Context, params generated.ListBannersAdminParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) CreateBanner(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ReorderBanners(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetBannerAdminDetail(c *gin.Context, bannerId generated.BannerIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateBanner(c *gin.Context, bannerId generated.BannerIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) SetBannerStatus(c *gin.Context, bannerId generated.BannerIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListCategoriesAdmin(c *gin.Context, params generated.ListCategoriesAdminParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) CreateCategory(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ReorderCategories(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetCategoryAdminDetail(c *gin.Context, categoryId generated.CategoryIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateCategory(c *gin.Context, categoryId generated.CategoryIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) SetCategoryActive(c *gin.Context, categoryId generated.CategoryIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListCategoryPropertiesAdmin(c *gin.Context, categoryId generated.CategoryIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) CreateCategoryProperty(c *gin.Context, categoryId generated.CategoryIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ReorderCategoryProperties(c *gin.Context, categoryId generated.CategoryIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateCategoryProperty(c *gin.Context, categoryId generated.CategoryIdPath, propertyId generated.PropertyIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) SetCategoryPropertyActive(c *gin.Context, categoryId generated.CategoryIdPath, propertyId generated.PropertyIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ReparentCategory(c *gin.Context, categoryId generated.CategoryIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetAdminMediaProcessingStatus(c *gin.Context, assetId generated.AssetIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ConfirmAdminMediaUpload(c *gin.Context, assetId generated.AssetIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) InitiateAdminMediaUpload(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) IgnoreTJKSyncItemError(c *gin.Context, errorId generated.ErrorIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ResolveTJKSyncItemError(c *gin.Context, errorId generated.ErrorIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListTJKSyncRuns(c *gin.Context, params generated.ListTJKSyncRunsParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) TriggerTJKSync(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetTJKSyncRun(c *gin.Context, runId generated.RunIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) CancelTJKSync(c *gin.Context, runId generated.RunIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListTJKSyncItemErrors(c *gin.Context, runId generated.RunIdPath, params generated.ListTJKSyncItemErrorsParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListUsers(c *gin.Context, params generated.ListUsersParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) CreateAdminUser(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ResendAdminUserInvitation(c *gin.Context, userId generated.UserIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateAdminUser(c *gin.Context, userId generated.UserIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) RequestAdminUserEmailChange(c *gin.Context, userId generated.UserIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetUserAdminDetail(c *gin.Context, userId generated.UserIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ChangeUserRole(c *gin.Context, userId generated.UserIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListUserSecurityEvents(c *gin.Context, userId generated.UserIdPath, params generated.ListUserSecurityEventsParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ChangeUserStatus(c *gin.Context, userId generated.UserIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) SearchPublishedAdverts(c *gin.Context, params generated.SearchPublishedAdvertsParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetPublishedAdvertDetail(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ConfirmEmailChange(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) Login(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) LogoutCurrentSession(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) LogoutAllSessions(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) RequestPasswordReset(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ResetPassword(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) RefreshSession(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) RegisterUser(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ResendRegistrationEmailVerification(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) VerifyRegistrationEmail(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListActiveBannersByPlacement(c *gin.Context, params generated.ListActiveBannersByPlacementParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) SearchHorsesForSelection(c *gin.Context, params generated.SearchHorsesForSelectionParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetHorsePublicDetail(c *gin.Context, horseId generated.HorseIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetMyProfile(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateMyProfile(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListMyAdverts(c *gin.Context, params generated.ListMyAdvertsParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) CreateAdvertDraft(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) SoftDeleteAdvertDraft(c *gin.Context, advertId generated.AdvertIdPath, params generated.SoftDeleteAdvertDraftParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetMyAdvert(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateAdvertDraftDetails(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ArchiveAdvert(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ChangeAdvertDraftCategory(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) AttachMediaToAdvert(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) SetAdvertCover(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ReorderAdvertMedia(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) DetachMediaFromAdvert(c *gin.Context, advertId generated.AdvertIdPath, assetId generated.AssetIdPath, params generated.DetachMediaFromAdvertParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ReplaceAdvertDynamicProperties(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ResubmitAdvertForReview(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) MarkAdvertSold(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) SubmitAdvertForReview(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) RequestEmailChange(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListMyFavorites(c *gin.Context, params generated.ListMyFavoritesParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) RemoveFavorite(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) AddFavorite(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ChangePassword(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListMySessions(c *gin.Context, params generated.ListMySessionsParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) RevokeMySession(c *gin.Context, sessionId generated.SessionIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetMediaProcessingStatus(c *gin.Context, assetId generated.AssetIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ConfirmMediaUpload(c *gin.Context, assetId generated.AssetIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) InitiateMediaUpload(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListAdminPackages(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetAdminPackage(c *gin.Context, packageCode generated.PackageCodePath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateAdminPackage(c *gin.Context, packageCode generated.PackageCodePath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetAdminAdvertPackage(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) AssignAdminAdvertPackage(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListAdminAdvertPackageHistory(c *gin.Context, advertId generated.AdvertIdPath, params generated.ListAdminAdvertPackageHistoryParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) CancelAdminAdvertPackage(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ActivateAdvertUrgent(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) DeactivateAdvertUrgent(c *gin.Context, advertId generated.AdvertIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListAdminCampaigns(c *gin.Context, params generated.ListAdminCampaignsParams) {
	respondNotImplemented(c)
}

func (NotImplementedServer) CreateAdminCampaign(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetAdminCampaign(c *gin.Context, campaignId generated.CampaignIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateAdminCampaign(c *gin.Context, campaignId generated.CampaignIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListAdminNotificationTemplates(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetAdminNotificationTemplate(c *gin.Context, eventType generated.NotificationEventTypePath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateAdminNotificationTemplate(c *gin.Context, eventType generated.NotificationEventTypePath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) CreateAdminPackage(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ReorderPackages(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetPublicMedia(c *gin.Context, assetId generated.AssetIdPath, profile generated.MediaDeliveryProfile) {
	respondNotImplemented(c)
}

func (NotImplementedServer) HeadPublicMedia(c *gin.Context, assetId generated.AssetIdPath, profile generated.MediaDeliveryProfile) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListAdminProviderEmailTemplates(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetAdminProviderEmailTemplateVariables(c *gin.Context, templateId generated.ProviderEmailTemplateIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListAdminJobs(c *gin.Context) {
	respondNotImplemented(c)
}

func (NotImplementedServer) GetAdminJob(c *gin.Context, jobId generated.JobIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) UpdateAdminJob(c *gin.Context, jobId generated.JobIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) RunAdminJob(c *gin.Context, jobId generated.JobIdPath) {
	respondNotImplemented(c)
}

func (NotImplementedServer) ListAdminJobHistory(c *gin.Context, jobId generated.JobIdPath, params generated.ListAdminJobHistoryParams) {
	respondNotImplemented(c)
}
