package browser

import (
	"mistervision/internal/media"
	"mistervision/internal/rendering"
)

// selectionData holds selected artwork. Library counts belong to home items.
type selectionData struct {
	artwork rendering.Artwork
}

// selectionUpdateKind distinguishes metadata from decoded image results.
type selectionUpdateKind uint8

const (
	selectionArtwork selectionUpdateKind = iota
	selectionDetails
)

// selectionUpdate carries one progressive result. Only the payload named by
// kind is meaningful. err is the request error for every kind. Image workers may
// emit concurrently. Producers must not mutate data after publication.
type selectionUpdate struct {
	kind   selectionUpdateKind
	art    artUpdate
	detail *media.Item
	err    error
}
