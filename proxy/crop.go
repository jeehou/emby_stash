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

	"github.com/metatube-community/metatube-sdk-go/detector"
	"github.com/metatube-community/metatube-sdk-go/imageutil"
	_ "golang.org/x/image/webp"
)

const posterRatio = 2.0 / 3.0
const algorithmVersion = "metatube-fdadc9c-auto-v1-q90"
const maxImageBytes = 16 << 20
const maxImagePixels = 24_000_000

type ImageResult struct {
	Body              []byte
	ContentType, Mode string
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
	pos, found := detector.FindPrimaryFaceAxisRatio(img, posterRatio, true)
	if !found {
		return ImageResult{Body: data, Mode: "original-no-face"}, nil
	}
	if math.IsNaN(pos) || math.IsInf(pos, 0) || pos < 0 || pos > 1 {
		return out, fmt.Errorf("invalid detected position")
	}
	cropped := imageutil.CropImagePosition(img, posterRatio, pos)
	var buf bytes.Buffer
	if err = jpeg.Encode(&buf, cropped, &jpeg.Options{Quality: 90}); err != nil {
		return out, errors.New("image encode failed")
	}
	return ImageResult{Body: buf.Bytes(), ContentType: "image/jpeg", Mode: "auto", Position: pos}, nil
}
