package imagetools

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

// imageContentType names a picture's real type from its first bytes, so an
// upload is labeled image/png, image/jpeg or image/webp even when the file's
// extension is wrong or missing. Image APIs reject application/octet-stream.
func imageContentType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	head = head[:n]
	switch typ := http.DetectContentType(head); typ {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return typ, nil
	}
	if len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP" {
		return "image/webp", nil
	}
	return "", fmt.Errorf("%s is not a PNG, JPEG, WebP or GIF picture", filepath.Base(path))
}

var quoteEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// addMultipartFile adds a picture to a multipart form with its real content
// type. multipart.Writer.CreateFormFile would label every file
// application/octet-stream.
func addMultipartFile(writer *multipart.Writer, field, path string) error {
	typ, err := imageContentType(path)
	if err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	if field == "mask" && typ != "image/png" {
		return fmt.Errorf("mask: must be a PNG with transparency, not %s", typ)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, quoteEscaper.Replace(field), quoteEscaper.Replace(filepath.Base(path))))
	header.Set("Content-Type", typ)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

// checkUploadSize refuses a file larger than the configured image limit
// before it is read into a request.
func (t *ImageTools) checkUploadSize(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a file", filepath.Base(path))
	}
	if info.Size() > t.maxImageBytes {
		return fmt.Errorf("%s is %d bytes, over the %d-byte limit (--max-image-bytes)", filepath.Base(path), info.Size(), t.maxImageBytes)
	}
	return nil
}
