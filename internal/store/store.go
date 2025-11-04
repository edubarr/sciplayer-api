package store

import (
	"context"
	"errors"
	"time"
)

var ErrDeviceNotFound = errors.New("device not found")

type Playlist struct {
	Name      string
	URL       string
	CreatedAt time.Time
}

type Store interface {
	CreateDevice(ctx context.Context, deviceID string) (bool, error)
	AddPlaylist(ctx context.Context, deviceID, name, playlistURL string) error
	ListPlaylists(ctx context.Context, deviceID string) ([]Playlist, error)
	Close() error
}
