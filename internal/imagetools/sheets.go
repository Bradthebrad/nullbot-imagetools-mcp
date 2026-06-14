package imagetools

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"tinychain/mcp"
)

func (t *ImageTools) makeContactSheetTool() mcp.Tool {
	return mcp.Tool{
		Name:        "make_contact_sheet",
		Description: "Create a contact sheet PNG/JPEG from workspace-relative images, optionally with captions.",
		InputSchema: schema(map[string]any{
			"image_paths":  stringArrayProp("Workspace-relative image paths."),
			"output_path":  stringProp("Workspace-relative output image path."),
			"columns":      numberProp("Number of columns. Default: 3."),
			"thumb_width":  numberProp("Thumbnail width. Default: 320."),
			"thumb_height": numberProp("Thumbnail height. Default: 220."),
			"captions":     boolProp("Draw file-name captions under thumbnails."),
		}, "image_paths", "output_path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			paths := stringSliceArg(args, "image_paths")
			if len(paths) == 0 {
				return mcp.ToolResult{}, fmt.Errorf("image_paths must contain at least one path")
			}
			output, err := t.resolveOutput(textArg(args, "output_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			columns := clamp(intArg(args, "columns", 3), 1, 12)
			tw := clamp(intArg(args, "thumb_width", 320), 32, 3000)
			th := clamp(intArg(args, "thumb_height", 220), 32, 3000)
			captionHeight := 0
			if boolArg(args, "captions") {
				captionHeight = 24
			}
			rows := (len(paths) + columns - 1) / columns
			padding := 18
			canvas := image.NewRGBA(image.Rect(0, 0, columns*(tw+padding)+padding, rows*(th+captionHeight+padding)+padding))
			fill(canvas, color.RGBA{R: 12, G: 16, B: 18, A: 255})
			for i, rel := range paths {
				full, err := t.resolve(rel)
				if err != nil {
					return mcp.ToolResult{}, err
				}
				img, _, err := t.readImage(full)
				if err != nil {
					return mcp.ToolResult{}, err
				}
				x := padding + (i%columns)*(tw+padding)
				y := padding + (i/columns)*(th+captionHeight+padding)
				draw.Draw(canvas, image.Rect(x, y, x+tw, y+th), fitImage(img, tw, th, "cover"), image.Point{}, draw.Src)
				if captionHeight > 0 {
					drawText(canvas, x, y+th+17, filepath.Base(rel), color.RGBA{R: 232, G: 238, B: 241, A: 255})
				}
			}
			if err := saveImage(output, canvas); err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"output_path": t.rel(output), "images": len(paths), "columns": columns})), nil
		},
	}
}

func drawText(dst *image.RGBA, x, y int, text string, c color.Color) {
	d := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(c),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}
