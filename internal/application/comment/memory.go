package comment

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincomment "github.com/hkizilbulak/haradan-be/internal/domain/comment"
)

type memoryRepo struct {
	mu       sync.RWMutex
	adverts  map[int64]AdvertStatusResult
	comments map[uuid.UUID]domaincomment.Comment
	users    map[uuid.UUID]string
}

// NewMemoryRepository returns a thread-safe in-memory Repository implementation for testing.
func NewMemoryRepository() *memoryRepo {
	return &memoryRepo{
		adverts:  make(map[int64]AdvertStatusResult),
		comments: make(map[uuid.UUID]domaincomment.Comment),
		users:    make(map[uuid.UUID]string),
	}
}

func (m *memoryRepo) AddAdvert(adv AdvertStatusResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adverts[adv.ID] = adv
}

func (m *memoryRepo) AddUser(userID uuid.UUID, authorName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[userID] = authorName
}

func (m *memoryRepo) FindAdvertStatus(ctx context.Context, advertID int64) (AdvertStatusResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	adv, ok := m.adverts[advertID]
	if !ok {
		return AdvertStatusResult{}, apperr.NotFound("advert not found")
	}
	return adv, nil
}

func (m *memoryRepo) InsertComment(ctx context.Context, c domaincomment.Comment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.comments[c.ID] = c
	return nil
}

func (m *memoryRepo) FindCommentByID(ctx context.Context, commentID uuid.UUID) (domaincomment.Comment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.comments[commentID]
	if !ok || c.DeletedAt != nil {
		return domaincomment.Comment{}, apperr.NotFound("comment not found")
	}
	return c, nil
}

func (m *memoryRepo) DeleteComment(ctx context.Context, commentID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.comments[commentID]
	if !ok || c.DeletedAt != nil {
		return apperr.NotFound("comment not found")
	}
	now := time.Now().UTC()
	c.DeletedAt = &now
	m.comments[commentID] = c
	return nil
}

func (m *memoryRepo) GetUserAuthorName(ctx context.Context, userID uuid.UUID) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name, ok := m.users[userID]
	if !ok {
		return "Kullanıcı", nil
	}
	return name, nil
}

func (m *memoryRepo) ListCommentsByAdvert(ctx context.Context, advertID int64, limit, offset int) ([]CommentRow, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []CommentRow
	for _, c := range m.comments {
		if c.AdvertID == advertID && c.DeletedAt == nil && c.Status == domaincomment.StatusPublished {
			authorName := m.users[c.UserID]
			if authorName == "" {
				authorName = "Kullanıcı"
			}
			list = append(list, CommentRow{
				Comment:    c,
				AuthorName: authorName,
			})
		}
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Comment.CreatedAt.Equal(list[j].Comment.CreatedAt) {
			return list[i].Comment.ID.String() > list[j].Comment.ID.String()
		}
		return list[i].Comment.CreatedAt.After(list[j].Comment.CreatedAt)
	})

	total := len(list)
	if offset >= total {
		return []CommentRow{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return list[offset:end], total, nil
}

// NewMemoryService constructs a Service backed by an in-memory repository for unit tests.
func NewMemoryService(repo Repository, opts ...Option) *Service {
	return NewService(repo, opts...)
}
