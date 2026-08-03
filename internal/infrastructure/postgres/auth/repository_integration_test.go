package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	pgauth "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/auth"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/testutil"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/token"
)

func TestUserSessionSecurityIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC()
	users := pguser.NewRepository(tx)
	sessions := pgauth.NewRepository(nil).WithTx(tx)

	u := domainuser.User{
		ID: uuid.New(), Email: "it@example.com", EmailNormalized: "it@example.com",
		PasswordHash: "hash", Role: domainuser.RoleUser, Status: domainuser.StatusActive,
		FirstName: "A", LastName: "B", SecurityStamp: uuid.New(), CreatedAt: now, UpdatedAt: now,
	}
	if err := users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := users.FindByNormalizedEmail(ctx, "it@example.com")
	if err != nil || got.ID != u.ID {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	plain, hash, err := token.NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	_ = plain
	s := domainauth.Session{
		ID: uuid.New(), UserID: u.ID, ClientContext: domainauth.ClientContextPublicWeb,
		RefreshTokenHash: hash, FamilyID: uuid.New(),
		AbsoluteExpiresAt: now.Add(24 * time.Hour), IdleExpiresAt: now.Add(2 * time.Hour),
		CreatedAt: now, LastUsedAt: now,
	}
	if err := sessions.CreateSession(ctx, s); err != nil {
		t.Fatal(err)
	}
	found, err := sessions.FindSessionByRefreshHashForUpdate(ctx, hash)
	if err != nil || found.ID != s.ID {
		t.Fatalf("found=%+v err=%v", found, err)
	}
	if err := sessions.RevokeSession(ctx, s.ID, now, "LOGOUT", nil); err != nil {
		t.Fatal(err)
	}
	if err := sessions.InsertSecurityEvent(ctx, domainauth.SecurityEvent{
		ID: uuid.New(), SubjectUserID: &u.ID, ActorUserID: &u.ID,
		EventType: domainauth.EventLogout, CreatedAt: now, Metadata: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}

	otcPlain, otcHash, err := token.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	_ = otcPlain
	cred := domainauth.OneTimeCredential{
		ID: uuid.New(), UserID: u.ID, Purpose: domainauth.PurposeEmailVerification,
		TokenHash: otcHash, TargetEmail: u.Email, TargetEmailNormalized: u.EmailNormalized,
		ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}
	if err := sessions.CreateOneTimeCredential(ctx, cred); err != nil {
		t.Fatal(err)
	}
	locked, err := sessions.FindOneTimeCredentialByHashForUpdate(ctx, otcHash)
	if err != nil || locked.ID != cred.ID {
		t.Fatalf("otc=%+v err=%v", locked, err)
	}
	if err := sessions.ConsumeOneTimeCredential(ctx, cred.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := sessions.ConsumeOneTimeCredential(ctx, cred.ID, now); err == nil {
		t.Fatal("second consume must fail")
	}
	if err := users.MarkEmailVerified(ctx, u.ID, now); err != nil {
		t.Fatal(err)
	}
	verified, err := users.FindByID(ctx, u.ID)
	if err != nil || verified.EmailVerifiedAt == nil {
		t.Fatalf("verified=%+v err=%v", verified, err)
	}
}
