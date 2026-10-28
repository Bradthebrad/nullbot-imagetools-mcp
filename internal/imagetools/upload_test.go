package imagetools

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{200, 10, 10, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The smallest header Go recognizes as WebP.
var webpBytes = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 \x18\x00\x00\x00"), make([]byte, 24)...)

type editCapture struct {
	mu    sync.Mutex
	parts map[string]string // form field -> Content-Type
	names map[string]string // form field -> file name
}

// editServer stands in for the OpenAI image edit endpoint and records what
// each uploaded part was labeled.
func editServer(t *testing.T, result []byte) *editCapture {
	t.Helper()
	capture := &editCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		capture.mu.Lock()
		capture.parts, capture.names = map[string]string{}, map[string]string{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				capture.mu.Unlock()
				http.Error(w, err.Error(), 400)
				return
			}
			if part.FileName() != "" {
				capture.parts[part.FormName()] = part.Header.Get("Content-Type")
				capture.names[part.FormName()] = part.FileName()
			}
			io.Copy(io.Discard, part)
		}
		capture.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"data":[{"b64_json":"`+base64.StdEncoding.EncodeToString(result)+`"}]}`)
	}))
	t.Cleanup(server.Close)
	old := openAIImageEditsURL
	openAIImageEditsURL = server.URL
	t.Cleanup(func() { openAIImageEditsURL = old })
	t.Setenv("OPENAI_API_KEY", "test-key-not-real")
	return capture
}

func callEdit(t *testing.T, tools *ImageTools, args map[string]any) error {
	t.Helper()
	_, err := tools.editImageOpenAITool().Handler(context.Background(), args)
	return err
}

func TestEditUploadsCarryTheirImageType(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{"in.png": pngBytes(t), "in.jpg": jpegBytes(t), "in.webp": webpBytes, "mask.png": pngBytes(t), "mislabeled.jpg": pngBytes(t)}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tools, err := New(Config{Workspace: dir})
	if err != nil {
		t.Fatal(err)
	}
	capture := editServer(t, pngBytes(t))
	for _, tc := range []struct{ input, mask, want string }{
		{"in.png", "", "image/png"},
		{"in.jpg", "", "image/jpeg"},
		{"in.webp", "", "image/webp"},
		{"mislabeled.jpg", "", "image/png"}, // the bytes decide, not the extension
		{"in.png", "mask.png", "image/png"},
	} {
		args := map[string]any{"input_path": tc.input, "output_path": "out/" + tc.input + ".png", "prompt": "make it blue"}
		if tc.mask != "" {
			args["mask_path"] = tc.mask
		}
		if err := callEdit(t, tools, args); err != nil {
			t.Fatalf("%s: %v", tc.input, err)
		}
		capture.mu.Lock()
		if got := capture.parts["image"]; got != tc.want {
			t.Errorf("%s: image part labeled %q, want %q", tc.input, got, tc.want)
		}
		if capture.names["image"] != tc.input {
			t.Errorf("%s: file name %q", tc.input, capture.names["image"])
		}
		if tc.mask != "" && capture.parts["mask"] != "image/png" {
			t.Errorf("mask labeled %q", capture.parts["mask"])
		}
		capture.mu.Unlock()
		if _, err := os.Stat(filepath.Join(dir, "out", tc.input+".png")); err != nil {
			t.Errorf("%s: edited picture not saved: %v", tc.input, err)
		}
	}
}

func TestEditRefusesBadInputsBeforeUploading(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "in.png"), pngBytes(t), 0o600)
	os.WriteFile(filepath.Join(dir, "mask.jpg"), jpegBytes(t), 0o600)
	os.WriteFile(filepath.Join(dir, "notes.png"), []byte("not a picture at all"), 0o600)
	os.WriteFile(filepath.Join(dir, "big.png"), append(pngBytes(t), make([]byte, 4096)...), 0o600)
	tools, err := New(Config{Workspace: dir, MaxImageBytes: 2048})
	if err != nil {
		t.Fatal(err)
	}
	capture := editServer(t, pngBytes(t))
	for name, tc := range map[string]struct {
		args map[string]any
		want string
	}{
		"jpeg mask":     {map[string]any{"input_path": "in.png", "mask_path": "mask.jpg"}, "mask: must be a PNG"},
		"missing mask":  {map[string]any{"input_path": "in.png", "mask_path": "nope/mask.png"}, "mask"},
		"escaping mask": {map[string]any{"input_path": "in.png", "mask_path": "../mask.png"}, "outside the workspace"},
		"not a picture": {map[string]any{"input_path": "notes.png"}, "not a PNG, JPEG, WebP or GIF"},
		"too large":     {map[string]any{"input_path": "big.png"}, "over the 2048-byte limit"},
	} {
		tc.args["output_path"] = "out.png"
		tc.args["prompt"] = "x"
		err := callEdit(t, tools, tc.args)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, tc.want)
		}
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.parts != nil {
		t.Errorf("a refused edit still reached the provider: %v", capture.parts)
	}
}

func TestPathsMayBeAbsoluteInsideTheWorkspace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "in.png"), pngBytes(t), 0o600)
	tools, err := New(Config{Workspace: dir})
	if err != nil {
		t.Fatal(err)
	}
	inside, err := tools.resolve(filepath.Join(tools.root, "in.png"))
	if err != nil || filepath.Base(inside) != "in.png" {
		t.Fatalf("absolute path inside the workspace: %q, %v", inside, err)
	}
	if runtime.GOOS == "windows" {
		upper := strings.ToUpper(filepath.Join(tools.root, "in.png"))
		if _, err := tools.resolve(upper); err != nil {
			t.Fatalf("a differently cased path inside the workspace was refused: %v", err)
		}
	}
	if _, err := tools.resolve(filepath.Join(t.TempDir(), "in.png")); err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("absolute path outside the workspace: %v", err)
	}
	if _, err := tools.resolve("sub/../../in.png"); err == nil {
		t.Fatal("relative escape accepted")
	}
	if p, err := tools.resolve("new/dir/out.png"); err != nil || !strings.HasPrefix(p, tools.root) {
		t.Fatalf("a path that doesn't exist yet: %q, %v", p, err)
	}
}

func TestLinksCannotLeadOutOfTheWorkspace(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.png"), pngBytes(t), 0o600)
	if err := os.Symlink(filepath.Join(outside, "secret.png"), filepath.Join(dir, "link.png")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "out")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	tools, err := New(Config{Workspace: dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"link.png", "out/secret.png", "out/new.png"} {
		if _, err := tools.resolve(path); err == nil || !strings.Contains(err.Error(), "through a link") {
			t.Errorf("%s: err = %v", path, err)
		}
	}
}
