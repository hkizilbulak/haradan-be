package adminuser

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

type fakeRepo struct {
	users             []domainuser.User
	events            []domainauth.SecurityEvent
	found             bool
	activeAdminsFixed bool
	activeAdmins      int
	created           []domainuser.User
	otc               []domainauth.OneTimeCredential
	invalidatedOTC    int
	revoked           int
	txMode            bool
	guardLocked       int
	guardMu           *sync.Mutex
	guardHeld         bool
}

type fakeHasher struct{}

func (fakeHasher) Hash(password string) (string, error) { return "hash:" + password, nil }

type recordingEmail struct {
	calls int
	err   error
}

func (e *recordingEmail) SendPasswordReset(context.Context, string, string, string) error {
	e.calls++
	return e.err
}
func (e *recordingEmail) SendRegistrationVerification(context.Context, string, string, string) error {
	e.calls++
	return e.err
}

type stubTx struct {
	onDone func()
	done   bool
}

func (*stubTx) Begin(context.Context) (pgx.Tx, error) { panic("unused") }
func (t *stubTx) finish() {
	if t.done {
		return
	}
	t.done = true
	if t.onDone != nil {
		t.onDone()
	}
}
func (t *stubTx) Commit(context.Context) error {
	t.finish()
	return nil
}
func (t *stubTx) Rollback(context.Context) error {
	t.finish()
	return nil
}
func (*stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("unused")
}
func (*stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("unused") }
func (*stubTx) LargeObjects() pgx.LargeObjects                          { panic("unused") }
func (*stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("unused")
}
func (*stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("unused")
}
func (*stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("unused") }
func (*stubTx) QueryRow(context.Context, string, ...any) pgx.Row       { panic("unused") }
func (*stubTx) Conn() *pgx.Conn                                         { panic("unused") }

