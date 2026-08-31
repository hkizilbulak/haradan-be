package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	pgmedia "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/media"
	pgnotification "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/notification"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
)

type postgresSnapshots struct{ pool *pgxpool.Pool }

func (s postgresSnapshots) GetAdvertSnapshot(ctx context.Context, id int64) (AdvertSnapshot, error) {
	var out AdvertSnapshot
	err := s.pool.QueryRow(ctx, `SELECT id, owner_user_id, COALESCE(title, ''), status FROM hrd_adverts WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&out.ID, &out.OwnerUserID, &out.Title, &out.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdvertSnapshot{}, apperr.NotFound("İlan bulunamadı.")
	}
	if err != nil {
		return AdvertSnapshot{}, apperr.Internal(fmt.Errorf("get advert snapshot: %w", err))
	}
	return out, nil
}

func (s postgresSnapshots) GetPackageByID(ctx context.Context, id uuid.UUID) (domainpackaging.Package, error) {
	var (
		out  domainpackaging.Package
		code string
	)
	err := s.pool.QueryRow(ctx, `SELECT id, code, display_name, description, badge_text, benefits, display_price_amount_minor, currency_code, default_duration_days, allows_urgent, showcase_eligible, search_priority, is_active, sort_order, version, created_at, updated_at FROM hrd_packages WHERE id = $1`, id).Scan(
		&out.ID, &code, &out.DisplayName, &out.Description, &out.BadgeText, &out.BenefitsJSON,
		&out.DisplayPriceAmountMinor, &out.CurrencyCode, &out.DefaultDurationDays, &out.AllowsUrgent,
		&out.ShowcaseEligible, &out.SearchPriority, &out.IsActive, &out.SortOrder, &out.Version, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.Package{}, apperr.NotFound("Paket bulunamadı.")
	}
	if err != nil {
		return domainpackaging.Package{}, apperr.Internal(fmt.Errorf("get package snapshot: %w", err))
	}
	out.Code = domainpackaging.PackageCode(code)
	return out, nil
}

func (s postgresSnapshots) GetEffectiveAssignment(ctx context.Context, advertID int64, at time.Time) (PackageAssignmentSnapshot, error) {
	return s.assignment(ctx, `SELECT id, advert_id, package_id, ends_at FROM hrd_advert_package_assignments WHERE advert_id = $1 AND status = 'ACTIVE' AND starts_at <= $2 AND (ends_at IS NULL OR ends_at > $2)`, advertID, at)
}
func (s postgresSnapshots) GetAssignmentByID(ctx context.Context, id uuid.UUID) (PackageAssignmentSnapshot, error) {
	return s.assignment(ctx, `SELECT id, advert_id, package_id, ends_at FROM hrd_advert_package_assignments WHERE id = $1`, id)
}
func (s postgresSnapshots) assignment(ctx context.Context, q string, args ...any) (PackageAssignmentSnapshot, error) {
	var out PackageAssignmentSnapshot
	err := s.pool.QueryRow(ctx, q, args...).Scan(&out.ID, &out.AdvertID, &out.PackageID, &out.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PackageAssignmentSnapshot{}, apperr.NotFound("Paket ataması bulunamadı.")
	}
	if err != nil {
		return PackageAssignmentSnapshot{}, apperr.Internal(fmt.Errorf("get assignment snapshot: %w", err))
	}
	return out, nil
}
func (s postgresSnapshots) FindActiveUrgent(ctx context.Context, advertID int64) (domainpackaging.AdvertFeatureActivation, error) {
	var (
		out                 domainpackaging.AdvertFeatureActivation
		featureCode, status string
	)
	err := s.pool.QueryRow(ctx, `SELECT id, advert_id, package_assignment_id, feature_code, status, activated_by_user_id, activated_at, deactivated_at, reason, activation_version, created_at, updated_at FROM hrd_advert_feature_activations WHERE advert_id = $1 AND feature_code = 'URGENT' AND status = 'ACTIVE'`, advertID).Scan(
		&out.ID, &out.AdvertID, &out.PackageAssignmentID, &featureCode, &status, &out.ActivatedByUserID, &out.ActivatedAt, &out.DeactivatedAt, &out.Reason, &out.ActivationVersion, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.AdvertFeatureActivation{}, apperr.NotFound("URGENT aktivasyonu bulunamadı.")
	}
	if err != nil {
		return domainpackaging.AdvertFeatureActivation{}, apperr.Internal(fmt.Errorf("get urgent snapshot: %w", err))
	}
	out.FeatureCode, out.Status = domainpackaging.FeatureCode(featureCode), domainpackaging.FeatureActivationStatus(status)
	return out, nil
}

type pgRuntimeRepo struct{ *pgnotification.Repository }

func (r pgRuntimeRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.Repository.BeginTx(ctx)
}

func (r pgRuntimeRepo) WithTx(tx pgx.Tx) RuntimeRepository {
	return pgRuntimeRepo{r.Repository.WithTx(tx)}
}

var _ RuntimeRepository = pgRuntimeRepo{}

// pgJobEnqueuer scopes durable job inserts to the media jobs table via the
// media postgres repository, so WithTx wraps the *same* repo.WithTx used by
// the media domain: a notification row and its fan-out job commit or roll
// back together instead of the job surviving a rolled-back transaction (or
// vice versa).
type pgJobEnqueuer struct{ repo *pgmedia.Repository }

// NewPostgresJobEnqueuer wires a JobEnqueuer backed by the shared
// hrd_background_jobs table via the media postgres repository.
func NewPostgresJobEnqueuer(pool *pgxpool.Pool) JobEnqueuer {
	return pgJobEnqueuer{repo: pgmedia.NewRepository(pool)}
}

func (e pgJobEnqueuer) WithTx(tx pgx.Tx) JobEnqueuer {
	return pgJobEnqueuer{repo: e.repo.WithTx(tx)}
}

func (e pgJobEnqueuer) EnqueueJob(ctx context.Context, job domainmedia.BackgroundJob) error {
	return e.repo.EnqueueJob(ctx, job)
}

var _ JobEnqueuer = pgJobEnqueuer{}

// NewPostgresUserNotificationService wires UserNotificationService directly
// to a PostgreSQL pool for callers (e.g. main.go DI) that only need the
// current-user inbox use cases, not the admin template/fanout services.
func NewPostgresUserNotificationService(pool *pgxpool.Pool, clock Clock) (*UserNotificationService, error) {
	repo := pgRuntimeRepo{pgnotification.NewRepository(pool)}
	return NewUserNotificationService(UserNotificationConfig{Repo: repo, Clock: clock})
}

// NewPostgresEmitter builds the transactional notification hooks used by the
// advert and packaging services. The notification repository also supplies the
// immutable snapshots consumed by template rendering.
func NewPostgresEmitter(pool *pgxpool.Pool, frontendURL string, clock Clock) (*Emitter, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	raw := pgnotification.NewRepository(pool)
	repo := pgRuntimeRepo{raw}
	snapshots := postgresSnapshots{pool: pool}
	writer, err := NewEventWriter(EventWriterConfig{
		Repo: repo, Jobs: NewPostgresJobEnqueuer(pool), Adverts: snapshots, Packages: snapshots,
		Clock: clock, FrontendURL: frontendURL,
	})
	if err != nil {
		return nil, err
	}
	return NewEmitter(EmitterConfig{Writer: writer, Adverts: snapshots, Packages: snapshots, Clock: clock})
}

// RuntimeWorker binds notification queue handlers to a shared database pool.
type RuntimeWorker struct {
	fanout *FanoutService
	expiry *ExpiryScanService
}

func NewPostgresRuntimeWorker(pool *pgxpool.Pool, email NotificationEmailSender, frontendURL string, clock Clock) (*RuntimeWorker, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	raw := pgnotification.NewRepository(pool)
	repo := pgRuntimeRepo{raw}
	snapshots := postgresSnapshots{pool: pool}
	jobs := NewPostgresJobEnqueuer(pool)
	writer, err := NewEventWriter(EventWriterConfig{
		Repo: repo, Jobs: jobs, Adverts: snapshots, Packages: snapshots, Clock: clock, FrontendURL: frontendURL,
	})
	if err != nil {
		return nil, err
	}
	users := pguser.NewRepository(pool)
	fanout, err := NewFanoutService(FanoutConfig{Repo: repo, Jobs: jobs, Email: email, Users: users, Clock: clock})
	if err != nil {
		return nil, err
	}
	expiry, err := NewExpiryScanService(ExpiryScanConfig{
		Writer: writer, Repo: repo, Jobs: jobs, Adverts: snapshots, Users: users, Clock: clock,
	})
	if err != nil {
		return nil, err
	}
	return &RuntimeWorker{fanout: fanout, expiry: expiry}, nil
}

func (w *RuntimeWorker) ProcessAdvertFanout(ctx context.Context, jobType domainmedia.JobType, payload []byte) error {
	return w.fanout.ProcessAdvertFanout(ctx, jobType, payload)
}

func (w *RuntimeWorker) ProcessAdvertEmailChunk(ctx context.Context, payload []byte) error {
	return w.fanout.ProcessAdvertEmailChunk(ctx, payload)
}

func (w *RuntimeWorker) ProcessExpiryScan(ctx context.Context, payload []byte) error {
	return w.expiry.ProcessExpiryScan(ctx, payload)
}

func (w *RuntimeWorker) ProcessPackageExpiryEmail(ctx context.Context, payload []byte) error {
	return w.fanout.ProcessPackageExpiryEmail(ctx, payload)
}

// NewExpiryScheduler creates a scheduler backed by this worker's expiry
// service, allowing cmd/worker to stop it before draining queue work.
func (w *RuntimeWorker) NewExpiryScheduler(interval time.Duration, logger *slog.Logger) (*ExpiryScheduler, error) {
	return NewExpiryScheduler(ExpirySchedulerConfig{Scanner: w.expiry, Interval: interval, Logger: logger})
}
