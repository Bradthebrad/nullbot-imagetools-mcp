package imagetools

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

func init() {
	image.RegisterFormat("webp", "RIFF????WEBP", webp.Decode, webp.DecodeConfig)
}

func (t *ImageTools) readImage(path string) (image.Image, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	if info.Size() > t.maxImageBytes {
		return nil, "", fmt.Errorf("image exceeds max-image-bytes: %d > %d", info.Size(), t.maxImageBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	img, format, err := image.Decode(file)
	return img, format, err
}

func saveImage(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 92})
	case ".gif":
		return gif.Encode(file, img, nil)
	default:
		return png.Encode(file, img)
	}
}

func imageBytes(path string, limit int64) ([]byte, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	if info.Size() > limit {
		return nil, "", fmt.Errorf("image exceeds max bytes: %d > %d", info.Size(), limit)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return data, mimeFromName(path), nil
}

func decodeImageBytes(data []byte) (image.Image, string, error) {
	return image.Decode(bytes.NewReader(data))
}

func fitImage(src image.Image, width, height int, mode string) image.Image {
	if width <= 0 {
		width = src.Bounds().Dx()
	}
	if height <= 0 {
		height = src.Bounds().Dy()
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	switch strings.ToLower(mode) {
	case "stretch":
		xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	case "cover":
		sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
		scale := maxFloat(float64(width)/float64(sw), float64(height)/float64(sh))
		nw, nh := int(float64(sw)*scale), int(float64(sh)*scale)
		temp := image.NewRGBA(image.Rect(0, 0, nw, nh))
		xdraw.ApproxBiLinear.Scale(temp, temp.Bounds(), src, src.Bounds(), draw.Over, nil)
		x := (width - nw) / 2
		y := (height - nh) / 2
		draw.Draw(dst, dst.Bounds(), temp, image.Point{-x, -y}, draw.Src)
	default:
		fill(dst, color.RGBA{R: 8, G: 11, B: 13, A: 255})
		sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
		scale := minFloat(float64(width)/float64(sw), float64(height)/float64(sh))
		nw, nh := int(float64(sw)*scale), int(float64(sh)*scale)
		temp := image.NewRGBA(image.Rect(0, 0, nw, nh))
		xdraw.ApproxBiLinear.Scale(temp, temp.Bounds(), src, src.Bounds(), draw.Over, nil)
		x := (width - nw) / 2
		y := (height - nh) / 2
		draw.Draw(dst, image.Rect(x, y, x+nw, y+nh), temp, image.Point{}, draw.Over)
	}
	return dst
}

func copyToFile(path string, reader io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, reader)
	return err
}

func mimeFromName(name string) string {
	if typ := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); typ != "" {
		return strings.Split(typ, ";")[0]
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func fill(img draw.Image, c color.Color) {
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
}
