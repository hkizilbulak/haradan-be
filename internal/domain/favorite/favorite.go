// Package favorite holds the user–advert favorite relation model.
package favorite

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrDuplicate is returned when (user_id, advert_id) already exists.
var ErrDuplicate = errors.New("favorite already exists")

// Favorite is one hrd_favorites row. There is no soft-delete column; remove is a
// hard delete of the relation. Uniqueness is (user_id, advert_id).
type Favorite struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	AdvertID  int64
	CreatedAt time.Time
}
