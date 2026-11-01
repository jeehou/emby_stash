package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	"github.com/disintegration/imaging"
)

func wraparound(t *testing.T) image.Image {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 800, 538))
	draw.Draw(im, im.Bounds(), &image.Uniform{color.RGBA{160, 150, 130, 255}}, image.Point{}, draw.Src)
	draw.Draw(im, image.Rect(380, 0, 420, 538), &image.Uniform{color.RGBA{15, 20, 20, 255}}, image.Point{}, draw.Src)
	face := testFace(t)
	draw.Draw(im, image.Rect(470, 50, 590, 180), face, face.Bounds().Min, draw.Src)
	return im
}

func TestCoverExcludesSpineOnEitherSide(t *testing.T) {
	for _, mirrored := range []bool{false, true} {
		im := wraparound(t)
		if mirrored {
			im = imaging.FlipH(im)
		}
		got, err := cropPoster(imageBytes(t, im))
		if err != nil {
			t.Fatal(err)
		}
		if got.Strategy != "cover-panel" {
			t.Fatalf("mirrored=%v: expected cover panel, got %s at %.3f", mirrored, got.Strategy, got.Position)
		}
		left := int(800*got.Position) - 179
		if !mirrored && left < 420 || mirrored && left+358 > 380 {
			t.Fatalf("crop includes spine/back cover: mirrored=%v, x=%d", mirrored, left)
		}
	}
}

func TestCoverDetectionConservativeFallbacks(t *testing.T) {
	plain := image.NewRGBA(image.Rect(0, 0, 800, 538))
	draw.Draw(plain, plain.Bounds(), &image.Uniform{color.RGBA{160, 150, 130, 255}}, image.Point{}, draw.Src)
	singleEdge := imaging.Clone(plain)
	draw.Draw(singleEdge, image.Rect(400, 0, 800, 538), &image.Uniform{color.RGBA{15, 20, 20, 255}}, image.Point{}, draw.Src)
	for _, tc := range []struct {
		name string
		img  image.Image
		face float64
	}{
		{"no boundary", plain, .66},
		{"single edge", singleEdge, .66},
		{"center face", wraparound(t), .5},
		{"ordinary widescreen", imaging.Resize(wraparound(t), 960, 540, imaging.Linear), .66},
		{"face near outer edge", wraparound(t), .98},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pos, ok := coverPosition(tc.img, tc.face)
			if ok || math.Abs(pos-tc.face) > 1e-6 {
				t.Fatal("ambiguous cover must preserve face crop")
			}
		})
	}
}
