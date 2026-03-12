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

// ItemSource indicates how an item was originally created.
type ItemSource string

const (
	ItemSourceAI   ItemSource = "ai"
	ItemSourceUser ItemSource = "user"
)

type Item struct {
	ID        int64
	AreaID    int64
	PhotoID   *int64
	Name      string
	Quantity  string
	Source    ItemSource
	BBoxX1    *float64
	BBoxY1    *float64
	BBoxX2    *float64
	BBoxY2    *float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ItemEdit records a single field change made by a user.
type ItemEdit struct {
	ID       int64
	ItemID   int64
	Field    string
	OldValue string
	NewValue string
	EditedAt time.Time
}
