package ui

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestRasterRowsClipWithoutMovingLayout(t *testing.T) {
	c := NewRaster(640, 288, 720, 540)
	c.Rect(0, 0, 640, 288, 0xffffff)
	rows := c.Rows(40, 30)
	rows.Rect(0, -10, 640, 100, 0xff0000)
	if rows.Width != 640 || rows.Height != 30 {
		t.Fatal("layout geometry changed")
	}
	for y := 0; y < 540; y++ {
		for x := 0; x < 720; x++ {
			pixel := c.Pixels[(y*720+x)*4 : (y*720+x+1)*4]
			want := []byte{255, 255, 255, 0}
			if y >= 75 && y < 131 {
				want = []byte{0, 0, 255, 0}
			}
			if !bytes.Equal(pixel, want) {
				t.Fatalf("unexpected pixel at %d,%d: %v", x, y, pixel)
			}
		}
	}
}

func TestRasterImageKeepsDetailAndLogicalCrop(t *testing.T) {
	low := image.NewRGBA(image.Rect(0, 0, 2, 1))
	high := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := range 2 {
		for x := range 4 {
			high.SetRGBA(x, y, color.RGBA{R: byte(40 * x), A: 255})
		}
	}
	im := &RasterImage{RGBA: low, Source: high}
	crop := im.SubImage(image.Rect(1, 0, 2, 1))
	c := NewRaster(1, 1, 2, 2)
	c.Blit(crop, 0, 0, 1, 1)
	if c.Pixels[2] != 80 || c.Pixels[6] != 120 {
		t.Fatalf("high-resolution crop lost detail: %v", c.Pixels)
	}
}

func TestBlitEmptyRasterCropPreservesDestination(t *testing.T) {
	// A one-unit logical crop can contain no source pixels at fractional density.
	im := &RasterImage{
		RGBA:   image.NewRGBA(image.Rect(0, 0, 4, 2)),
		Source: image.NewRGBA(image.Rect(0, 0, 3, 2)),
	}
	crop := im.SubImage(image.Rect(0, 0, 1, 2))
	c := NewRaster(4, 2, 3, 2)
	c.Rect(0, 0, 4, 2, 0x123456)
	before := bytes.Clone(c.Pixels)
	c.Blit(crop, 1, 0, 1, 2)
	if !bytes.Equal(before, c.Pixels) {
		t.Fatal("empty source crop changed the destination")
	}
}

func TestBlitInViewportPreservesCenterAndExtendsClipping(t *testing.T) {
	viewport := NewRaster(640, 288, 480, 360)
	wide := NewRaster(640, 288, 641, 360)
	im := image.NewRGBA(image.Rect(0, 0, 100, 20))
	for i := range im.Pix {
		im.Pix[i] = 255
	}
	// A negative coordinate exercises rounding and clipping at the left edge.
	viewport.Blit(im, -23, 103, 100, 20)
	wide.BlitInViewport(viewport, im, -23, 103, 100, 20)
	for y := 0; y < 360; y++ {
		for x := 0; x < 480; x++ {
			for channel := 0; channel < 4; channel++ {
				if viewport.Pixels[(y*480+x)*4+channel] != wide.Pixels[(y*641+x+80)*4+channel] {
					t.Fatalf("center changed at %d,%d", x, y)
				}
			}
		}
	}
	if wide.Pixels[(130*641+70)*4] == 0 {
		t.Fatal("image did not extend outside viewport")
	}
}
