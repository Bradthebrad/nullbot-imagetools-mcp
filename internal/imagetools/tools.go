package imagetools

import (
	"context"
	"os"

	"tinychain/mcp"
)

func (t *ImageTools) Tools() []mcp.Tool {
	return []mcp.Tool{
		t.imageWorkspaceInfoTool(),
		t.listImageModelsTool(),
		t.generateImageOpenAITool(),
		t.editImageOpenAITool(),
		t.generateImageOpenRouterTool(),
		t.inspectImageTool(),
		t.resizeImageTool(),
		t.cropImageTool(),
		t.addTitleTextTool(),
		t.composeThumbnailTool(),
		t.makeContactSheetTool(),
		t.makeImagePDFTool(),
	}
}

func (t *ImageTools) imageWorkspaceInfoTool() mcp.Tool {
	return mcp.Tool{
		Name:        "image_workspace_info",
		Description: "Describe image tools workspace, path policy, provider key status, and available local/generation tools.",
		InputSchema: schema(map[string]any{}),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			return mcp.Text(pretty(map[string]any{
				"workspace":       t.root,
				"path_policy":     "all input/output file paths must be relative to workspace and cannot escape it",
				"max_image_bytes": t.maxImageBytes,
				"providers": map[string]any{
					"openai_key":     os.Getenv("OPENAI_API_KEY") != "",
					"openrouter_key": os.Getenv("OPENROUTER_API_KEY") != "",
					"openai_models":  []string{"gpt-image-1", "gpt-image-2", "dall-e-3", "dall-e-2"},
				},
				"local_tools": []string{
					"inspect_image", "resize_image", "crop_image", "add_title_text",
					"compose_thumbnail", "make_contact_sheet", "make_image_pdf",
				},
				"provider_tools": []string{
					"generate_image_openai", "edit_image_openai", "generate_image_openrouter", "list_image_models",
				},
			})), nil
		},
	}
}
