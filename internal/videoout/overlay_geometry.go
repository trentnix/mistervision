package videoout

import "mistervision/internal/platform"

// ScaleOverlay fits the shared 4:3 UX within a complete video frame without
// stretching text. Aspect describes the physical screen, not its pixel shape.
// The destination is reused and cleared so earlier margins never remain visible.
func ScaleOverlay(dst, src []byte, g platform.Geometry, aspect float64) []byte {
	size := g.OutputWidth * g.OutputHeight * 4
	if len(dst) != size {
		dst = make([]byte, size)
	} else {
		clear(dst)
	}
	w, h := g.OutputWidth, g.OutputHeight
	if aspect > 4.0/3 {
		w = max(1, int(float64(w)*(4.0/3)/aspect+0.5))
	} else if aspect > 0 && aspect < 4.0/3 {
		h = max(1, int(float64(h)*aspect/(4.0/3)+0.5))
	}
	left, top := (g.OutputWidth-w)/2, (g.OutputHeight-h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s := ((y*g.Height/h)*g.Width + x*g.Width/w) * 4
			d := ((top+y)*g.OutputWidth + left + x) * 4
			copy(dst[d:d+4], src[s:s+4])
		}
	}
	return dst
}
