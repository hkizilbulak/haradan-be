package comment_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appcomment "github.com/hkizilbulak/haradan-be/internal/application/comment"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	handlercomment "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/comment"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

func fakeResponder(c *gin.Context, _ *slog.Logger, err error) {
	if apErr, ok := apperr.As(err); ok {
		switch apErr.Kind {
		case apperr.KindNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": apErr.Message})
			return
		case apperr.KindForbidden:
			c.JSON(http.StatusForbidden, gin.H{"message": apErr.Message})
			return
		case apperr.KindUnauthenticated:
			c.JSON(http.StatusUnauthorized, gin.H{"message": apErr.Message})
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
}

func TestDeleteAdvertComment_Handler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := appcomment.NewMemoryRepository()
	advertID := uuid.New()
	userID := uuid.New()

	repo.AddAdvert(appcomment.AdvertStatusResult{
		ID:     advertID,
		Status: "PUBLISHED",
	})
	repo.AddUser(userID, "Test User")

	svc := appcomment.NewMemoryService(repo)
	created, err := svc.CreateComment(context.Background(), appcomment.CreateCommentInput{
		UserID:   userID,
		AdvertID: advertID,
		Content:  "Test yorum",
	})
	if err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}

	h := handlercomment.NewHandler(svc, nil, fakeResponder)

	// 1. Success delete
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
	c.Request = c.Request.WithContext(authctx.WithPrincipal(c.Request.Context(), domainauth.Principal{
		UserID: userID,
	}))

	h.DeleteAdvertComment(c, advertID, created.Comment.ID)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 No Content, got %d", w.Code)
	}

	// 2. Second delete -> 404 Not Found
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
	c2.Request = c2.Request.WithContext(authctx.WithPrincipal(c2.Request.Context(), domainauth.Principal{
		UserID: userID,
	}))

	h.DeleteAdvertComment(c2, advertID, created.Comment.ID)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 Not Found on second delete, got %d", w2.Code)
	}
}
