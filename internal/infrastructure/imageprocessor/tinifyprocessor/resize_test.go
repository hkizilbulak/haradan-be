package tinifyprocessor

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"testing"
)

func TestFitDimensions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		srcW, srcH   int
		maxW, maxH   int
		wantW, wantH int
	}{
		{name: "landscape", srcW: 800, srcH: 400, maxW: 200, maxH: 200, wantW: 200, wantH: 100},
		{name: "portrait", srcW: 400, srcH: 800, maxW: 200, maxH: 200, wantW: 100, wantH: 200},
		{name: "square", srcW: 500, srcH: 500, maxW: 200, maxH: 200, wantW: 200, wantH: 200},
		{name: "exact", srcW: 100, srcH: 80, maxW: 100, maxH: 80, wantW: 100, wantH: 80},
		{name: "no_upscale", srcW: 50, srcH: 40, maxW: 200, maxH: 200, wantW: 50, wantH: 40},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotW, gotH, err := fitDimensions(tc.srcW, tc.srcH, tc.maxW, tc.maxH)
			if err != nil {
				t.Fatal(err)
			}
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Fatalf("got %dx%d want %dx%d", gotW, gotH, tc.wantW, tc.wantH)
			}
			if gotW > tc.maxW || gotH > tc.maxH {
				t.Fatalf("exceeded bounds: %dx%d > %dx%d", gotW, gotH, tc.maxW, tc.maxH)
			}
			if gotW < 1 || gotH < 1 {
				t.Fatalf("zero dimension: %dx%d", gotW, gotH)
			}
			srcRatio := float64(tc.srcW) / float64(tc.srcH)
			gotRatio := float64(gotW) / float64(gotH)
			diff := srcRatio - gotRatio
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.02 {
				t.Fatalf("aspect drift: src=%v got=%v", srcRatio, gotRatio)
			}
		})
	}
}

func TestResizeFitPreservesFormatAndBounds(t *testing.T) {
	t.Parallel()

	jpegBytes := mustEncodeJPEG(t, 400, 200)
	pngBytes := mustEncodePNG(t, 400, 200)

	jpegSrc, err := decodeImage(jpegBytes)
	if err != nil {
		t.Fatal(err)
	}
	out, w, h, err := resizeFit(jpegSrc, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if w != 100 || h != 50 {
		t.Fatalf("jpeg dims %dx%d", w, h)
	}
	if ct, err := canonicalContentType(http.DetectContentType(out)); err != nil || ct != "image/jpeg" {
		t.Fatalf("jpeg content type %q err=%v", ct, err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != w || cfg.Height != h {
		t.Fatalf("encoded dims %dx%d want %dx%d", cfg.Width, cfg.Height, w, h)
	}

	pngSrc, err := decodeImage(pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	outPNG, pw, ph, err := resizeFit(pngSrc, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if pw != 100 || ph != 50 {
		t.Fatalf("png dims %dx%d", pw, ph)
	}
	if ct, err := canonicalContentType(http.DetectContentType(outPNG)); err != nil || ct != "image/png" {
		t.Fatalf("png content type %q err=%v", ct, err)
	}
}

func TestResizeFitNoUpscale(t *testing.T) {
	t.Parallel()

	srcBytes := mustEncodePNG(t, 40, 30)
	src, err := decodeImage(srcBytes)
	if err != nil {
		t.Fatal(err)
	}
	out, w, h, err := resizeFit(src, 400, 400)
	if err != nil {
		t.Fatal(err)
	}
	if w != 40 || h != 30 {
		t.Fatalf("upscaled unexpectedly to %dx%d", w, h)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 40 || cfg.Height != 30 {
		t.Fatalf("encoded %dx%d", cfg.Width, cfg.Height)
	}
}

func mustEncodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func mustEncodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: uint8(x % 255), B: uint8(y % 255), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
