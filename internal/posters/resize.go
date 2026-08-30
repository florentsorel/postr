package posters

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
	// Registers the WEBP decoder. There is no encoder, so a resized WEBP comes
	// back out as JPEG — see Resize.
	_ "golang.org/x/image/webp"
)

// jpegQuality is high enough that re-encoding a poster is not visible at the
// sizes Postr stores.
const jpegQuality = 90

// Resize scales an image down to maxWidth, preserving its aspect ratio, and
// returns the encoded result with the extension it should be stored under.
//
// An image already at or below maxWidth is returned untouched: re-encoding it
// would cost quality for nothing. The extension can change — WEBP has a decoder
// but no encoder in the Go ecosystem, so a resized WEBP is returned as JPEG.
func Resize(data []byte, ext string, maxWidth int) ([]byte, string, error) {
	if maxWidth <= 0 {
		return data, ext, nil
	}

	src, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	if bounds.Dx() <= maxWidth {
		return data, ext, nil
	}

	// Round the height rather than truncating, so a 3158x4737 poster keeps its
	// proportions instead of drifting by a pixel.
	height := (bounds.Dy()*maxWidth + bounds.Dx()/2) / bounds.Dx()
	if height < 1 {
		height = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, maxWidth, height))
	// CatmullRom is the sharpest of the stock kernels, which matters when the
	// reduction factor is large — posters are often downscaled 3x or more.
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if format == "png" {
		if err := png.Encode(&buf, dst); err != nil {
			return nil, "", fmt.Errorf("encode png: %w", err)
		}
		return buf.Bytes(), "png", nil
	}
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, "", fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), "jpg", nil
}
