package packaging

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

const (
	packageNotFoundMessage    = "Paket bulunamadı."
	assignmentNotFoundMessage = "Aktif paket ataması bulunamadı."
	activationNotFoundMessage = "Aktif URGENT aktivasyonu bulunamadı."
	assignmentConflictMessage = "Paket ataması aynı anda başka bir işlem tarafından güncellendi."
	activationConflictMessage = "URGENT aktivasyonu aynı anda başka bir işlem tarafından güncellendi."
)

const packageColumns = `id, code, display_name, description, badge_text, benefits,
display_price_amount_minor, currency_code, default_duration_days, allows_urgent,
showcase_eligible, featured_days, search_priority, broadcast_on_publish, is_active,
sort_order, version, created_at, updated_at`

const assignmentColumns = `id, advert_id, package_id, status, starts_at, ends_at,
assigned_by_user_id, assigned_at, superseded_at, expired_at, cancelled_at, reason,
source, version, created_at, updated_at`

const featureColumns = `id, advert_id, package_assignment_id, feature_code, status,
activated_by_user_id, activated_at, ends_at, deactivated_at, reason, activation_version,
created_at, updated_at`

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository persists packages, assignments, and feature activations.
type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

// NewRepository constructs a packaging repository bound to a pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, db: pool}
}

// WithTx returns a repository scoped to a transaction querier.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{pool: r.pool, db: tx}
}

// BeginTx starts a read-write transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if r.pool == nil {
		return nil, apperr.Internal(errors.New("packaging repository has no pool"))
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("begin packaging tx: %w", pg.SanitizeErr(err)))
	}
	return tx, nil
}

// FindPackageByID loads a package by id.
func (r *Repository) FindPackageByID(ctx context.Context, id uuid.UUID) (domainpackaging.Package, error) {
	q := `SELECT ` + packageColumns + ` FROM hrd_packages WHERE id = $1`
	row := r.db.QueryRow(ctx, q, id)
	p, err := scanPackage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.Package{}, apperr.NotFound(packageNotFoundMessage)
	}
	if err != nil {
		return domainpackaging.Package{}, apperr.Internal(fmt.Errorf("find package: %w", pg.SanitizeErr(err)))
	}
	return p, nil
}

// FindPackageByCode loads a package by code.
func (r *Repository) FindPackageByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error) {
	q := `SELECT ` + packageColumns + ` FROM hrd_packages WHERE code = $1`
	row := r.db.QueryRow(ctx, q, string(code))
	p, err := scanPackage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.Package{}, apperr.NotFound(packageNotFoundMessage)
	}
	if err != nil {
		return domainpackaging.Package{}, apperr.Internal(fmt.Errorf("find package by code: %w", pg.SanitizeErr(err)))
	}
	return p, nil
}

// LockPackageByCode locks a package row by code.
func (r *Repository) LockPackageByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error) {
	q := `SELECT ` + packageColumns + ` FROM hrd_packages WHERE code = $1 FOR UPDATE`
	row := r.db.QueryRow(ctx, q, string(code))
	p, err := scanPackage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.Package{}, apperr.NotFound(packageNotFoundMessage)
	}
	if err != nil {
		return domainpackaging.Package{}, apperr.Internal(fmt.Errorf("lock package by code: %w", pg.SanitizeErr(err)))
	}
	return p, nil
}

