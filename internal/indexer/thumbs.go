package indexer

import (
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// resizeToJPEG decodes the source image, scales it so the height matches targetHeight
// (preserving aspect ratio), and writes a JPEG to dest. Returns the dest path on success.
func resizeToJPEG(src, dest string, targetHeight int) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	srcImg, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	sb := srcImg.Bounds()
	srcW, srcH := sb.Dx(), sb.Dy()
	if srcH == 0 {
		return nil
	}
	dstH := targetHeight
	if srcH < dstH {
		dstH = srcH
	}
	dstW := srcW * dstH / srcH
	if dstW < 1 {
		dstW = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), srcImg, sb, draw.Over, nil)

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	return jpeg.Encode(out, dst, &jpeg.Options{Quality: 85})
}

func isImageExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".bmp", ".gif":
		return true
	}
	return false
}
