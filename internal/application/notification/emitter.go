package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

// AdvertNotificationEmitter is the narrow port advert moderation uses.
type AdvertNotificationEmitter interface {
	OnAdvertPublished(ctx context.Context, tx pgx.Tx, advertID uuid.UUID) error
}

// PackagingNotificationEmitter is the narrow port packaging uses.
type PackagingNotificationEmitter interface {
	OnPackageAssignedWhilePublished(ctx context.Context, tx pgx.Tx, advertID, assignmentID uuid.UUID) error
	OnUrgentActivated(ctx context.Context, tx pgx.Tx, advertID, assignmentID uuid.UUID, activationVersion int) error
}

// Emitter implements advert and packaging notification hooks.
type Emitter struct {
	writer   *EventWriter
	adverts  AdvertSnapshotReader
	packages PackageSnapshotReader
	clock    Clock
}

// EmitterConfig wires Emitter dependencies.
type EmitterConfig struct {
	Writer   *EventWriter
	Adverts  AdvertSnapshotReader
	Packages PackageSnapshotReader
	Clock    Clock
}

// NewEmitter constructs an Emitter.
func NewEmitter(cfg EmitterConfig) (*Emitter, error) {
	if cfg.Writer == nil || cfg.Adverts == nil || cfg.Packages == nil {
		return nil, fmt.Errorf("notification emitter dependencies are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	return &Emitter{writer: cfg.Writer, adverts: cfg.Adverts, packages: cfg.Packages, clock: clock}, nil
}

// OnAdvertPublished emits package-broadcast and urgent events when an advert is
// published. The caller already transitioned the advert to PUBLISHED inside
// tx (not yet committed), so this must NOT re-read advert status through the
// non-tx AdvertSnapshotReader to decide whether to emit: that read would go
// through a different connection and would still see the pre-publish status,
// silently dropping the notification. The caller's own transition guard is
// the only gate; this method trusts it. Packaging assignment/urgent reads
// below stay non-tx because those rows were committed in an earlier,
// unrelated transaction (they pre-exist relative to this publish).
func (e *Emitter) OnAdvertPublished(ctx context.Context, tx pgx.Tx, advertID uuid.UUID) error {
	now := e.clock.Now().UTC()
	if asg, _, ok, err := EffectiveBroadcastAssignment(ctx, e.packages, advertID, now); err != nil {
		return err
	} else if ok {
		if err := e.writer.WritePackageAdvertPublished(ctx, tx, WritePackageAdvertPublishedInput{
			AdvertID: advertID, AssignmentID: asg.ID,
		}); err != nil {
			return err
		}
	}
	urgent, err := e.packages.FindActiveUrgent(ctx, advertID)
	if err != nil {
		if isNotFoundErr(err) {
			return nil
		}
		return err
	}
	return e.writer.WriteUrgentAdvertActivated(ctx, tx, WriteUrgentAdvertActivatedInput{
		AdvertID:          advertID,
		AssignmentID:      urgent.PackageAssignmentID,
		ActivationVersion: urgent.ActivationVersion,
	})
}

// OnPackageAssignedWhilePublished emits when a broadcast-capable package is
// assigned to a published advert.
func (e *Emitter) OnPackageAssignedWhilePublished(ctx context.Context, tx pgx.Tx, advertID, assignmentID uuid.UUID) error {
	advert, err := e.adverts.GetAdvertSnapshot(ctx, advertID)
	if err != nil {
		return err
	}
	if advert.Status != string(domainadvert.StatusPublished) {
		return nil
	}
	asg, err := e.packages.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		return err
	}
	pkg, err := e.packages.GetPackageByID(ctx, asg.PackageID)
	if err != nil {
		return err
	}
	if !pkg.EmitsPublishBroadcast() {
		return nil
	}
	return e.writer.WritePackageAdvertPublished(ctx, tx, WritePackageAdvertPublishedInput{
		AdvertID: advertID, AssignmentID: assignmentID,
	})
}

// OnUrgentActivated emits when URGENT is activated on a published advert.
func (e *Emitter) OnUrgentActivated(ctx context.Context, tx pgx.Tx, advertID, assignmentID uuid.UUID, activationVersion int) error {
	advert, err := e.adverts.GetAdvertSnapshot(ctx, advertID)
	if err != nil {
		return err
	}
	if advert.Status != string(domainadvert.StatusPublished) {
		return nil
	}
	return e.writer.WriteUrgentAdvertActivated(ctx, tx, WriteUrgentAdvertActivatedInput{
		AdvertID:          advertID,
		AssignmentID:      assignmentID,
		ActivationVersion: activationVersion,
	})
}

func isNotFoundErr(err error) bool {
	ae, ok := apperr.As(err)
	return ok && ae.Kind == apperr.KindNotFound
}
