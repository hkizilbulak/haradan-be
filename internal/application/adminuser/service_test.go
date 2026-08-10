package adminuser

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

type fakeRepo struct {
	users  []domainuser.User
	events []domainauth.SecurityEvent
	found  bool
}

func (r *fakeRepo) BeginTx(context.Context) (pgx.Tx, error) { panic("not used") }
func (r *fakeRepo) WithTx(pgx.Tx) Repository                { return r }
func (r *fakeRepo) ListUsers(_ context.Context, _ *domainuser.Status, _ *domainuser.Role, _ string, _ *time.Time, _ *uuid.UUID, limit int) ([]domainuser.User, error) {
	return r.users[:min(limit, len(r.users))], nil
}
func (r *fakeRepo) FindUser(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	if !r.found {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	return domainuser.User{ID: id}, nil
}
func (r *fakeRepo) FindUserForUpdate(context.Context, uuid.UUID) (domainuser.User, error) {
	panic("not used")
}
func (r *fakeRepo) GetDetail(context.Context, uuid.UUID, time.Time) (Detail, error) {
	panic("not used")
}
func (r *fakeRepo) ActiveSessionCount(context.Context, uuid.UUID, time.Time) (int, error) {
	panic("not used")
}
func (r *fakeRepo) UpdateRole(context.Context, uuid.UUID, domainuser.Role, uuid.UUID, time.Time) (domainuser.User, error) {
	panic("not used")
}
func (r *fakeRepo) UpdateStatus(context.Context, uuid.UUID, domainuser.Status, uuid.UUID, time.Time) (domainuser.User, error) {
	panic("not used")
}
func (r *fakeRepo) RevokeAllSessions(context.Context, uuid.UUID, time.Time, string) error {
	panic("not used")
}
func (r *fakeRepo) InsertSecurityEvent(context.Context, domainauth.SecurityEvent) error {
	panic("not used")
}
func (r *fakeRepo) ListSecurityEvents(_ context.Context, _ uuid.UUID, _ *domainauth.SecurityEventType, _ *time.Time, _ *uuid.UUID, limit int) ([]domainauth.SecurityEvent, error) {
	return r.events[:min(limit, len(r.events))], nil
}

func TestListUsersPaginatesWithOpaqueCursor(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeRepo{users: []domainuser.User{
		{ID: uuid.New(), CreatedAt: now},
		{ID: uuid.New(), CreatedAt: now.Add(-time.Minute)},
	}}
	svc, err := NewService(Config{Repository: repo})
	if err != nil {
		t.Fatal(err)
	}
	limit := 1
	out, err := svc.ListUsers(context.Background(), ListInput{Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if !out.HasMore || out.NextCursor == nil || len(out.Items) != 1 {
		t.Fatalf("unexpected page: %#v", out)
	}
	if _, _, err := decodeCursor(out.NextCursor); err != nil {
		t.Fatalf("generated cursor is invalid: %v", err)
	}
}

func TestListSecurityEventsRequiresExistingUser(t *testing.T) {
	svc, err := NewService(Config{Repository: &fakeRepo{found: false}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ListSecurityEvents(context.Background(), uuid.New(), EventListInput{})
	if err == nil {
		t.Fatal("expected error for absent user")
	}
}
