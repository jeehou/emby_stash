package main

import (
	"image"
	"math"
	"sort"

	"github.com/disintegration/imaging"
)

// coverPosition recognizes a narrow spine bounded by two sustained vertical
// edges near the middle of a wraparound cover. The primary face chooses the
// panel; neither left nor right is preferred. Ambiguous images keep face crop.
func coverPosition(img image.Image, face float64) (float64, bool) {
	b := img.Bounds()
	aspect := float64(b.Dx()) / float64(b.Dy())
	if aspect < 1.25 || aspect > 1.65 || math.Abs(face-.5) < .12 {
		return face, false
	}
	if b.Dx() > 400 {
		img = imaging.Resize(img, 400, 0, imaging.Linear)
	}
	b = img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 80 || h < 80 {
		return face, false
	}
	type edge struct {
		x     int
		score float64
	}
	var edges []edge
	for x := int(.38 * float64(w)); x < int(.62*float64(w)); x++ {
		var diffs []float64
		strong := 0
		for y := h / 20; y < h-h/20; y += 2 {
			var left, right [3]float64
			for d := 1; d <= 2; d++ {
				r, g, blue, _ := img.At(b.Min.X+x-d, b.Min.Y+y).RGBA()
				left[0] += float64(r>>8) / 2
				left[1] += float64(g>>8) / 2
				left[2] += float64(blue>>8) / 2
				r, g, blue, _ = img.At(b.Min.X+x+d, b.Min.Y+y).RGBA()
				right[0] += float64(r>>8) / 2
				right[1] += float64(g>>8) / 2
				right[2] += float64(blue>>8) / 2
			}
			diff := math.Max(math.Abs(left[0]-right[0]), math.Max(math.Abs(left[1]-right[1]), math.Abs(left[2]-right[2])))
			diffs = append(diffs, diff)
			if diff > 30 {
				strong++
			}
		}
		support := float64(strong) / float64(len(diffs))
		median := medianValue(diffs)
		if support >= .60 && median >= 50 {
			edges = append(edges, edge{x, support * median})
		}
	}
	// Collapse adjacent peaks and bound the pair search even on busy collages.
	sort.Slice(edges, func(i, j int) bool { return edges[i].score > edges[j].score })
	var peaks []edge
	leftCount, rightCount := 0, 0
	for _, e := range edges {
		if e.x == w/2 || e.x < w/2 && leftCount >= 8 || e.x > w/2 && rightCount >= 8 {
			continue
		}
		near := false
		for _, p := range peaks {
			if math.Abs(float64(e.x-p.x)) <= 3 {
				near = true
				break
			}
		}
		if !near {
			peaks = append(peaks, e)
			if e.x < w/2 {
				leftCount++
			} else {
				rightCount++
			}
		}
	}
	best, left, right := 0.0, 0, 0
	for _, a := range peaks {
		for _, z := range peaks {
			width := z.x - a.x
			if a.x >= w/2 || z.x <= w/2 || width < max(6, w/50) || width > w/10 {
				continue
			}
			spine := bandLuma(img, a.x+2, z.x-2)
			before := bandLuma(img, a.x-width, a.x-2)
			after := bandLuma(img, z.x+2, z.x+width)
			// A spine should form a distinct dark or light band, not just two
			// unrelated edges on a face, clothing or scenery.
			if !(spine+15 < math.Min(before, after) || spine-15 > math.Max(before, after)) {
				continue
			}
			score := math.Min(a.score, z.score) / (1 + 5*math.Abs(float64(a.x+z.x)/float64(2*w)-.5))
			if score > best {
				best, left, right = score, a.x, z.x
			}
		}
	}
	if best == 0 {
		return face, false
	}
	start, end := 0.0, float64(left-2)/float64(w)
	if face > .5 {
		start, end = float64(right+2)/float64(w), 1
	}
	cropWidth := posterRatio / aspect
	if end-start < cropWidth || face < start || face > end {
		return face, false
	}
	center := (start + end) / 2
	// Keep the face comfortably inside the resulting crop. The panel boundary
	// alone is not enough evidence to sacrifice the detected subject.
	if math.Abs(center-face) > cropWidth*.35 {
		return face, false
	}
	return center, true
}

func medianValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	return values[len(values)/2]
}

func bandLuma(img image.Image, start, end int) float64 {
	b := img.Bounds()
	var values []float64
	for y := b.Dy() / 20; y < b.Dy()-b.Dy()/20; y += 4 {
		for x := max(0, start); x < min(b.Dx(), end); x += 2 {
			r, g, blue, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			values = append(values, .299*float64(r>>8)+.587*float64(g>>8)+.114*float64(blue>>8))
		}
	}
	return medianValue(values)
}
