package room

import (
	"context"
	"sync"

	"github.com/sivchari/chat-example/pkg/domain/entity"
	"github.com/sivchari/chat-example/pkg/domain/repository/room"
)

type repository struct {
	mapByID map[string]*entity.Room
	// TODO: sync.RWMutexとの違いを考えて最適化しよう
	mu sync.Mutex
}

func New() room.Repository {
	return &repository{
		mapByID: make(map[string]*entity.Room, 0),
	}
}

func (r *repository) Insert(_ context.Context, room *entity.Room) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mapByID[room.ID] = room
	return nil
}

func (r *repository) Select(_ context.Context, id string) (*entity.Room, error) {
	return r.mapByID[id], nil
}

func (r *repository) SelectAll(_ context.Context) ([]*entity.Room, error) {
	return nil, nil
}