// CreatePackage inserts a new package catalog row.
func (r *Repository) CreatePackage(ctx context.Context, p domainpackaging.Package) error {
	const q = `
INSERT INTO hrd_packages (
  id, code, display_name, description, badge_text, benefits,
  display_price_amount_minor, currency_code, default_duration_days, allows_urgent,
  showcase_eligible, featured_days, search_priority, broadcast_on_publish, is_active,
  sort_order, version, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19
)`
	_, err := r.db.Exec(ctx, q,
		p.ID, string(p.Code), p.DisplayName, p.Description, p.BadgeText, p.BenefitsJSON,
		p.DisplayPriceAmountMinor, p.CurrencyCode, p.DefaultDurationDays, p.AllowsUrgent,
		p.ShowcaseEligible, p.FeaturedDays, p.SearchPriority, p.BroadcastOnPublish, p.IsActive, p.SortOrder,
		p.Version, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("Bu paket kodu zaten kullanılıyor.")
		}
		if isCheckViolation(err) {
			return apperr.Validation("Paket oluşturma geçersiz.")
		}
		return apperr.Internal(fmt.Errorf("create package: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// UpdatePackageOptimistic updates a package when version matches.
func (r *Repository) UpdatePackageOptimistic(
	ctx context.Context,
	p domainpackaging.Package,
	expectedVersion int,
) (domainpackaging.Package, error) {
	const q = `
UPDATE hrd_packages
SET display_name = $3,
    description = $4,
    badge_text = $5,
    benefits = $6,
    display_price_amount_minor = $7,
    currency_code = $8,
    default_duration_days = $9,
    allows_urgent = $10,
    showcase_eligible = $11,
    featured_days = $12,
    search_priority = $13,
    broadcast_on_publish = $14,
    is_active = $15,
    sort_order = $16,
    updated_at = $17,
    version = version + 1
WHERE code = $1 AND version = $2
RETURNING ` + packageColumns
	row := r.db.QueryRow(ctx, q,
		string(p.Code), expectedVersion,
		p.DisplayName, p.Description, p.BadgeText, p.BenefitsJSON,
		p.DisplayPriceAmountMinor, p.CurrencyCode, p.DefaultDurationDays,
		p.AllowsUrgent, p.ShowcaseEligible, p.FeaturedDays, p.SearchPriority, p.BroadcastOnPublish,
		p.IsActive, p.SortOrder,
		p.UpdatedAt,
	)
	out, err := scanPackage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.Package{}, apperr.StaleVersion(stalePackageVersionMessage)
	}
	if err != nil {
		if isCheckViolation(err) {
			return domainpackaging.Package{}, apperr.Validation("Paket güncellemesi geçersiz.")
		}
		return domainpackaging.Package{}, apperr.Internal(fmt.Errorf("update package: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

const stalePackageVersionMessage = "Paket başka bir işlem tarafından güncellendi."

// MarkAssignmentCancelled marks an assignment CANCELLED.
func (r *Repository) MarkAssignmentCancelled(
	ctx context.Context,
	id uuid.UUID,
	cancelledAt, updatedAt time.Time,
	reason *string,
) error {
	const q = `
UPDATE hrd_advert_package_assignments
SET status = 'CANCELLED',
    cancelled_at = $2,
    reason = COALESCE($4, reason),
    updated_at = $3,
    version = version + 1
WHERE id = $1 AND status = 'ACTIVE'`
	tag, err := r.db.Exec(ctx, q, id, cancelledAt, updatedAt, reason)
	if err != nil {
		return apperr.Internal(fmt.Errorf("cancel assignment: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound(assignmentNotFoundMessage)
	}
	return nil
}

// DeactivateActiveUrgentForPackage deactivates ACTIVE URGENT rows linked to
// ACTIVE assignments of the given package.
func (r *Repository) DeactivateActiveUrgentForPackage(
	ctx context.Context,
	packageID uuid.UUID,
	deactivatedAt time.Time,
	reason *string,
	updatedAt time.Time,
) (int64, error) {
	const q = `
UPDATE hrd_advert_feature_activations AS fa
SET status = 'DEACTIVATED',
    deactivated_at = $2,
    reason = COALESCE($3, fa.reason),
    updated_at = $4
FROM hrd_advert_package_assignments AS a
WHERE fa.package_assignment_id = a.id
  AND a.package_id = $1
  AND a.status = 'ACTIVE'
  AND fa.feature_code = 'URGENT'
  AND fa.status = 'ACTIVE'`
	tag, err := r.db.Exec(ctx, q, packageID, deactivatedAt, reason, updatedAt)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("deactivate urgent for package: %w", pg.SanitizeErr(err)))
	}
	return tag.RowsAffected(), nil
}

// ListPackages returns packages ordered by sort_order ASC, code ASC.
func (r *Repository) ListPackages(ctx context.Context, includeInactive bool) ([]domainpackaging.Package, error) {
	q := `SELECT ` + packageColumns + ` FROM hrd_packages`
	if !includeInactive {
		q += ` WHERE is_active = true`
	}
	q += ` ORDER BY sort_order ASC, code ASC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list packages: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainpackaging.Package, 0)
	for rows.Next() {
		p, err := scanPackage(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan package: %w", pg.SanitizeErr(err)))
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate packages: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// FindActiveAssignmentByAdvertID loads the ACTIVE assignment for an advert.
func (r *Repository) FindActiveAssignmentByAdvertID(
	ctx context.Context,
	advertID int64,
) (domainpackaging.AdvertPackageAssignment, error) {
	q := `SELECT ` + assignmentColumns + `
FROM hrd_advert_package_assignments
WHERE advert_id = $1 AND status = 'ACTIVE'`
	row := r.db.QueryRow(ctx, q, advertID)
	a, err := scanAssignment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.AdvertPackageAssignment{}, apperr.NotFound(assignmentNotFoundMessage)
	}
	if err != nil {
		return domainpackaging.AdvertPackageAssignment{}, apperr.Internal(fmt.Errorf("find active assignment: %w", pg.SanitizeErr(err)))
	}
	return a, nil
}

// FindEffectiveActiveAssignmentByAdvertID loads ACTIVE covering instant at.
func (r *Repository) FindEffectiveActiveAssignmentByAdvertID(
	ctx context.Context,
	advertID int64,
	at time.Time,
) (domainpackaging.AdvertPackageAssignment, error) {
	q := `SELECT ` + assignmentColumns + `
FROM hrd_advert_package_assignments
WHERE advert_id = $1
  AND status = 'ACTIVE'
  AND starts_at <= $2
  AND (ends_at IS NULL OR ends_at > $2)`
	row := r.db.QueryRow(ctx, q, advertID, at)
	a, err := scanAssignment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.AdvertPackageAssignment{}, apperr.NotFound(assignmentNotFoundMessage)
	}
	if err != nil {
		return domainpackaging.AdvertPackageAssignment{}, apperr.Internal(fmt.Errorf("find effective assignment: %w", pg.SanitizeErr(err)))
	}
	return a, nil
}

// LockActiveAssignmentByAdvertID locks the ACTIVE assignment row.
func (r *Repository) LockActiveAssignmentByAdvertID(
	ctx context.Context,
	advertID int64,
) (domainpackaging.AdvertPackageAssignment, error) {
	q := `SELECT ` + assignmentColumns + `
FROM hrd_advert_package_assignments
WHERE advert_id = $1 AND status = 'ACTIVE'
FOR UPDATE`
	row := r.db.QueryRow(ctx, q, advertID)
	a, err := scanAssignment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.AdvertPackageAssignment{}, apperr.NotFound(assignmentNotFoundMessage)
	}
	if err != nil {
		return domainpackaging.AdvertPackageAssignment{}, apperr.Internal(fmt.Errorf("lock active assignment: %w", pg.SanitizeErr(err)))
	}
	return a, nil
}

// ListAssignmentHistoryByAdvertID returns history newest first with keyset paging.
func (r *Repository) ListAssignmentHistoryByAdvertID(
	ctx context.Context,
	advertID int64,
	afterAssignedAt *time.Time,
	afterID *uuid.UUID,
	limit int,
) ([]domainpackaging.AdvertPackageAssignment, error) {
	q := `SELECT ` + assignmentColumns + `
FROM hrd_advert_package_assignments
WHERE advert_id = $1`
	args := []any{advertID}
	if afterAssignedAt != nil && afterID != nil {
		q += ` AND (assigned_at, id) < ($2, $3)`
		args = append(args, *afterAssignedAt, *afterID)
	}
	q += ` ORDER BY assigned_at DESC, id DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list assignment history: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainpackaging.AdvertPackageAssignment, 0)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan assignment: %w", pg.SanitizeErr(err)))
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate assignments: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// CreateAssignment inserts a new assignment row.
func (r *Repository) CreateAssignment(ctx context.Context, a domainpackaging.AdvertPackageAssignment) error {
	const q = `
INSERT INTO hrd_advert_package_assignments (
  id, advert_id, package_id, status, starts_at, ends_at, assigned_by_user_id, assigned_at,
  superseded_at, expired_at, cancelled_at, reason, source, version, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16
)`
	_, err := r.db.Exec(ctx, q,
		a.ID, a.AdvertID, a.PackageID, string(a.Status), a.StartsAt, a.EndsAt, a.AssignedByUserID, a.AssignedAt,
		a.SupersededAt, a.ExpiredAt, a.CancelledAt, a.Reason, string(a.Source), a.Version, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict(assignmentConflictMessage)
		}
		if isCheckViolation(err) {
			return apperr.Validation("Paket ataması geçersiz.")
		}
		if isForeignKeyViolation(err) {
			return apperr.Validation("Paket ataması ilişkisi geçersiz.")
		}
		return apperr.Internal(fmt.Errorf("create assignment: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// MarkAssignmentSuperseded marks an assignment SUPERSEDED.
func (r *Repository) MarkAssignmentSuperseded(
	ctx context.Context,
	id uuid.UUID,
	supersededAt, updatedAt time.Time,
) error {
	const q = `
UPDATE hrd_advert_package_assignments
SET status = 'SUPERSEDED',
    superseded_at = $2,
    updated_at = $3,
    version = version + 1
WHERE id = $1 AND status = 'ACTIVE'`
	tag, err := r.db.Exec(ctx, q, id, supersededAt, updatedAt)
	if err != nil {
		return apperr.Internal(fmt.Errorf("supersede assignment: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound(assignmentNotFoundMessage)
	}
	return nil
}

// FindActiveFeatureByAdvertIDAndCode loads an ACTIVE feature activation.
func (r *Repository) FindActiveFeatureByAdvertIDAndCode(
	ctx context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
) (domainpackaging.AdvertFeatureActivation, error) {
	q := `SELECT ` + featureColumns + `
FROM hrd_advert_feature_activations
WHERE advert_id = $1 AND feature_code = $2 AND status = 'ACTIVE'`
	row := r.db.QueryRow(ctx, q, advertID, string(code))
	a, err := scanFeature(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.AdvertFeatureActivation{}, apperr.NotFound(activationNotFoundMessage)
	}
	if err != nil {
		return domainpackaging.AdvertFeatureActivation{}, apperr.Internal(fmt.Errorf("find active feature: %w", pg.SanitizeErr(err)))
	}
	return a, nil
}

// LockActiveFeatureByAdvertIDAndCode locks an ACTIVE feature activation.
func (r *Repository) LockActiveFeatureByAdvertIDAndCode(
	ctx context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
) (domainpackaging.AdvertFeatureActivation, error) {
	q := `SELECT ` + featureColumns + `
FROM hrd_advert_feature_activations
WHERE advert_id = $1 AND feature_code = $2 AND status = 'ACTIVE'
FOR UPDATE`
	row := r.db.QueryRow(ctx, q, advertID, string(code))
	a, err := scanFeature(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainpackaging.AdvertFeatureActivation{}, apperr.NotFound(activationNotFoundMessage)
	}
	if err != nil {
		return domainpackaging.AdvertFeatureActivation{}, apperr.Internal(fmt.Errorf("lock active feature: %w", pg.SanitizeErr(err)))
	}
	return a, nil
}

// FindLatestActivationVersion returns the max activation_version or 0.
func (r *Repository) FindLatestActivationVersion(
	ctx context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
) (int, error) {
	const q = `
SELECT COALESCE(MAX(activation_version), 0)
FROM hrd_advert_feature_activations
WHERE advert_id = $1 AND feature_code = $2`
	var latest int
	if err := r.db.QueryRow(ctx, q, advertID, string(code)).Scan(&latest); err != nil {
		return 0, apperr.Internal(fmt.Errorf("latest activation version: %w", pg.SanitizeErr(err)))
	}
	return latest, nil
}

// CreateFeatureActivation inserts a feature activation row.
func (r *Repository) CreateFeatureActivation(ctx context.Context, a domainpackaging.AdvertFeatureActivation) error {
	const q = `
INSERT INTO hrd_advert_feature_activations (
  id, advert_id, package_assignment_id, feature_code, status, activated_by_user_id,
  activated_at, ends_at, deactivated_at, reason, activation_version, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13
)`
	_, err := r.db.Exec(ctx, q,
		a.ID, a.AdvertID, a.PackageAssignmentID, string(a.FeatureCode), string(a.Status), a.ActivatedByUserID,
		a.ActivatedAt, a.EndsAt, a.DeactivatedAt, a.Reason, a.ActivationVersion, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict(activationConflictMessage)
		}
		if isCheckViolation(err) {
			return apperr.Validation("URGENT aktivasyonu geçersiz.")
		}
		if isForeignKeyViolation(err) {
			return apperr.Validation("URGENT aktivasyonu ilişkisi geçersiz.")
		}
		return apperr.Internal(fmt.Errorf("create feature activation: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// DeactivateActiveFeature deactivates the ACTIVE feature row if present.
func (r *Repository) DeactivateActiveFeature(
	ctx context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
	deactivatedAt time.Time,
	reason *string,
	updatedAt time.Time,
) (bool, error) {
	const q = `
UPDATE hrd_advert_feature_activations
SET status = 'DEACTIVATED',
    deactivated_at = $3,
    reason = COALESCE($4, reason),
    updated_at = $5
WHERE advert_id = $1 AND feature_code = $2 AND status = 'ACTIVE'`
	tag, err := r.db.Exec(ctx, q, advertID, string(code), deactivatedAt, reason, updatedAt)
	if err != nil {
		return false, apperr.Internal(fmt.Errorf("deactivate feature: %w", pg.SanitizeErr(err)))
	}
	return tag.RowsAffected() > 0, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPackage(row rowScanner) (domainpackaging.Package, error) {
	var (
		p    domainpackaging.Package
		code string
	)
	err := row.Scan(
		&p.ID, &code, &p.DisplayName, &p.Description, &p.BadgeText, &p.BenefitsJSON,
		&p.DisplayPriceAmountMinor, &p.CurrencyCode, &p.DefaultDurationDays, &p.AllowsUrgent,
		&p.ShowcaseEligible, &p.FeaturedDays, &p.SearchPriority, &p.BroadcastOnPublish, &p.IsActive, &p.SortOrder,
		&p.Version, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return domainpackaging.Package{}, err
	}
	p.Code = domainpackaging.PackageCode(code)
	return p, nil
}

func scanAssignment(row rowScanner) (domainpackaging.AdvertPackageAssignment, error) {
	var (
		a      domainpackaging.AdvertPackageAssignment
		status string
		source string
	)
	err := row.Scan(
		&a.ID, &a.AdvertID, &a.PackageID, &status, &a.StartsAt, &a.EndsAt,
		&a.AssignedByUserID, &a.AssignedAt, &a.SupersededAt, &a.ExpiredAt, &a.CancelledAt, &a.Reason,
		&source, &a.Version, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return domainpackaging.AdvertPackageAssignment{}, err
	}
	a.Status = domainpackaging.AssignmentStatus(status)
	a.Source = domainpackaging.AssignmentSource(source)
	return a, nil
}

func scanFeature(row rowScanner) (domainpackaging.AdvertFeatureActivation, error) {
	var (
		a           domainpackaging.AdvertFeatureActivation
		featureCode string
		status      string
	)
	err := row.Scan(
		&a.ID, &a.AdvertID, &a.PackageAssignmentID, &featureCode, &status,
		&a.ActivatedByUserID, &a.ActivatedAt, &a.EndsAt, &a.DeactivatedAt, &a.Reason, &a.ActivationVersion,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return domainpackaging.AdvertFeatureActivation{}, err
	}
	a.FeatureCode = domainpackaging.FeatureCode(featureCode)
	a.Status = domainpackaging.FeatureActivationStatus(status)
	return a, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}
