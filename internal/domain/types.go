package domain

import "time"

type Area struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Photo struct {
	ID         int64
	AreaID     int64
	StorageKey string
	MimeType   string
	UploadedAt time.Time
}

type Item struct {
	ID        int64
	AreaID    int64
	PhotoID   *int64
	Name      string
	Quantity  string
	Notes     string
	CreatedAt time.Time
}
