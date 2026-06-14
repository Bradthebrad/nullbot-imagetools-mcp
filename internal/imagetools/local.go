package imagetools

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"tinychain/mcp"
)

func (t *ImageTools) inspectImageTool() mcp.Tool {
	return mcp.Tool{
		Name:        "inspect_image",
		Description: "Inspect an image in the workspace and return format, dimensions, size, and MIME details.",
		InputSchema: schema(map[string]any{"path": stringProp("Workspace-relative image path.")}, "path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := t.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			img, format, err := t.readImage(path)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			info, err := os.Stat(path)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			b := img.Bounds()
			return mcp.Text(pretty(map[string]any{
				"path":   t.rel(path),
				"format": format,
				"mime":   mimeFromName(path),
				"width":  b.Dx(),
				"height": b.Dy(),
				"bytes":  info.Size(),
			})), nil
		},
	}
}

func (t *ImageTools) resizeImageTool() mcp.Tool {
	return mcp.Tool{
		Name:        "resize_image",
		Description: "Resize an image to a target width/height. Mode can be contain, cover, or stretch.",
		InputSchema: schema(map[string]any{
			"input_path":  stringProp("Workspace-relative source image path."),
			"output_path": stringProp("Workspace-relative output image path."),
			"width":       numberProp("Output width in pixels."),
			"height":      numberProp("Output height in pixels."),
			"mode":        stringProp("contain, cover, or stretch. Default: contain."),
		}, "input_path", "output_path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			input, err := t.resolve(textArg(args, "input_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			output, err := t.resolveOutput(textArg(args, "output_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			img, _, err := t.readImage(input)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			width := intArg(args, "width", img.Bounds().Dx())
			height := intArg(args, "height", img.Bounds().Dy())
			if width <= 0 || height <= 0 {
				return mcp.ToolResult{}, fmt.Errorf("width and height must be positive")
			}
			out := fitImage(img, width, height, textArg(args, "mode"))
			if err := saveImage(output, out); err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"output_path": t.rel(output), "width": width, "height": height})), nil
		},
	}
}

func (t *ImageTools) cropImageTool() mcp.Tool {
	return mcp.Tool{
		Name:        "crop_image",
		Description: "Crop a rectangular region from an image.",
		InputSchema: schema(map[string]any{
			"input_path":  stringProp("Workspace-relative source image path."),
			"output_path": stringProp("Workspace-relative output image path."),
			"x":           numberProp("Left pixel."),
			"y":           numberProp("Top pixel."),
			"width":       numberProp("Crop width in pixels."),
			"height":      numberProp("Crop height in pixels."),
		}, "input_path", "output_path", "width", "height"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			input, err := t.resolve(textArg(args, "input_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			output, err := t.resolveOutput(textArg(args, "output_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			img, _, err := t.readImage(input)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			b := img.Bounds()
			x := clamp(intArg(args, "x", 0), 0, b.Dx())
			y := clamp(intArg(args, "y", 0), 0, b.Dy())
			w := clamp(intArg(args, "width", b.Dx()-x), 1, b.Dx()-x)
			h := clamp(intArg(args, "height", b.Dy()-y), 1, b.Dy()-y)
			rect := image.Rect(0, 0, w, h)
			dst := image.NewRGBA(rect)
			draw.Draw(dst, rect, img, image.Point{X: b.Min.X + x, Y: b.Min.Y + y}, draw.Src)
			if err := saveImage(output, dst); err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"output_path": t.rel(output), "x": x, "y": y, "width": w, "height": h})), nil
		},
	}
}

func (t *ImageTools) addTitleTextTool() mcp.Tool {
	return mcp.Tool{
		Name:        "add_title_text",
		Description: "Add title/subtitle text with a readable backing band to an image for thumbnails or posters.",
		InputSchema: schema(map[string]any{
			"input_path":       stringProp("Workspace-relative source image path."),
			"output_path":      stringProp("Workspace-relative output image path."),
			"title":            stringProp("Main title text."),
			"subtitle":         stringProp("Optional subtitle text."),
			"position":         stringProp("top, center, or bottom. Default: bottom."),
			"text_color":       stringProp("Hex text color. Default: #ffffff."),
			"background_color": stringProp("Hex band color. Default: #000000."),
		}, "input_path", "output_path", "title"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			input, err := t.resolve(textArg(args, "input_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			output, err := t.resolveOutput(textArg(args, "output_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			img, _, err := t.readImage(input)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			out := drawTitle(img, textArg(args, "title"), textArg(args, "subtitle"), textArg(args, "position"), textArg(args, "text_color"), textArg(args, "background_color"))
			if err := saveImage(output, out); err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"output_path": t.rel(output)})), nil
		},
	}
}

