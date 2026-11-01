package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/disintegration/imaging"
)

func testFace(t *testing.T) image.Image {
	t.Helper()
	f, err := os.Open("testdata/astronaut.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return imaging.Crop(img, image.Rect(165, 50, 285, 180))
}
func canvas() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 600, 400))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{130, 145, 150, 255}}, image.Point{}, draw.Src)
	return img
}
func imageBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRealFaceLeftCenterRight(t *testing.T) {
	face := testFace(t)
	for _, tc := range []struct {
		name string
		x    int
		want float64
	}{{"left", 40, .167}, {"center", 240, .5}, {"right", 440, .833}} {
		t.Run(tc.name, func(t *testing.T) {
			img := canvas()
			draw.Draw(img, image.Rect(tc.x, 100, tc.x+120, 230), face, face.Bounds().Min, draw.Src)
			got, err := cropPoster(imageBytes(t, img))
			if err != nil {
				t.Fatal(err)
			}
			if got.Mode != "auto" {
				t.Fatalf("face not detected: %s", got.Mode)
			}
			if math.Abs(got.Position-tc.want) > .12 {
				t.Fatalf("center %.3f, expected near %.3f", got.Position, tc.want)
			}
			cfg, _, err := image.DecodeConfig(bytes.NewReader(got.Body))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Width != 266 || cfg.Height != 400 {
				t.Fatalf("unexpected crop: %dx%d", cfg.Width, cfg.Height)
			}
			t.Logf("detected axis %.3f; output %dx%d", got.Position, cfg.Width, cfg.Height)
		})
	}
}
func TestDominantFaceGroup(t *testing.T) {
	face := testFace(t)
	img := canvas()
	small := imaging.Resize(face, 50, 0, imaging.Lanczos)
	large := imaging.Resize(face, 180, 0, imaging.Lanczos)
	draw.Draw(img, image.Rect(40, 120, 90, 120+small.Bounds().Dy()), small, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(370, 90, 550, 90+large.Bounds().Dy()), large, image.Point{}, draw.Src)
	got, err := cropPoster(imageBytes(t, img))
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "auto" || got.Position < .6 {
		t.Fatalf("dominant right face not selected: %+v", got.Position)
	}
}
func TestNoFacePreservesExactOriginal(t *testing.T) {
	b := imageBytes(t, canvas())
	got, err := cropPoster(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "original-no-face" || !bytes.Equal(got.Body, b) {
		t.Fatal("no-face image was changed")
	}
}
func TestMalformedImageReturnsError(t *testing.T) {
	if _, err := cropPoster([]byte("not an image")); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestRealFaceRotation(t *testing.T) {
	face := imaging.Rotate90(testFace(t))
	img := canvas()
	draw.Draw(img, image.Rect(220, 120, 220+face.Bounds().Dx(), 120+face.Bounds().Dy()), face, image.Point{}, draw.Src)
	v, err := cropPoster(imageBytes(t, img))
	if err != nil {
		t.Fatal(err)
	}
	if v.Mode != "auto" {
		t.Fatal("rotated face not detected")
	}
}
