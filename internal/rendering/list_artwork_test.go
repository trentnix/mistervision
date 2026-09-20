package rendering

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"testing"
	"time"

	"mistervision/internal/media"
	"mistervision/internal/ui"
)

func TestListArtworkCenteredWithoutMovingRows(t *testing.T) {
	for _, height := range []int{240, 288} {
		for _, shape := range []struct {
			name, kind    string
			width, height int
		}{{"album", "MusicAlbum", 100, 100}, {"movie", "Movie", 80, 120}, {"artist", "MusicArtist", 160, 90}} {
			t.Run(fmt.Sprintf("%s/%d", shape.name, height), func(t *testing.T) {
				art := image.NewRGBA(image.Rect(0, 0, shape.width, shape.height))
				draw.Draw(art, art.Bounds(), image.NewUniform(color.RGBA{R: 200, G: 20, B: 90, A: 255}), image.Point{}, draw.Src)
				scene := Scene{ListMode: true, Now: time.Unix(0, 0), Content: Content{Title: "Library", Page: media.Page{Items: []media.Item{{ID: "one", Name: "A sample title", Type: shape.kind, RunTimeTicks: 67 * 600000000, ChildCount: 3}}}}}
				r := NewRenderer()
				before := bytes.Clone(r.Render(640, height, scene).UI)
				scene.Artwork.Primary = art
				frame := r.Render(640, height, scene).UI
				minY, maxY := height, -1
				for y := range height {
					left := y * 640 * 4
					right := left + (640-24-listSideWidth)*4
					if !bytes.Equal(before[left:right], frame[left:right]) {
						t.Fatal("artwork arrival moved or changed the list")
					}
					for x := 640 - 24 - listSideWidth; x < 640-24; x++ {
						i := (y*640 + x) * 4
						if frame[i] == 90 && frame[i+1] == 20 && frame[i+2] == 200 {
							minY, maxY = min(minY, y), max(maxY, y)
						}
					}
				}
				box := r.cache.coverBox
				offset := (minY + maxY + 1) - (box.Min.Y + box.Max.Y)
				if maxY < minY || minY < box.Min.Y || maxY >= box.Max.Y || (offset < -2 || offset > 2) {
					t.Fatalf("artwork not centered within %v: rows %d..%d", box, minY, maxY)
				}
				if dir := os.Getenv("LIST_ARTWORK_PREVIEW_DIR"); dir != "" && height == 240 {
					canvas := ui.New(640, height)
					copy(canvas.Pixels, frame)
					writeSetupPreview(t, dir, shape.name+".png", canvas)
				}
			})
		}
	}
}