func (t *ImageTools) composeThumbnailTool() mcp.Tool {
	return mcp.Tool{
		Name:        "compose_thumbnail",
		Description: "Create a thumbnail canvas from an optional background image plus title/subtitle text. Defaults to 1280x720.",
		InputSchema: schema(map[string]any{
			"background_path":   stringProp("Optional workspace-relative background image path."),
			"output_path":       stringProp("Workspace-relative output image path."),
			"title":             stringProp("Main title text."),
			"subtitle":          stringProp("Optional subtitle text."),
			"width":             numberProp("Canvas width. Default: 1280."),
			"height":            numberProp("Canvas height. Default: 720."),
			"background_color":  stringProp("Hex fallback background. Default: #081018."),
			"title_band_color":  stringProp("Hex title band color. Default: #000000."),
			"title_text_color":  stringProp("Hex title text color. Default: #ffffff."),
			"background_fit":    stringProp("contain, cover, or stretch. Default: cover."),
			"include_safe_area": boolProp("Draw a subtle title-safe guide."),
		}, "output_path", "title"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			width := intArg(args, "width", 1280)
			height := intArg(args, "height", 720)
			if width <= 0 || height <= 0 {
				return mcp.ToolResult{}, fmt.Errorf("width and height must be positive")
			}
			output, err := t.resolveOutput(textArg(args, "output_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			canvas := image.NewRGBA(image.Rect(0, 0, width, height))
			fill(canvas, rgba(colorFromHex(textArg(args, "background_color"), 0x081018), 255))
			if bg := textArg(args, "background_path"); bg != "" {
				input, err := t.resolve(bg)
				if err != nil {
					return mcp.ToolResult{}, err
				}
				img, _, err := t.readImage(input)
				if err != nil {
					return mcp.ToolResult{}, err
				}
				draw.Draw(canvas, canvas.Bounds(), fitImage(img, width, height, textArg(args, "background_fit")), image.Point{}, draw.Src)
			}
			if boolArg(args, "include_safe_area") {
				drawRectOutline(canvas, image.Rect(width/20, height/12, width-width/20, height-height/12), color.RGBA{255, 255, 255, 80})
			}
			out := drawTitle(canvas, textArg(args, "title"), textArg(args, "subtitle"), "bottom", textArg(args, "title_text_color"), textArg(args, "title_band_color"))
			if err := saveImage(output, out); err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"output_path": t.rel(output), "width": width, "height": height})), nil
		},
	}
}

func drawTitle(src image.Image, title, subtitle, position, textHex, bgHex string) image.Image {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), src, b.Min, draw.Src)
	if strings.TrimSpace(title) == "" {
		return out
	}
	face := basicfont.Face7x13
	scale := max(2, b.Dx()/420)
	lineHeight := 13*scale + 8
	lines := wrapText(strings.ToUpper(title), max(12, b.Dx()/(7*scale)-8))
	if subtitle != "" {
		lines = append(lines, wrapText(subtitle, max(16, b.Dx()/(7*scale)-8))...)
	}
	bandHeight := clamp(lineHeight*len(lines)+36, 72, b.Dy())
	y := b.Dy() - bandHeight
	switch strings.ToLower(position) {
	case "top":
		y = 0
	case "center":
		y = (b.Dy() - bandHeight) / 2
	}
	bg := rgba(colorFromHex(bgHex, 0x000000), 190)
	draw.Draw(out, image.Rect(0, y, b.Dx(), y+bandHeight), &image.Uniform{C: bg}, image.Point{}, draw.Over)
	textColor := rgba(colorFromHex(textHex, 0xffffff), 255)
	cursorY := y + 26
	for _, line := range lines {
		drawScaledText(out, face, 24, cursorY, scale, line, textColor)
		cursorY += lineHeight
	}
	return out
}

func drawScaledText(dst *image.RGBA, face font.Face, x, y, scale int, text string, c color.Color) {
	for dy := 0; dy < scale; dy++ {
		for dx := 0; dx < scale; dx++ {
			d := font.Drawer{
				Dst:  dst,
				Src:  image.NewUniform(c),
				Face: face,
				Dot:  fixed.P(x+dx, y+dy),
			}
			d.DrawString(text)
		}
	}
}

func wrapText(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
			continue
		}
		if len(current)+1+len(word) > width {
			lines = append(lines, current)
			current = word
			continue
		}
		current += " " + word
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func rgba(rgb uint32, alpha uint8) color.RGBA {
	return color.RGBA{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb), A: alpha}
}

func drawRectOutline(dst *image.RGBA, rect image.Rectangle, c color.Color) {
	draw.Draw(dst, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+2), &image.Uniform{C: c}, image.Point{}, draw.Over)
	draw.Draw(dst, image.Rect(rect.Min.X, rect.Max.Y-2, rect.Max.X, rect.Max.Y), &image.Uniform{C: c}, image.Point{}, draw.Over)
	draw.Draw(dst, image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+2, rect.Max.Y), &image.Uniform{C: c}, image.Point{}, draw.Over)
	draw.Draw(dst, image.Rect(rect.Max.X-2, rect.Min.Y, rect.Max.X, rect.Max.Y), &image.Uniform{C: c}, image.Point{}, draw.Over)
}

func imageName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}
