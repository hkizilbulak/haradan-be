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
	OnAdvertPublished(ctx context.Context, tx pgx.Tx, advertID int64) error
	OnAdvertPriceDropped(ctx context.Context, tx pgx.Tx, advertID int64, oldPrice int64, newPrice int64) error
}

// PackagingNotificationEmitter is the narrow port packaging uses.
type PackagingNotificationEmitter interface {
	OnPackageAssignedWhilePublished(ctx context.Context, tx pgx.Tx, advertID int64, assignmentID uuid.UUID) error
	OnUrgentActivated(ctx context.Context, tx pgx.Tx, advertID int64, assignmentID uuid.UUID, activationVersion int) error
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
// published.
func (e *Emitter) OnAdvertPublished(ctx context.Context, tx pgx.Tx, advertID int64) error {
	packages := e.packages
	if tx != nil {
		packages = packages.WithTx(tx)
	}
	now := e.clock.Now().UTC()
	if asg, _, ok, err := EffectiveBroadcastAssignment(ctx, packages, advertID, now); err != nil {
		return err
	} else if ok {
		if err := e.writer.WritePackageAdvertPublished(ctx, tx, WritePackageAdvertPublishedInput{
			AdvertID: advertID, AssignmentID: asg.ID,
		}); err != nil {
			return err
		}
	}
	urgent, err := packages.FindActiveUrgent(ctx, advertID)
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

// OnAdvertPriceDropped emits an event when an advert's price drops.
func (e *Emitter) OnAdvertPriceDropped(ctx context.Context, tx pgx.Tx, advertID int64, oldPrice int64, newPrice int64) error {
	return e.writer.WriteAdvertPriceDropped(ctx, tx, WriteAdvertPriceDroppedInput{
		AdvertID: advertID,
		OldPrice: oldPrice,
		NewPrice: newPrice,
	})
}

// OnPackageAssignedWhilePublished emits when a broadcast-capable package is
// assigned to a published advert.
func (e *Emitter) OnPackageAssignedWhilePublished(ctx context.Context, tx pgx.Tx, advertID int64, assignmentID uuid.UUID) error {
	adverts := e.adverts
	if tx != nil {
		adverts = adverts.WithTx(tx)
	}
	advert, err := adverts.GetAdvertSnapshot(ctx, advertID)
	if err != nil {
		return err
	}
	if advert.Status != string(domainadvert.StatusPublished) {
		return nil
	}
	packages := e.packages
	if tx != nil {
		packages = packages.WithTx(tx)
	}
	asg, err := packages.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		return err
	}
	pkg, err := packages.GetPackageByID(ctx, asg.PackageID)
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
func (e *Emitter) OnUrgentActivated(ctx context.Context, tx pgx.Tx, advertID int64, assignmentID uuid.UUID, activationVersion int) error {
	adverts := e.adverts
	if tx != nil {
		adverts = adverts.WithTx(tx)
	}
	advert, err := adverts.GetAdvertSnapshot(ctx, advertID)
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
