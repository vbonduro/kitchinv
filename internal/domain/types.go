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
	ID        int64      `json:"ID"`
	AreaID    int64      `json:"AreaID"`
	PhotoID   *int64     `json:"PhotoID,omitempty"`
	Name      string     `json:"Name"`
	Quantity  string     `json:"Quantity"`
	Source    ItemSource `json:"Source"`
	BBoxX1    *float64   `json:"BBoxX1,omitempty"`
	BBoxY1    *float64   `json:"BBoxY1,omitempty"`
	BBoxX2    *float64   `json:"BBoxX2,omitempty"`
	BBoxY2    *float64   `json:"BBoxY2,omitempty"`
	CreatedAt time.Time  `json:"CreatedAt"`
	UpdatedAt time.Time  `json:"UpdatedAt"`
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
