package posters_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/florentsorel/postr/internal/posters"
)

// encode builds a real image of the given size so the resizer has something
// decodable to work with.
func encode(t *testing.T, format string, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	var err error
	if format == "png" {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	}
	if err != nil {
		t.Fatalf("encode %s: %v", format, err)
	}
	return buf.Bytes()
}

func dimensions(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return cfg.Width, cfg.Height
}

// TestResize_ScalesDownKeepingAspectRatio uses the exact dimensions of the
// ThePosterDB asset that prompted this: 3158x4737 at ~11 MB.
func TestResize_ScalesDownKeepingAspectRatio(t *testing.T) {
	original := encode(t, "jpg", 3158, 4737)

	out, ext, err := posters.Resize(original, "jpg", 1000)
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if ext != "jpg" {
		t.Errorf("ext = %q, want jpg", ext)
	}

	w, h := dimensions(t, out)
	if w != 1000 {
		t.Errorf("width = %d, want 1000", w)
	}
	// 4737/3158 * 1000 = 1500 (rounded)
	if h != 1500 {
		t.Errorf("height = %d, want 1500 (aspect ratio preserved)", h)
	}
	if len(out) >= len(original) {
		t.Errorf("resized image is not smaller: %d -> %d bytes", len(original), len(out))
	}
}

// TestResize_LeavesSmallImagesUntouched avoids a pointless re-encode, which
// would cost quality for nothing.
func TestResize_LeavesSmallImagesUntouched(t *testing.T) {
	original := encode(t, "jpg", 800, 1200)

	out, ext, err := posters.Resize(original, "jpg", 1000)
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if !bytes.Equal(out, original) {
		t.Error("an image already under the target width should be returned byte-for-byte")
	}
	if ext != "jpg" {
		t.Errorf("ext = %q, want jpg", ext)
	}
}

func TestResize_ExactlyAtTargetIsUntouched(t *testing.T) {
	original := encode(t, "jpg", 1000, 1500)

	out, _, err := posters.Resize(original, "jpg", 1000)
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if !bytes.Equal(out, original) {
		t.Error("an image exactly at the target width should not be re-encoded")
	}
}

func TestResize_KeepsPNGAsPNG(t *testing.T) {
	out, ext, err := posters.Resize(encode(t, "png", 2000, 3000), "png", 1000)
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if ext != "png" {
		t.Errorf("ext = %q, want png preserved (transparency would be lost otherwise)", ext)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("output is not valid PNG: %v", err)
	}
}

// TestResize_ZeroWidthIsANoOp covers the setting being disabled.
func TestResize_ZeroWidthIsANoOp(t *testing.T) {
	original := encode(t, "jpg", 3000, 4500)

	out, ext, err := posters.Resize(original, "jpg", 0)
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if !bytes.Equal(out, original) || ext != "jpg" {
		t.Error("a non-positive width should leave the image alone")
	}
}

func TestResize_RejectsUndecodableData(t *testing.T) {
	if _, _, err := posters.Resize([]byte("<html>not an image</html>"), "jpg", 1000); err == nil {
		t.Error("want an error for data that is not an image")
	}
}
