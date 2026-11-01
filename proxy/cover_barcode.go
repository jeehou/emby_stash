package main

import (
	"image"
	"math"

	"github.com/disintegration/imaging"
)

// barcodeFrontPanel uses the retail barcode as evidence for the BACK panel.
// Low resolution cover scans often cannot decode the digits, so this recognizes
// a quiet-zone-bounded run of dense, vertically stable dark/light bars instead.
// It is deliberately limited to wraparound proportions and the lower panel.
// Both panels containing bars (or neither) is ambiguous: keep the face strategy.
func barcodeFrontPanel(img image.Image) (image.Rectangle, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	aspect := float64(w) / float64(h)
	if aspect < 1.4 || aspect > 1.65 || w < 400 || h < 250 {
		return image.Rectangle{}, false
	}
	probe := img
	if w > 800 {
		probe = imaging.Resize(img, 800, 0, imaging.Linear)
	}
	gray := imaging.Grayscale(probe)
	left, right := panelHasBarcode(gray, 0), panelHasBarcode(gray, 1)
	if left == right {
		return image.Rectangle{}, false
	}
	// Reserve the central gutter/spine, symmetrically. Only use the panel if it
	// can hold a full-height 2:3 poster; never enlarge the crop into the back.
	panel := image.Rect(0, 0, w*47/100, h)
	if left {
		panel = image.Rect(w-w*47/100, 0, w, h)
	}
	if panel.Dx() < int(float64(h)*posterRatio) {
		return image.Rectangle{}, false
	}
	return panel.Add(b.Min), true
}

func panelHasBarcode(gray *image.NRGBA, side int) bool {
	w, h := gray.Bounds().Dx(), gray.Bounds().Dy()
	height := max(12, h/30)
	gap := max(4, w/100)
	quiet := max(4, w/125)
	means, variances := make([]float64, w), make([]float64, w)
	lo, hi := side*w/2+2, (side+1)*w/2-2
	for y := h * 3 / 5; y < h-height; y += 2 {
		for x := lo; x < hi; x++ {
			sum, squares := 0.0, 0.0
			for dy := 0; dy < height; dy++ {
				v := float64(gray.Pix[(y+dy)*gray.Stride+x*4])
				sum += v
				squares += v * v
			}
			means[x] = sum / float64(height)
			variances[x] = squares/float64(height) - means[x]*means[x]
		}
		start, last, sign, count := 0, 0, 0, 0
		valid := func() bool {
			if count < 24 || sign != 1 || last-start < w/20 || last-start > w/4 || start-quiet < lo || last+quiet >= hi {
				return false
			}
			// Quiet zones distinguish a bounded printed barcode from text or
			// a large striped background. Allow antialiasing at the first bar.
			for d := 2; d <= quiet; d++ {
				for _, x := range []int{start - d, last + d} {
					if means[x] < 165 || variances[x] > 22*22 {
						return false
					}
				}
			}
			return true
		}
		for x := lo + 1; x < hi; x++ {
			diff := means[x] - means[x-1]
			if math.Abs(diff) <= 35 || variances[x] >= 22*22 || variances[x-1] >= 22*22 {
				continue
			}
			nextSign := 1
			if diff < 0 {
				nextSign = -1
			}
			if x-last > gap {
				if valid() {
					return true
				}
				start, count, sign = x, 0, 0
			}
			if sign != nextSign {
				count++
				sign = nextSign
			}
			last = x
		}
		if valid() {
			return true
		}
	}
	return false
}
