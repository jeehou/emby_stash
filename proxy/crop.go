package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"

	"github.com/disintegration/imaging"
	"github.com/metatube-community/metatube-sdk-go/detector"
	"github.com/metatube-community/metatube-sdk-go/imageutil"
	_ "golang.org/x/image/webp"
)

const posterRatio = 2.0 / 3.0
const algorithmVersion = "metatube-fdadc9c-cover-v3-q90"
const maxImageBytes = 16 << 20
const maxImagePixels = 24_000_000

type ImageResult struct {
	Body              []byte
	ContentType, Mode string
	Strategy          string
	Position          float64
	NoStore           bool
}
type CropFunc func([]byte) (ImageResult, error)

func cropPoster(data []byte) (out ImageResult, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("face detection failed")
		}
	}()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return out, errors.New("unsupported or invalid image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 12000 || cfg.Height > 12000 || int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return out, errors.New("image dimensions exceed processing limit")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return out, errors.New("image decode failed")
	}
	// Choose the front before looking for faces: a back-cover collage can have
	// more (and higher-scoring) detections than the actual front photograph.
	panel, hasBarcode := barcodeFrontPanel(img)
	detectionImage := img
	if hasBarcode {
		detectionImage = imaging.Crop(img, panel)
	}
	pos, found := detector.FindPrimaryFaceAxisRatio(detectionImage, posterRatio, true)
	if !found {
		return ImageResult{Body: data, Mode: "original-no-face"}, nil
	}
	if math.IsNaN(pos) || math.IsInf(pos, 0) || pos < 0 || pos > 1 {
		return out, fmt.Errorf("invalid detected position")
	}
	strategy := "face"
	if hasBarcode {
		b := img.Bounds()
		faceX := float64(panel.Min.X-b.Min.X) + pos*float64(panel.Dx())
		half := float64(int(float64(b.Dy())*posterRatio)) / 2
		center := math.Max(float64(panel.Min.X-b.Min.X)+half, math.Min(faceX, float64(panel.Max.X-b.Min.X)-half))
		pos, strategy = center/float64(b.Dx()), "cover-barcode"
	} else if coverPos, ok := coverPosition(img, pos); ok {
		pos, strategy = coverPos, "cover-panel"
	}
	cropped := imageutil.CropImagePosition(img, posterRatio, pos)
	var buf bytes.Buffer
	if err = jpeg.Encode(&buf, cropped, &jpeg.Options{Quality: 90}); err != nil {
		return out, errors.New("image encode failed")
	}
	return ImageResult{Body: buf.Bytes(), ContentType: "image/jpeg", Mode: "auto", Position: pos, Strategy: strategy}, nil
}
