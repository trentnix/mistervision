package caption

import (
	"image"
	"image/color"
	"math"

	"github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/vector"
)

// rasterize draws vector outlines directly at logical pixel height. Vertical
// scaling accounts for tall CRT pixels without changing shaping or line breaks.
func rasterize(lines []shaping.Line, width, size int, scaleY float32) *image.NRGBA {
	mask := rasterMask(lines, width, size, scaleY, true)
	if mask == nil {
		return nil
	}
	return outlineMask(mask, 2, max(1, int(math.Ceil(float64(2*scaleY)))))
}

// rasterMask shares glyph coverage while allowing labels to align left.
func rasterMask(lines []shaping.Line, width, size int, scaleY float32, centered bool) *image.Alpha {
	return rasterMaskDensity(lines, width, size, scaleY, centered, 1, 1)
}

func rasterMaskDensity(lines []shaping.Line, width, size int, scaleY float32, centered bool, sx, sy float32) *image.Alpha {
	if len(lines) == 0 {
		return nil
	}
	const padding = 3
	var baselines []float32
	height := float32(padding)
	for _, line := range lines {
		ascent, descent := float32(size), float32(0)
		for _, run := range line {
			ascent = max(ascent, float32(run.GlyphBounds.Ascent)/64)
			descent = max(descent, -float32(run.GlyphBounds.Descent)/64)
		}
		baselines = append(baselines, height+ascent)
		height += ascent + descent + 4
	}
	height += padding
	mask := image.NewAlpha(image.Rect(0, 0, max(1, int(math.Round(float64(float32(width)*sx)))), max(1, int(math.Round(math.Ceil(float64(height*scaleY))*float64(sy))))))
	var path vector.Rasterizer
	path.Reset(mask.Rect.Dx(), mask.Rect.Dy())
	for index, line := range lines {
		advance := float32(0)
		for _, run := range line {
			advance += float32(run.Advance) / 64
		}
		x := float32(padding)
		if centered {
			x = (float32(width) - advance) / 2
		}
		for _, run := range line {
			scale := float32(run.Size) / 64 / float32(run.Face.Upem())
			for _, glyph := range run.Glyphs {
				outline, _ := run.Face.GlyphDataOutline(glyph.GlyphID)
				originX := x + float32(glyph.XOffset)/64
				originY := baselines[index] - float32(glyph.YOffset)/64
				for _, segment := range outline.Segments {
					var points [3]opentype.SegmentPoint
					for i, p := range segment.Args {
						points[i] = opentype.SegmentPoint{X: (originX + p.X*scale) * sx, Y: (originY - p.Y*scale) * scaleY * sy}
					}
					switch segment.Op {
					case opentype.SegmentOpMoveTo:
						path.ClosePath()
						path.MoveTo(points[0].X, points[0].Y)
					case opentype.SegmentOpLineTo:
						path.LineTo(points[0].X, points[0].Y)
					case opentype.SegmentOpQuadTo:
						path.QuadTo(points[0].X, points[0].Y, points[1].X, points[1].Y)
					case opentype.SegmentOpCubeTo:
						path.CubeTo(points[0].X, points[0].Y, points[1].X, points[1].Y, points[2].X, points[2].Y)
					}
				}
				path.ClosePath()
				x += float32(glyph.Advance) / 64
			}
		}
	}
	path.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})
	return mask
}

// outlineMask expands glyph coverage into a black outline, then composites white
// glyphs over it. Straight alpha preserves antialiasing over both dark and light video.
func outlineMask(mask *image.Alpha, radiusX, radiusY int) *image.NRGBA {
	out := image.NewNRGBA(mask.Bounds())
	for y := 0; y < mask.Rect.Dy(); y++ {
		for x := 0; x < mask.Rect.Dx(); x++ {
			alpha := byte(0)
			for yy := max(0, y-radiusY); yy <= min(mask.Rect.Dy()-1, y+radiusY); yy++ {
				for xx := max(0, x-radiusX); xx <= min(mask.Rect.Dx()-1, x+radiusX); xx++ {
					alpha = max(alpha, mask.Pix[yy*mask.Stride+xx])
				}
			}
			if alpha == 0 {
				continue
			}
			white := int(mask.Pix[y*mask.Stride+x])
			// White over black, both with coverage alpha. Use straight channels.
			a := white + int(alpha)*(255-white)/255
			value := byte(white * 255 / a)
			i := y*out.Stride + x*4
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = value, value, value, byte(a)
		}
	}
	return out
}
