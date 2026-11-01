package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"testing"

	"github.com/disintegration/imaging"
)

func addTestBarcode(img draw.Image, x, y int) {
	// EAN-13 style bars with white quiet zones. No private cover fixtures.
	const bars = "10100010110100111011001100100110111101001110101010110011011011001000010101110010011101000100101"
	draw.Draw(img, image.Rect(x-12, y, x+len(bars)+12, y+32), image.White, image.Point{}, draw.Src)
	for i, c := range bars {
		if c == '1' {
			draw.Draw(img, image.Rect(x+i, y, x+i+1, y+32), image.Black, image.Point{}, draw.Src)
		}
	}
}

func barcodeCover(t *testing.T, frontFace bool) *image.NRGBA {
	t.Helper()
	im := imaging.New(800, 538, color.NRGBA{160, 150, 130, 255})
	face := testFace(t)
	// The back contains many more faces than the front. A primary-face vote
	// across the entire scan must not determine which panel is the poster.
	for _, p := range []image.Point{{25, 20}, {210, 20}, {25, 210}, {210, 210}} {
		draw.Draw(im, image.Rect(p.X, p.Y, p.X+120, p.Y+130), face, face.Bounds().Min, draw.Src)
	}
	if frontFace {
		small := imaging.Resize(face, 75, 0, imaging.Linear)
		draw.Draw(im, image.Rect(620, 35, 695, 117), small, small.Bounds().Min, draw.Src)
	}
	addTestBarcode(im, 140, 480)
	return im
}

func TestBarcodeExcludesBackCollageOnEitherSide(t *testing.T) {
	for _, mirror := range []bool{false, true} {
		im := barcodeCover(t, true)
		if mirror {
			im = imaging.FlipH(im)
		}
		// Exercise lossy cover scans as well as the actual detector/crop path.
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, im, &jpeg.Options{Quality: 85}); err != nil {
			t.Fatal(err)
		}
		got, err := cropPoster(buf.Bytes())
		if err != nil || got.Mode != "auto" || got.Strategy != "cover-barcode" {
			t.Fatalf("mirror=%v mode=%s strategy=%s err=%v", mirror, got.Mode, got.Strategy, err)
		}
		left := int(800*got.Position) - 179
		if !mirror && left < 424 || mirror && left+358 > 376 {
			t.Fatalf("crop crosses into back/spine: mirror=%v x=%d", mirror, left)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(got.Body))
		if err != nil || cfg.Width != 358 || cfg.Height != 538 {
			t.Fatalf("wrong output dimensions: %+v, %v", cfg, err)
		}
	}
}

func TestBarcodeRequiresFrontFace(t *testing.T) {
	data := imageBytes(t, barcodeCover(t, false))
	got, err := cropPoster(data)
	if err != nil || got.Mode != "original-no-face" || !bytes.Equal(data, got.Body) {
		t.Fatalf("back-cover faces cannot substitute for a front face: mode=%s err=%v", got.Mode, err)
	}
}

func TestBarcodeConservativeFallbacks(t *testing.T) {
	plain := imaging.New(800, 538, color.White)
	both := imaging.Clone(plain)
	addTestBarcode(both, 140, 480)
	addTestBarcode(both, 550, 480)
	top := imaging.Clone(plain)
	addTestBarcode(top, 140, 30)
	fewBars := imaging.Clone(plain)
	for x := 140; x < 210; x += 10 {
		draw.Draw(fewBars, image.Rect(x, 480, x+4, 512), image.Black, image.Point{}, draw.Src)
	}
	broken := imaging.Clone(plain)
	addTestBarcode(broken, 140, 480)
	for y := 480; y < 512; y += 4 {
		draw.Draw(broken, image.Rect(125, y, 250, y+2), image.White, image.Point{}, draw.Src)
	}
	noQuiet := imaging.Clone(plain)
	addTestBarcode(noQuiet, 140, 480)
	draw.Draw(noQuiet, image.Rect(125, 480, 140, 512), image.Black, image.Point{}, draw.Src)
	for name, im := range map[string]image.Image{
		"no bars": plain, "both sides": both, "upper bars": top,
		"few stripes": fewBars, "broken vertical bars": broken, "no quiet zone": noQuiet,
		"video proportions": imaging.Resize(barcodeCover(t, true), 960, 540, imaging.Linear),
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := barcodeFrontPanel(im); ok {
				t.Fatal("ambiguous layout must not select a front panel")
			}
		})
	}
}

func TestBarcodeScaledAndOffsetBounds(t *testing.T) {
	im := imaging.Resize(barcodeCover(t, true), 1600, 1076, imaging.NearestNeighbor)
	offset := image.NewRGBA(im.Bounds().Add(image.Pt(17, 23)))
	draw.Draw(offset, offset.Bounds(), im, im.Bounds().Min, draw.Src)
	panel, ok := barcodeFrontPanel(offset)
	if !ok || panel != image.Rect(865, 23, 1617, 1099) {
		t.Fatalf("incorrect source coordinates: %v, %v", panel, ok)
	}
}
