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
	BBoxes    [][]float64 `json:"BBoxes,omitempty"`
	CreatedAt time.Time  `json:"CreatedAt"`
	UpdatedAt time.Time  `json:"UpdatedAt"`
}

// SnapshotItem is a lightweight item record stored inside a snapshot.
type SnapshotItem struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity,omitempty"`
}

// Snapshot captures the item list for an area at a point in time.
type Snapshot struct {
	ID      int64
	AreaID  int64
	TakenAt time.Time
	Items   []SnapshotItem
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

// OverrideRule defines a pattern-to-replacement mapping applied at upload time.
type OverrideRule struct {
	ID                   int64
	MatchPattern         string
	Replacement          string
	MatchExact           bool
	MatchCaseInsensitive bool
	MatchSubstring       bool
	Scope                string // "global" or "area"
	AreaIDs              []int64
	SortOrder            int
	CreatedAt            time.Time
}

// EditSuggestion represents a user rename that can be turned into an override rule.
type EditSuggestion struct {
	ItemID   int64
	OldName  string
	NewName  string
	AreaID   int64
	AreaName string
	EditedAt time.Time
}
