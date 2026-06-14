package imagetools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"tinychain/mcp"
)

func (t *ImageTools) makeImagePDFTool() mcp.Tool {
	return mcp.Tool{
		Name:        "make_image_pdf",
		Description: "Build a PDF from workspace-relative images. Useful for coloring books, workbooks, proofs, and image packets.",
		InputSchema: schema(map[string]any{
			"image_paths": stringArrayProp("Workspace-relative image paths, in page order."),
			"output_path": stringProp("Workspace-relative PDF output path."),
			"title":       stringProp("Optional title for PDF metadata and first page heading."),
			"page_size":   stringProp("letter or a4. Default: letter."),
			"orientation": stringProp("portrait or landscape. Default: portrait."),
			"captions":    boolProp("Add image filename captions."),
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
			pageSize := strings.ToUpper(textArg(args, "page_size"))
			if pageSize == "" {
				pageSize = "LETTER"
			}
			orientation := "P"
			if strings.HasPrefix(strings.ToLower(textArg(args, "orientation")), "land") {
				orientation = "L"
			}
			pdf := gofpdf.New(orientation, "mm", pageSize, "")
			pdf.SetTitle(textArg(args, "title"), true)
			for _, rel := range paths {
				full, err := t.resolve(rel)
				if err != nil {
					return mcp.ToolResult{}, err
				}
				if _, err := os.Stat(full); err != nil {
					return mcp.ToolResult{}, err
				}
				img, _, err := t.readImage(full)
				if err != nil {
					return mcp.ToolResult{}, err
				}
				pdf.AddPage()
				pageW, pageH := pdf.GetPageSize()
				margin := 12.0
				usableW := pageW - margin*2
				usableH := pageH - margin*2
				if boolArg(args, "captions") {
					usableH -= 10
				}
				iw := float64(img.Bounds().Dx())
				ih := float64(img.Bounds().Dy())
				scale := minFloat(usableW/iw, usableH/ih)
				w := iw * scale
				h := ih * scale
				x := (pageW - w) / 2
				y := margin
				pdf.ImageOptions(full, x, y, w, h, false, gofpdf.ImageOptions{ImageType: strings.TrimPrefix(strings.ToUpper(filepath.Ext(full)), ".")}, 0, "")
				if boolArg(args, "captions") {
					pdf.SetFont("Helvetica", "", 9)
					pdf.SetXY(margin, y+h+3)
					pdf.CellFormat(usableW, 6, filepath.Base(rel), "", 0, "C", false, 0, "")
				}
			}
			if err := pdf.OutputFileAndClose(output); err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"output_path": t.rel(output), "pages": len(paths)})), nil
		},
	}
}
