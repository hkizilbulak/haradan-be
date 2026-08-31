package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	domainbanner "github.com/hkizilbulak/haradan-be/internal/domain/banner"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	cataloghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/catalog"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

func (s *Server) SearchPublishedAdverts(c *gin.Context, params generated.SearchPublishedAdvertsParams) {
	if s.advert == nil || !s.publicService().PublicEnabled() {
		respondNotImplemented(c)
		return
	}
	var sort *string
	if params.Sort != nil {
		v := string(*params.Sort)
		sort = &v
	}
	out, err := s.publicService().SearchPublishedAdverts(c.Request.Context(), appadvert.PublicSearchInput{
		Cursor: params.Cursor, Limit: params.Limit, CategoryID: params.CategoryId, ProvinceID: params.ProvinceId,
		DistrictID: params.DistrictId, HorseID: params.HorseId, HasPhoto: params.HasPhoto, Sort: sort,
		PropertyFilters: params.PropertyFilters, ActorUserID: publicActor(c),
	})
	if err != nil {
		respondError(c, s.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapPublicPage(out))
}

func (s *Server) GetPublishedAdvertDetail(c *gin.Context, advertID generated.AdvertIdPath) {
	if s.advert == nil || !s.publicService().PublicEnabled() {
		respondNotImplemented(c)
		return
	}
	out, err := s.publicService().GetPublishedAdvertDetail(c.Request.Context(), advertID, publicActor(c), c.ClientIP())
	if err != nil {
		respondError(c, s.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapPublicDetail(out))
}

func (s *Server) ListHomepageNewAdverts(c *gin.Context, params generated.ListHomepageNewAdvertsParams) {
	if s.advert == nil || !s.publicService().PublicEnabled() {
		respondNotImplemented(c)
		return
	}
	out, err := s.publicService().ListHomepageNewAdverts(c.Request.Context(), params.Cursor, params.Limit, publicActor(c))
	if err != nil {
		respondError(c, s.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapPublicPage(out))
}

func (s *Server) ListHomepageShowcase(c *gin.Context, params generated.ListHomepageShowcaseParams) {
	if s.advert == nil || !s.publicService().PublicEnabled() {
		respondNotImplemented(c)
		return
	}
	out, err := s.publicService().ListHomepageShowcase(c.Request.Context(), params.Seed, params.Limit, publicActor(c))
	if err != nil {
		respondError(c, s.logger, err)
		return
	}
	items := make([]publicCardJSON, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, mapPublicCardJSON(item))
	}
	c.JSON(http.StatusOK, homepageShowcaseJSON{Items: items, Seed: out.Seed})
}

func (s *Server) ListHomepageUrgent(c *gin.Context, params generated.ListHomepageUrgentParams) {
	if s.advert == nil || !s.publicService().PublicEnabled() {
		respondNotImplemented(c)
		return
	}
	out, err := s.publicService().ListHomepageUrgent(c.Request.Context(), params.Limit, publicActor(c))
	if err != nil {
		respondError(c, s.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapPublicPage(out))
}

func (s *Server) ListHomepageFeatured(c *gin.Context, params generated.ListHomepageFeaturedParams) {
	if s.advert == nil || !s.publicService().PublicEnabled() {
		respondNotImplemented(c)
		return
	}
	out, err := s.publicService().ListHomepageFeatured(c.Request.Context(), params.Limit, publicActor(c))
	if err != nil {
		respondError(c, s.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapPublicPage(out))
}

func (s *Server) GetHomepageBootstrap(c *gin.Context, params generated.GetHomepageBootstrapParams) {
	if s.advert == nil || !s.publicService().PublicEnabled() {
		respondNotImplemented(c)
		return
	}

	ctx := c.Request.Context()
	actor := publicActor(c)

	feeds, err := s.publicService().GetHomepageBootstrap(ctx, params.Limit, actor)
	if err != nil {
		respondError(c, s.logger, err)
		return
	}

	bannerItems := make([]generated.ActiveBannerItem, 0)
	if s.banner != nil {
		placements := []domainbanner.Placement{
			domainbanner.PlacementHomepageHero,
			domainbanner.PlacementHomepagePromo,
			domainbanner.PlacementHomepage,
		}
		for _, p := range placements {
			items, e := s.banner.Service().ListActiveBannersByPlacement(ctx, p)
			if e != nil {
				continue
			}
			mapped, e := s.banner.BuildActiveBannerItems(ctx, items)
			if e != nil {
				continue
			}
			bannerItems = append(bannerItems, mapped...)
		}
	}

	categories := generated.CategoryTreeResponse{Items: []generated.CategoryTreeNode{}}
	if s.catalog != nil {
		tree, e := s.catalog.Service().GetPublicCategoryTree(ctx)
		if e == nil {
			categories.Items = cataloghandler.MapPublicCategoryTree(tree)
		}
	}

	c.JSON(http.StatusOK, generated.HomepageBootstrapResponse{
		NewAdverts: mapGeneratedSearchPage(feeds.NewAdverts),
		Urgent:     mapGeneratedSearchPage(feeds.Urgent),
		Featured:   mapGeneratedSearchPage(feeds.Featured),
		Showcase: generated.HomepageShowcaseResponse{
			Seed:  feeds.Showcase.Seed,
			Items: mapGeneratedCards(feeds.Showcase.Items),
		},
		Banners:    generated.ActiveBannerListResponse{Items: bannerItems},
		Categories: categories,
	})
}

func mapGeneratedSearchPage(v appadvert.PublicSearchResult) generated.PublishedAdvertSearchResponse {
	items := make([]generated.PublishedAdvertCard, 0, len(v.Items))
	for _, item := range v.Items {
		items = append(items, mapPublicCard(item))
	}
	return generated.PublishedAdvertSearchResponse{Items: items, HasMore: v.HasMore, NextCursor: v.NextCursor}
}

func mapGeneratedCards(items []domainadvert.PublicCard) []generated.PublishedAdvertCard {
	out := make([]generated.PublishedAdvertCard, 0, len(items))
	for _, item := range items {
		out = append(out, mapPublicCard(item))
	}
	return out
}

// Server owns the application service already; this small accessor keeps the
// public transport in the root handler without widening child handler APIs.
func (s *Server) publicService() *appadvert.Service { return s.advert.Service() }

func publicActor(c *gin.Context) *uuid.UUID {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		return nil
	}
	id := p.UserID
	return &id
}

type publicCardJSON struct {
	generated.PublishedAdvertCard
	Properties map[string]any `json:"properties,omitempty"`
}

type publicSearchPageJSON struct {
	Items      []publicCardJSON `json:"items"`
	NextCursor *string          `json:"nextCursor,omitempty"`
	HasMore    bool             `json:"hasMore"`
}

type homepageShowcaseJSON struct {
	Items []publicCardJSON `json:"items"`
	Seed  string           `json:"seed"`
}

func mapPublicCardJSON(v domainadvert.PublicCard) publicCardJSON {
	return publicCardJSON{
		PublishedAdvertCard: mapPublicCard(v),
		Properties:          v.Properties,
	}
}

func mapPublicPage(v appadvert.PublicSearchResult) publicSearchPageJSON {
	items := make([]publicCardJSON, 0, len(v.Items))
	for _, item := range v.Items {
		items = append(items, mapPublicCardJSON(item))
	}
	return publicSearchPageJSON{Items: items, HasMore: v.HasMore, NextCursor: v.NextCursor}
}
func mapPublicCard(v domainadvert.PublicCard) generated.PublishedAdvertCard {
	card := generated.PublishedAdvertCard{Id: v.ID, CategoryId: v.CategoryID, DistrictId: v.DistrictID, ProvinceId: v.ProvinceID,
		HorseId: v.HorseID, Title: v.Title, Price: mapPublicMoney(v.Price), PublishedAt: v.PublishedAt, Cover: mapPublicMedia(v.Cover),
		PackageCode: mapPackageCode(v.PackageCode), PackageDisplayName: v.PackageDisplayName, PackageBadgeText: v.PackageBadgeText,
		IsUrgent: v.IsUrgent, UrgentActivatedAt: v.UrgentActivatedAt, IsFeatured: v.IsFeatured, FeaturedUntil: v.FeaturedUntil,
		IsFavorite: v.IsFavorite, ViewCount: v.ViewCount}
	if v.DistrictName != "" {
		name := v.DistrictName
		card.DistrictName = &name
	}
	if v.ProvinceName != "" {
		name := v.ProvinceName
		card.ProvinceName = &name
	}
	return card
}

type publicDetailJSON struct {
	generated.PublishedAdvertDetailResponse
	SellerPhone *string    `json:"sellerPhone,omitempty"`
	SellerID    *uuid.UUID `json:"sellerId,omitempty"`
}

func mapPublicDetail(v domainadvert.PublicDetail) publicDetailJSON {
	media := make([]generated.PublicMediaItem, 0, len(v.Media))
	for i := range v.Media {
		m := v.Media[i]
		media = append(media, *mapPublicMedia(&m))
	}
	props := make([]generated.PublicPropertyValue, 0, len(v.Properties))
	for _, p := range v.Properties {
		props = append(props, generated.PublicPropertyValue{Code: p.Code, Title: p.Title, Value: p.Value, DisplayValue: p.DisplayValue})
	}
	var horse *generated.HorseSelectionItem
	if v.Horse != nil {
		horse = &generated.HorseSelectionItem{Id: v.Horse.ID, OriginalName: v.Horse.Name, TjkNumber: stringValue(v.Horse.TJKNumber)}
	}
	return publicDetailJSON{
		PublishedAdvertDetailResponse: generated.PublishedAdvertDetailResponse{
			Id: v.ID, Title: v.Title, Description: v.Description, Price: mapPublicMoney(v.Price), PublishedAt: v.PublishedAt,
			Category: generated.PublicCategorySummary{Id: v.CategoryID, Name: v.CategoryName, Slug: v.CategorySlug},
			Location: generated.PublicLocationSummary{DistrictId: v.DistrictID, DistrictName: v.DistrictName, ProvinceId: v.ProvinceID, ProvinceName: v.ProvinceName},
			Horse:    horse, Media: media, Properties: props, PackageCode: mapPackageCode(v.PackageCode), PackageDisplayName: v.PackageDisplayName,
			PackageBadgeText: v.PackageBadgeText, IsUrgent: v.IsUrgent, UrgentActivatedAt: v.UrgentActivatedAt,
			IsFeatured: v.IsFeatured, FeaturedUntil: v.FeaturedUntil, IsFavorite: v.IsFavorite, ViewCount: v.ViewCount,
		},
		SellerPhone: v.SellerPhone,
		SellerID:    v.SellerID,
	}
}
func mapPublicMedia(v *domainadvert.PublicMedia) *generated.PublicMediaItem {
	if v == nil {
		return nil
	}
	profile := domainmedia.ProfileDetail
	if v.Usage != nil && domainmedia.IsKnownDeliveryProfile(*v.Usage) {
		profile = *v.Usage
	}
	url := domainmedia.PublicDeliveryURL(v.AssetID, profile)
	return &generated.PublicMediaItem{AssetId: v.AssetID, DisplayOrder: v.DisplayOrder, IsCover: v.IsCover, PublicUrl: url, Usage: v.Usage}
}
func mapPublicMoney(v *domainadvert.Money) *generated.Money {
	if v == nil {
		return nil
	}
	return &generated.Money{AmountMinor: int(v.AmountMinor), Currency: v.Currency}
}
func mapPackageCode(v *string) *generated.PackageCode {
	if v == nil {
		return nil
	}
	out := generated.PackageCode(*v)
	return &out
}
func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
