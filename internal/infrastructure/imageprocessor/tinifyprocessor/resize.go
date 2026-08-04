package tinifyprocessor

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
)

const localJPEGQuality = 95

type decodedImage struct {
	Img         image.Image
	ContentType string
	Width       int
	Height      int
}

func fitDimensions(srcW, srcH, maxW, maxH int) (int, int, error) {
	if srcW <= 0 || srcH <= 0 {
		return 0, 0, fmt.Errorf("invalid source dimensions")
	}
	if maxW <= 0 || maxH <= 0 {
		return 0, 0, fmt.Errorf("invalid target dimensions")
	}
	if srcW <= maxW && srcH <= maxH {
		return srcW, srcH, nil
	}

	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}
	if scale > 1 {
		scale = 1
	}

	dstW := int(float64(srcW)*scale + 1e-9)
	dstH := int(float64(srcH)*scale + 1e-9)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}
	if dstW > maxW {
		dstW = maxW
	}
	if dstH > maxH {
		dstH = maxH
	}
	return dstW, dstH, nil
}

func resizeFit(src decodedImage, maxW, maxH int) ([]byte, int, int, error) {
	dstW, dstH, err := fitDimensions(src.Width, src.Height, maxW, maxH)
	if err != nil {
		return nil, 0, 0, err
	}

	outImg := src.Img
	if dstW != src.Width || dstH != src.Height {
		dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src.Img, src.Img.Bounds(), draw.Over, nil)
		outImg = dst
	}

	var buf bytes.Buffer
	switch src.ContentType {
	case "image/jpeg":
		if err := jpeg.Encode(&buf, outImg, &jpeg.Options{Quality: localJPEGQuality}); err != nil {
			return nil, 0, 0, err
		}
	case "image/png":
		if err := png.Encode(&buf, outImg); err != nil {
			return nil, 0, 0, err
		}
	default:
		return nil, 0, 0, fmt.Errorf("unsupported content type")
	}
	if buf.Len() == 0 {
		return nil, 0, 0, fmt.Errorf("empty encoded image")
	}
	return buf.Bytes(), dstW, dstH, nil
}
