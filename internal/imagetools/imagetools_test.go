package imagetools

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tinychain/mcp"
)

func TestResolveRejectsEscape(t *testing.T) {
	tools, err := New(Config{Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tools.resolve("../nope.png"); err == nil {
		t.Fatal("expected path escape error")
	}
}

func TestInspectAndResizeImage(t *testing.T) {
	dir := t.TempDir()
	writeTestImage(t, filepath.Join(dir, "input.png"), 80, 40)
	tools, err := New(Config{Workspace: dir})
	if err != nil {
		t.Fatal(err)
	}
	callTool(t, tools.inspectImageTool(), map[string]any{"path": "input.png"})
	callTool(t, tools.resizeImageTool(), map[string]any{
		"input_path":  "input.png",
		"output_path": "out/resized.png",
		"width":       float64(40),
		"height":      float64(40),
		"mode":        "contain",
	})
	if _, err := os.Stat(filepath.Join(dir, "out", "resized.png")); err != nil {
		t.Fatal(err)
	}
}

func TestContactSheetAndPDF(t *testing.T) {
	dir := t.TempDir()
	writeTestImage(t, filepath.Join(dir, "a.png"), 60, 60)
	writeTestImage(t, filepath.Join(dir, "b.png"), 60, 60)
	tools, err := New(Config{Workspace: dir})
	if err != nil {
		t.Fatal(err)
	}
	callTool(t, tools.makeContactSheetTool(), map[string]any{
		"image_paths": []any{"a.png", "b.png"},
		"output_path": "sheet.png",
		"captions":    true,
	})
	callTool(t, tools.makeImagePDFTool(), map[string]any{
		"image_paths": []any{"a.png", "b.png"},
		"output_path": "book.pdf",
		"page_size":   "letter",
	})
	for _, name := range []string{"sheet.png", "book.pdf"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
}

func TestCollectImageURLs(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"images":[{"image_url":{"url":"data:image/png;base64,abc"}}]}}]}`)
	urls := collectImageURLs(raw)
	if len(urls) != 1 || !strings.HasPrefix(urls[0], "data:image/png") {
		t.Fatalf("unexpected urls: %v", urls)
	}
}

func callTool(t *testing.T, tool mcp.Tool, args map[string]any) mcp.ToolResult {
	t.Helper()
	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Fatal("tool returned no content")
	}
	assertJSON(t, result.Content[0].Text)
	return result
}

func writeTestImage(t *testing.T, path string, width, height int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func assertJSON(t *testing.T, text string) {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, text)
	}
}
