package platform

import "fmt"

// RasterPresenter accepts browsing frames whose raster is independent of the
// logical layout. Video frames continue to use the normal Presenter contract.
type RasterPresenter interface {
	RasterSize() (width, height int)
	PresentRaster(pixels []byte, width, height int) error
}

// RasterSize returns the preferred browsing raster or the logical fallback.
func RasterSize(d Presenter) (int, int) {
	if raster, ok := d.(RasterPresenter); ok {
		return raster.RasterSize()
	}
	g := d.Geometry()
	return g.Width, g.Height
}

// PresentRaster presents a sized browsing raster. Zero dimensions mean the
// existing logical frame contract. Unsupported sizes fail rather than stretch
// an incorrectly interpreted pixel buffer.
func PresentRaster(d Presenter, pixels []byte, width, height int) error {
	g := d.Geometry()
	if width == 0 && height == 0 || width == g.Width && height == g.Height {
		return d.Present(pixels)
	}
	if raster, ok := d.(RasterPresenter); ok {
		return raster.PresentRaster(pixels, width, height)
	}
	return fmt.Errorf("presenter cannot accept a %dx%d browsing raster", width, height)
}

// FullRasterPresenter accepts a composed frame covering the entire display,
// independently of the normal browsing viewport.
type FullRasterPresenter interface {
	PresentFullRaster(pixels []byte, width, height int) error
}
