package advert_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

func TestListAdvertModerationQueueDefaultAndEmpty(t *testing.T) {
	f := newFixture(t)
	admin := uuid.New()
	_ = admin

	got, err := f.svc.ListAdvertModerationQueue(context.Background(), appadvert.ModerationListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got.HasMore || len(got.Items) != 0 || got.Items == nil {
		t.Fatalf("%+v", got)
	}

	pending := f.seed(t, f.owner, domainadvert.StatusPendingReview, nil)
	f.seed(t, f.owner, domainadvert.StatusPublished, nil)
	f.seed(t, f.owner, domainadvert.StatusDraft, nil)
	deleted := f.seed(t, f.owner, domainadvert.StatusPendingReview, func(a *domainadvert.Advert) {
		now := f.clock.Now()
		a.DeletedAt = &now
	})

	// When status is omitted, all 3 non-deleted adverts are returned
	got, err = f.svc.ListAdvertModerationQueue(context.Background(), appadvert.ModerationListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 3 {
		t.Fatalf("expected 3 items, got %+v", got)
	}
	for _, item := range got.Items {
		if item.ID == deleted.ID {
			t.Fatal("soft-deleted must be excluded")
		}
	}

	// When status is filtered by PENDING_REVIEW, only 1 is returned
	pendingStatus := string(domainadvert.StatusPendingReview)
	got, err = f.svc.ListAdvertModerationQueue(context.Background(), appadvert.ModerationListInput{Status: &pendingStatus})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].ID != pending.ID {
		t.Fatalf("expected only pending item, got %+v", got)
	}
}

func TestListAdvertModerationQueuePaginationAndFilter(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 3; i++ {
		f.seed(t, f.owner, domainadvert.StatusPendingReview, func(a *domainadvert.Advert) {
			a.CreatedAt = f.clock.Now().Add(-time.Duration(i) * time.Minute)
			a.ID = uuid.New()
		})
		f.clock.Advance(time.Second)
	}
	f.seed(t, f.owner, domainadvert.StatusChangesRequested, nil)

	first, err := f.svc.ListAdvertModerationQueue(context.Background(), appadvert.ModerationListInput{Limit: ptr(2)})
	if err != nil {
		t.Fatal(err)
	}
	if !first.HasMore || len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("%+v", first)
	}
	second, err := f.svc.ListAdvertModerationQueue(context.Background(), appadvert.ModerationListInput{
		Limit: ptr(2), Cursor: first.NextCursor,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.HasMore || len(second.Items) != 2 {
		t.Fatalf("%+v", second)
	}

	status := string(domainadvert.StatusChangesRequested)
	filtered, err := f.svc.ListAdvertModerationQueue(context.Background(), appadvert.ModerationListInput{Status: &status})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].Status != domainadvert.StatusChangesRequested {
		t.Fatalf("%+v", filtered)
	}

	_, err = f.svc.ListAdvertModerationQueue(context.Background(), appadvert.ModerationListInput{Status: ptr("NOPE")})
	requireCode(t, err, apperr.CodeValidation)
	_, err = f.svc.ListAdvertModerationQueue(context.Background(), appadvert.ModerationListInput{Cursor: ptr("bad")})
	requireCode(t, err, apperr.CodeValidation)
}

func TestGetAdvertModerationDetail(t *testing.T) {
	f := newFixture(t)
	pending := f.seed(t, f.owner, domainadvert.StatusPendingReview, nil)
	from := domainadvert.StatusDraft
	actor := f.owner
	f.store.Repo().InsertHistory(context.Background(), domainadvert.StatusHistory{
		ID: uuid.New(), AdvertID: pending.ID, FromStatus: &from,
		ToStatus: domainadvert.StatusPendingReview, ActorUserID: &actor,
		IsSystem: false, CreatedAt: f.clock.Now().Add(-time.Minute),
	})

	detail, err := f.svc.GetAdvertModerationDetail(context.Background(), pending.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.OwnerUserID != f.owner || detail.ID != pending.ID {
		t.Fatalf("%+v", detail)
	}
	if len(detail.StatusHistory) != 1 {
		t.Fatalf("history=%+v", detail.StatusHistory)
	}
	if strings.Contains(string(detail.Properties), "password") {
		t.Fatal("unexpected leak")
	}

	_, err = f.svc.GetAdvertModerationDetail(context.Background(), uuid.New())
	requireCode(t, err, apperr.CodeNotFound)

	deleted := f.seed(t, f.owner, domainadvert.StatusPendingReview, func(a *domainadvert.Advert) {
		now := f.clock.Now()
		a.DeletedAt = &now
	})
	_, err = f.svc.GetAdvertModerationDetail(context.Background(), deleted.ID)
	requireCode(t, err, apperr.CodeNotFound)
}

func TestApproveAdvertSuccessAndIdempotency(t *testing.T) {
	f := newFixture(t)
	admin := uuid.New()
	pending := f.seed(t, f.owner, domainadvert.StatusPendingReview, nil)

	detail, err := f.svc.ApproveAdvert(context.Background(), admin, pending.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != domainadvert.StatusPublished || detail.PublishedAt == nil || detail.Version != 2 {
		t.Fatalf("%+v", detail)
	}
	if detail.OwnerUserID != f.owner {
		t.Fatalf("owner=%s", detail.OwnerUserID)
	}
	if len(detail.StatusHistory) != 1 {
		t.Fatalf("history=%+v", detail.StatusHistory)
	}
	h := detail.StatusHistory[0]
	if h.ActorUserID == nil || *h.ActorUserID != admin || h.ToStatus != domainadvert.StatusPublished {
		t.Fatalf("%+v", h)
	}

	_, err = f.svc.ApproveAdvert(context.Background(), admin, pending.ID, 2)
	requireCode(t, err, apperr.CodeInvalidState)
}

func TestApproveAdvertValidationAndConflicts(t *testing.T) {
	f := newFixture(t)
	admin := uuid.New()

	incomplete := f.seed(t, f.owner, domainadvert.StatusPendingReview, func(a *domainadvert.Advert) {
		a.Title = nil
	})
	_, err := f.svc.ApproveAdvert(context.Background(), admin, incomplete.ID, 1)
	requireCode(t, err, apperr.CodeValidation)

	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
	_, err = f.svc.ApproveAdvert(context.Background(), admin, draft.ID, 1)
	requireCode(t, err, apperr.CodeInvalidState)

	pending := f.seed(t, f.owner, domainadvert.StatusPendingReview, nil)
	_, err = f.svc.ApproveAdvert(context.Background(), admin, pending.ID, 9)
	requireCode(t, err, apperr.CodeStaleVersion)

	_, err = f.svc.ApproveAdvert(context.Background(), admin, uuid.New(), 1)
	requireCode(t, err, apperr.CodeNotFound)

	deleted := f.seed(t, f.owner, domainadvert.StatusPendingReview, func(a *domainadvert.Advert) {
		now := f.clock.Now()
		a.DeletedAt = &now
	})
	_, err = f.svc.ApproveAdvert(context.Background(), admin, deleted.ID, 1)
	requireCode(t, err, apperr.CodeNotFound)
}

func TestRequestChangesRejectSuspend(t *testing.T) {
	f := newFixture(t)
	admin := uuid.New()

	pending := f.seed(t, f.owner, domainadvert.StatusPendingReview, nil)
	detail, err := f.svc.RequestAdvertChanges(context.Background(), admin, pending.ID, appadvert.ModerationReasonInput{
		ExpectedVersion: 1, Reason: "  Eksik fotoğraf  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != domainadvert.StatusChangesRequested || detail.Version != 2 {
		t.Fatalf("%+v", detail)
	}
	if detail.StatusHistory[0].Reason == nil || *detail.StatusHistory[0].Reason != "Eksik fotoğraf" {
		t.Fatalf("%+v", detail.StatusHistory[0])
	}

	_, err = f.svc.RequestAdvertChanges(context.Background(), admin, pending.ID, appadvert.ModerationReasonInput{
		ExpectedVersion: 2, Reason: "   ",
	})
	requireCode(t, err, apperr.CodeValidation)

	pending2 := f.seed(t, f.owner, domainadvert.StatusPendingReview, nil)
	detail, err = f.svc.RejectAdvert(context.Background(), admin, pending2.ID, appadvert.ModerationReasonInput{
		ExpectedVersion: 1, Reason: "Uygun değil",
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != domainadvert.StatusRejected {
		t.Fatalf("%+v", detail)
	}

	published := f.seed(t, f.owner, domainadvert.StatusPublished, nil)
	detail, err = f.svc.SuspendAdvert(context.Background(), admin, published.ID, appadvert.ModerationReasonInput{
		ExpectedVersion: 1, Reason: "Şikayet",
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != domainadvert.StatusSuspended || detail.PublishedAt == nil {
		t.Fatalf("suspend must keep published_at: %+v", detail)
	}

	_, err = f.svc.SuspendAdvert(context.Background(), admin, pending2.ID, appadvert.ModerationReasonInput{
		ExpectedVersion: 2, Reason: "x",
	})
	requireCode(t, err, apperr.CodeInvalidState)
}

func TestAdminTransitionAtomicityAndConcurrency(t *testing.T) {
	f := newFixture(t)
	adminA := uuid.New()
	adminB := uuid.New()
	pending := f.seed(t, f.owner, domainadvert.StatusPendingReview, nil)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := f.svc.ApproveAdvert(context.Background(), adminA, pending.ID, 1)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := f.svc.RejectAdvert(context.Background(), adminB, pending.ID, appadvert.ModerationReasonInput{
			ExpectedVersion: 1, Reason: "çakışma",
		})
		errs <- err
	}()
	wg.Wait()
	close(errs)

	var success, fail int
	for err := range errs {
		if err == nil {
			success++
			continue
		}
		fail++
		ae, ok := apperr.As(err)
		if !ok || (ae.Code != apperr.CodeStaleVersion && ae.Code != apperr.CodeInvalidState) {
			t.Fatalf("unexpected err=%v", err)
		}
	}
	if success != 1 || fail != 1 {
		t.Fatalf("success=%d fail=%d", success, fail)
	}
	hist := f.store.History()
	advertHist := 0
	for _, h := range hist {
		if h.AdvertID == pending.ID {
			advertHist++
		}
	}
	if advertHist != 1 {
		t.Fatalf("history rows=%d want 1", advertHist)
	}
	a, ok := f.store.Advert(pending.ID)
	if !ok || a.Version != 2 {
		t.Fatalf("%+v", a)
	}
}
