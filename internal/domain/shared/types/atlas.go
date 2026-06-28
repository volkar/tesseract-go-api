package types

// AtlasItemMeta contains specific metadata for atlas elements, like dimensions
type AtlasItemMeta struct {
	Width  *int `json:"width,omitempty"`
	Height *int `json:"height,omitempty"`
}

// AtlasItem represents an item in the album's atlas
type AtlasItem struct {
	Type string         `json:"type" validate:"required,oneof=title text image"`
	Src  string         `json:"src" validate:"required,min=1"`
	Meta *AtlasItemMeta `json:"meta,omitempty"`
}

// Atlas represents the album's atlas
type Atlas []AtlasItem