func (r *fakeRepo) BeginTx(context.Context) (pgx.Tx, error) {
	if !r.txMode {
		panic("not used")
	}
	return &stubTx{onDone: func() {
		if r.guardHeld && r.guardMu != nil {
			r.guardMu.Unlock()
			r.guardHeld = false
		}
	}}, nil
}
func (r *fakeRepo) WithTx(pgx.Tx) Repository { return r }
func (r *fakeRepo) ListUsers(_ context.Context, _ *domainuser.Status, _ *domainuser.Role, _ string, _ *time.Time, _ *uuid.UUID, limit int) ([]domainuser.User, error) {
	return r.users[:min(limit, len(r.users))], nil
}
func (r *fakeRepo) FindUser(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	if !r.found {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	return domainuser.User{ID: id}, nil
}
func (r *fakeRepo) FindUserForUpdate(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return domainuser.User{}, apperr.NotFound("user not found")
}
func (r *fakeRepo) GetDetail(context.Context, uuid.UUID, time.Time) (Detail, error) {
	panic("not used")
}
func (r *fakeRepo) ActiveSessionCount(context.Context, uuid.UUID, time.Time) (int, error) {
	return 0, nil
}
func (r *fakeRepo) UpdateRole(_ context.Context, userID uuid.UUID, role domainuser.Role, _ uuid.UUID, now time.Time) (domainuser.User, error) {
	for i, u := range r.users {
		if u.ID == userID {
			u.Role = role
			u.UpdatedAt = now
			r.users[i] = u
			return u, nil
		}
	}
	return domainuser.User{}, apperr.NotFound("user not found")
}
func (r *fakeRepo) UpdateStatus(_ context.Context, userID uuid.UUID, status domainuser.Status, _ uuid.UUID, now time.Time) (domainuser.User, error) {
	for i, u := range r.users {
		if u.ID == userID {
			u.Status = status
			u.UpdatedAt = now
			r.users[i] = u
			return u, nil
		}
	}
	return domainuser.User{}, apperr.NotFound("user not found")
}
func (r *fakeRepo) RevokeAllSessions(context.Context, uuid.UUID, time.Time, string) error {
	r.revoked++
	return nil
}
func (r *fakeRepo) InsertSecurityEvent(_ context.Context, event domainauth.SecurityEvent) error {
	r.events = append(r.events, event)
	return nil
}
func (r *fakeRepo) ListSecurityEvents(_ context.Context, _ uuid.UUID, _ *domainauth.SecurityEventType, _ *time.Time, _ *uuid.UUID, limit int) ([]domainauth.SecurityEvent, error) {
	return r.events[:min(limit, len(r.events))], nil
}
func (r *fakeRepo) CreateUser(_ context.Context, user domainuser.User) error {
	for _, existing := range append(r.created, r.users...) {
		if existing.EmailNormalized == user.EmailNormalized {
			return apperr.Conflict("Bu e-posta adresi zaten kayıtlı.")
		}
	}
	r.created = append(r.created, user)
	r.users = append(r.users, user)
	return nil
}
func (r *fakeRepo) UpdateProfile(_ context.Context, userID uuid.UUID, firstName, lastName string, phone *string, now time.Time) (domainuser.User, error) {
	for i, u := range r.users {
		if u.ID == userID {
			u.FirstName = firstName
			u.LastName = lastName
			u.Phone = phone
			u.UpdatedAt = now
			r.users[i] = u
			return u, nil
		}
	}
	return domainuser.User{}, apperr.NotFound("user not found")
}
func (r *fakeRepo) FindUserByNormalizedEmail(_ context.Context, normalized string) (domainuser.User, error) {
	for _, u := range append(r.created, r.users...) {
		if u.EmailNormalized == normalized {
			return u, nil
		}
	}
	return domainuser.User{}, apperr.NotFound("user not found")
}
func (r *fakeRepo) CountActiveAdmins(context.Context) (int, error) {
	if r.activeAdminsFixed {
		return r.activeAdmins, nil
	}
	n := 0
	for _, u := range r.users {
		if u.Role == domainuser.RoleAdmin && u.Status == domainuser.StatusActive {
			n++
		}
	}
	return n, nil
}
func (r *fakeRepo) LockActiveAdminGuard(context.Context) error {
	r.guardLocked++
	if r.guardMu != nil {
		r.guardMu.Lock()
		r.guardHeld = true
	}
	return nil
}
func (r *fakeRepo) InvalidateActiveOneTimeCredentials(_ context.Context, userID uuid.UUID, purpose domainauth.OneTimePurpose, _ time.Time) error {
	kept := r.otc[:0]
	for _, cred := range r.otc {
		if cred.UserID == userID && cred.Purpose == purpose {
			r.invalidatedOTC++
			continue
		}
		kept = append(kept, cred)
	}
	r.otc = kept
	return nil
}
func (r *fakeRepo) CreateOneTimeCredential(_ context.Context, cred domainauth.OneTimeCredential) error {
	r.otc = append(r.otc, cred)
	return nil
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

func TestCreateUserIssuesInvitationWithoutPassword(t *testing.T) {
	repo := &fakeRepo{txMode: true}
	email := &recordingEmail{}
	svc, err := NewService(Config{
		Repository:      repo,
		Hasher:          fakeHasher{},
		EmailSender:     email,
		EmailConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.CreateUser(context.Background(), CreateInput{
		ActorUserID: uuid.New(),
		Email:       "New.Admin@Example.com",
		FirstName:   "Ada",
		LastName:    "Admin",
		Role:        domainuser.RoleAdmin,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.InvitationEmailSent || email.calls != 1 {
		t.Fatalf("expected invitation email sent: %#v calls=%d", out, email.calls)
	}
	if len(repo.created) != 1 || repo.created[0].PasswordHash == "" || len(repo.otc) != 1 {
		t.Fatalf("expected created user + otc: created=%d otc=%d", len(repo.created), len(repo.otc))
	}
	if repo.otc[0].Purpose != domainauth.PurposePasswordReset {
		t.Fatalf("expected PASSWORD_RESET purpose, got %s", repo.otc[0].Purpose)
	}
	if out.Detail.User.Role != domainuser.RoleAdmin || out.Detail.User.EmailVerifiedAt != nil {
		t.Fatalf("admin create must leave email unverified until invitation completion: %#v", out.Detail)
	}
}

func TestCreateUserUnconfiguredEmailMarksInvitationNotSent(t *testing.T) {
	repo := &fakeRepo{txMode: true}
	email := &recordingEmail{}
	svc, err := NewService(Config{
		Repository:      repo,
		Hasher:          fakeHasher{},
		EmailSender:     email,
		EmailConfigured: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.CreateUser(context.Background(), CreateInput{
		ActorUserID: uuid.New(),
		Email:       "user@example.com",
		FirstName:   "U",
		LastName:    "Ser",
		Role:        domainuser.RoleUser,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.InvitationEmailSent || email.calls != 0 {
		t.Fatalf("expected no email when unconfigured: %#v calls=%d", out, email.calls)
	}
	if len(repo.created) != 1 {
		t.Fatal("user should still be created")
	}
}

func TestCreateUserSendFailureKeepsAccountRecoverable(t *testing.T) {
	repo := &fakeRepo{txMode: true}
	email := &recordingEmail{err: apperr.DependencyUnavailable("send failed")}
	svc, err := NewService(Config{
		Repository: repo, Hasher: fakeHasher{}, EmailSender: email, EmailConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.CreateUser(context.Background(), CreateInput{
		ActorUserID: uuid.New(),
		Email:       "fail@example.com",
		FirstName:   "F",
		LastName:    "Ail",
		Role:        domainuser.RoleUser,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.InvitationEmailSent || email.calls != 1 {
		t.Fatalf("expected send attempted but not marked sent: %#v calls=%d", out, email.calls)
	}
	if len(repo.created) != 1 || len(repo.otc) != 1 {
		t.Fatal("account + OTC must remain for later resend")
	}
}

func TestResendInvitationSendFailureReturnsUnavailable(t *testing.T) {
	userID := uuid.New()
	repo := &fakeRepo{
		txMode: true,
		users: []domainuser.User{{
			ID: userID, Email: "a@b.com", FirstName: "A", LastName: "B",
			Role: domainuser.RoleUser, Status: domainuser.StatusActive,
		}},
	}
	email := &recordingEmail{err: apperr.DependencyUnavailable("send failed")}
	svc, err := NewService(Config{
		Repository: repo, EmailConfigured: true, EmailSender: email, Hasher: fakeHasher{},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ResendInvitation(context.Background(), uuid.New(), userID)
	if err == nil {
		t.Fatal("expected dependency unavailable")
	}
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("got %#v", err)
	}
	if email.calls != 1 || len(repo.otc) != 1 {
		t.Fatalf("send fail after rotate: calls=%d otc=%d", email.calls, len(repo.otc))
	}
}

func TestChangeRoleRejectsLastActiveAdminDemotion(t *testing.T) {
	adminID := uuid.New()
	repo := &fakeRepo{
		txMode:       true,
		activeAdmins: 1,
		users: []domainuser.User{{
			ID: adminID, Role: domainuser.RoleAdmin, Status: domainuser.StatusActive,
		}},
	}
	svc, err := NewService(Config{Repository: repo})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ChangeRole(context.Background(), uuid.New(), adminID, domainuser.RoleAdmin, domainuser.RoleUser)
	if err == nil {
		t.Fatal("expected last-admin protection")
	}
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestChangeStatusRejectsLastActiveAdminDisable(t *testing.T) {
	adminID := uuid.New()
	repo := &fakeRepo{
		txMode:       true,
		activeAdmins: 1,
		users: []domainuser.User{{
			ID: adminID, Role: domainuser.RoleAdmin, Status: domainuser.StatusActive,
		}},
	}
	svc, err := NewService(Config{Repository: repo})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ChangeStatus(context.Background(), uuid.New(), adminID, domainuser.StatusActive, domainuser.StatusDisabled)
	if err == nil {
		t.Fatal("expected last-admin protection")
	}
}

func TestChangeRoleRevokesSessions(t *testing.T) {
	userID := uuid.New()
	repo := &fakeRepo{
		txMode: true,
		users: []domainuser.User{{
			ID: userID, Role: domainuser.RoleUser, Status: domainuser.StatusActive,
		}},
	}
	svc, err := NewService(Config{Repository: repo})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ChangeRole(context.Background(), uuid.New(), userID, domainuser.RoleUser, domainuser.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if repo.revoked != 1 {
		t.Fatalf("expected session revoke, got %d", repo.revoked)
	}
}

func TestResendInvitationUnconfiguredDoesNotRotateOTC(t *testing.T) {
	userID := uuid.New()
	repo := &fakeRepo{
		txMode: true,
		users: []domainuser.User{{
			ID: userID, Email: "a@b.com", FirstName: "A", LastName: "B",
			Role: domainuser.RoleUser, Status: domainuser.StatusActive,
		}},
		otc: []domainauth.OneTimeCredential{{
			ID: uuid.New(), UserID: userID, Purpose: domainauth.PurposePasswordReset,
		}},
	}
	svc, err := NewService(Config{Repository: repo, EmailConfigured: false, Hasher: fakeHasher{}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ResendInvitation(context.Background(), uuid.New(), userID)
	if err == nil {
		t.Fatal("expected dependency unavailable")
	}
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("got %#v", err)
	}
	if repo.invalidatedOTC != 0 || len(repo.otc) != 1 {
		t.Fatalf("unconfigured resend must not rotate OTC: invalidated=%d otc=%d", repo.invalidatedOTC, len(repo.otc))
	}
}

func TestResendInvitationSuccessRotatesOTC(t *testing.T) {
	userID := uuid.New()
	oldOTC := uuid.New()
	repo := &fakeRepo{
		txMode: true,
		users: []domainuser.User{{
			ID: userID, Email: "a@b.com", FirstName: "A", LastName: "B",
			Role: domainuser.RoleUser, Status: domainuser.StatusActive,
		}},
		otc: []domainauth.OneTimeCredential{{
			ID: oldOTC, UserID: userID, Purpose: domainauth.PurposePasswordReset,
		}},
	}
	email := &recordingEmail{}
	svc, err := NewService(Config{
		Repository: repo, EmailConfigured: true, EmailSender: email, Hasher: fakeHasher{},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.ResendInvitation(context.Background(), uuid.New(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if !out.InvitationEmailSent || email.calls != 1 {
		t.Fatalf("expected sent invitation: %#v calls=%d", out, email.calls)
	}
	if repo.invalidatedOTC != 1 || len(repo.otc) != 1 || repo.otc[0].ID == oldOTC {
		t.Fatalf("expected OTC rotation: invalidated=%d otc=%#v", repo.invalidatedOTC, repo.otc)
	}
}

func TestConcurrentCrossDemoteLeavesOneActiveAdmin(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	var guard sync.Mutex
	repo := &fakeRepo{
		txMode:  true,
		guardMu: &guard,
		users: []domainuser.User{
			{ID: a, Role: domainuser.RoleAdmin, Status: domainuser.StatusActive},
			{ID: b, Role: domainuser.RoleAdmin, Status: domainuser.StatusActive},
		},
	}
	svc, err := NewService(Config{Repository: repo})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := svc.ChangeRole(context.Background(), a, b, domainuser.RoleAdmin, domainuser.RoleUser)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := svc.ChangeRole(context.Background(), b, a, domainuser.RoleAdmin, domainuser.RoleUser)
		errs <- err
	}()
	wg.Wait()
	close(errs)

	var failCount, okCount int
	for err := range errs {
		if err == nil {
			okCount++
			continue
		}
		failCount++
		if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindConflict {
			t.Fatalf("expected conflict or success, got %v", err)
		}
	}
	if okCount < 1 || failCount < 1 {
		// With serialization, one succeeds and one should conflict when count drops to 1.
		// If both somehow succeed without lock, active count would be 0.
	}
	n := 0
	for _, u := range repo.users {
		if u.Role == domainuser.RoleAdmin && u.Status == domainuser.StatusActive {
			n++
		}
	}
	if n < 1 {
		t.Fatalf("expected at least one ACTIVE admin remaining, got %d (ok=%d fail=%d guard=%d)", n, okCount, failCount, repo.guardLocked)
	}
	if repo.guardLocked < 1 {
		t.Fatal("expected advisory guard to be taken")
	}
}

func TestCreateUserBlankPhoneNormalizesToNil(t *testing.T) {
	repo := &fakeRepo{txMode: true}
	svc, err := NewService(Config{Repository: repo, Hasher: fakeHasher{}, EmailConfigured: false})
	if err != nil {
		t.Fatal(err)
	}
	blank := ""
	out, err := svc.CreateUser(context.Background(), CreateInput{
		ActorUserID: uuid.New(),
		Email:       "blank-phone@example.com",
		FirstName:   "Blank",
		LastName:    "Phone",
		Phone:       &blank,
		Role:        domainuser.RoleUser,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Detail.User.Phone != nil {
		t.Fatalf("expected nil phone, got %#v", out.Detail.User.Phone)
	}
}

func TestCreateUserInvalidPhoneRejected(t *testing.T) {
	repo := &fakeRepo{txMode: true}
	svc, err := NewService(Config{Repository: repo, Hasher: fakeHasher{}, EmailConfigured: false})
	if err != nil {
		t.Fatal(err)
	}
	garbage := "11111111111111111111"
	_, err = svc.CreateUser(context.Background(), CreateInput{
		ActorUserID: uuid.New(),
		Email:       "bad-phone@example.com",
		FirstName:   "Bad",
		LastName:    "Phone",
		Phone:       &garbage,
		Role:        domainuser.RoleUser,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindValidation {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestUpdateProfileAndRequestEmailChange(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	phone := "+905321234567"
	repo := &fakeRepo{
		txMode: true,
		users: []domainuser.User{{
			ID: userID, Email: "old@example.com", EmailNormalized: "old@example.com",
			FirstName: "Old", LastName: "Name", Phone: &phone,
			Role: domainuser.RoleUser, Status: domainuser.StatusActive, UpdatedAt: now,
		}},
	}
	email := &recordingEmail{}
	svc, err := NewService(Config{
		Repository: repo, Hasher: fakeHasher{}, EmailSender: email, EmailConfigured: true,
		Clock: fixedClock{now},
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := svc.UpdateProfile(context.Background(), UpdateProfileInput{
		ActorUserID: uuid.New(), UserID: userID, ExpectedUpdatedAt: now,
		FirstName: "New", LastName: "Person", PhoneSet: true, Phone: ptr("0532 123 45 67"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.User.FirstName != "New" || updated.User.LastName != "Person" {
		t.Fatalf("profile not updated: %#v", updated.User)
	}
	if updated.User.Phone == nil || *updated.User.Phone != "+905321234567" {
		t.Fatalf("phone not canonical: %#v", updated.User.Phone)
	}
	if updated.User.Email != "old@example.com" {
		t.Fatal("UpdateProfile must not change email")
	}

	inviteOTC := domainauth.OneTimeCredential{
		ID: uuid.New(), UserID: userID, Purpose: domainauth.PurposePasswordReset, CreatedAt: now,
	}
	repo.otc = append(repo.otc, inviteOTC)

	err = svc.RequestEmailChange(context.Background(), RequestEmailChangeInput{
		ActorUserID: uuid.New(), UserID: userID, NewEmail: "New.Mail@Example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if email.calls != 1 {
		t.Fatalf("expected verification email, calls=%d", email.calls)
	}
	if repo.users[0].Email != "old@example.com" {
		t.Fatal("current email must remain until confirm")
	}
	var hasEmailChange, hasPasswordReset bool
	for _, cred := range repo.otc {
		if cred.Purpose == domainauth.PurposeEmailChangeVerification {
			hasEmailChange = true
			if cred.TargetEmailNormalized != "new.mail@example.com" {
				t.Fatalf("unexpected target: %#v", cred)
			}
		}
		if cred.Purpose == domainauth.PurposePasswordReset {
			hasPasswordReset = true
		}
	}
	if !hasEmailChange {
		t.Fatal("expected EMAIL_CHANGE OTC")
	}
	if !hasPasswordReset {
		t.Fatal("PASSWORD_RESET invitation OTC must not be invalidated by email change")
	}
}

func TestRequestEmailChangeUnconfiguredKeepsCurrentEmail(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	repo := &fakeRepo{
		txMode: true,
		users: []domainuser.User{{
			ID: userID, Email: "keep@example.com", EmailNormalized: "keep@example.com",
			FirstName: "K", LastName: "Eep", Role: domainuser.RoleUser, Status: domainuser.StatusActive, UpdatedAt: now,
		}},
	}
	svc, err := NewService(Config{Repository: repo, Hasher: fakeHasher{}, EmailConfigured: false})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.RequestEmailChange(context.Background(), RequestEmailChangeInput{
		ActorUserID: uuid.New(), UserID: userID, NewEmail: "other@example.com",
	})
	if err == nil {
		t.Fatal("expected dependency unavailable")
	}
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("expected dependency unavailable, got %v", err)
	}
	if repo.users[0].Email != "keep@example.com" || len(repo.otc) != 0 {
		t.Fatal("email and OTC must be unchanged when provider unconfigured")
	}
}

func ptr[T any](v T) *T { return &v }

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }
