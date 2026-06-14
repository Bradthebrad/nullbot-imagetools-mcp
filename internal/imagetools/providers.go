package imagetools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tinychain/mcp"
)

const (
	openAIImagesURL       = "https://api.openai.com/v1/images/generations"
	openAIImageEditsURL   = "https://api.openai.com/v1/images/edits"
	openRouterChatURL     = "https://openrouter.ai/api/v1/chat/completions"
	openRouterModelsURL   = "https://openrouter.ai/api/v1/models"
	providerClientTimeout = 180 * time.Second
)

func (t *ImageTools) listImageModelsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_image_models",
		Description: "List known OpenAI image models and, when OPENROUTER_API_KEY is set, OpenRouter models that advertise image output.",
		InputSchema: schema(map[string]any{"provider": stringProp("Optional provider filter: openai or openrouter.")}),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			provider := strings.ToLower(textArg(args, "provider"))
			result := map[string]any{}
			if provider == "" || provider == "openai" {
				result["openai"] = []map[string]any{
					{"id": "gpt-image-2", "notes": "OpenAI image generation/editing model when available to the account."},
					{"id": "gpt-image-1", "notes": "OpenAI image generation/editing model."},
					{"id": "dall-e-3", "notes": "Legacy high quality generation model."},
					{"id": "dall-e-2", "notes": "Legacy generation/editing model."},
				}
			}
			if provider == "" || provider == "openrouter" {
				models, err := fetchOpenRouterImageModels(ctx)
				if err != nil {
					result["openrouter_error"] = err.Error()
				} else {
					result["openrouter"] = models
				}
			}
			return mcp.Text(pretty(result)), nil
		},
	}
}

func (t *ImageTools) generateImageOpenAITool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_image_openai",
		Description: "Generate an image with OpenAI Images API and save it into the workspace.",
		InputSchema: schema(map[string]any{
			"prompt":      stringProp("Image prompt."),
			"output_path": stringProp("Workspace-relative path to save the generated image."),
			"model":       stringProp("OpenAI image model. Default: gpt-image-1."),
			"size":        stringProp("Image size such as 1024x1024, 1536x1024, or auto. Default: 1024x1024."),
			"quality":     stringProp("Quality hint such as low, medium, high, hd, or auto."),
		}, "prompt", "output_path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			key := os.Getenv("OPENAI_API_KEY")
			if key == "" {
				return mcp.ToolResult{}, fmt.Errorf("OPENAI_API_KEY is not set")
			}
			output, err := t.resolveOutput(textArg(args, "output_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			model := textArg(args, "model")
			if model == "" {
				model = "gpt-image-1"
			}
			size := textArg(args, "size")
			if size == "" {
				size = "1024x1024"
			}
			body := map[string]any{
				"model":  model,
				"prompt": textArg(args, "prompt"),
				"size":   size,
				"n":      1,
			}
			if quality := textArg(args, "quality"); quality != "" {
				body["quality"] = quality
			}
			data, err := postJSON(ctx, openAIImagesURL, key, body, nil)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			if err := saveProviderImage(ctx, output, data); err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"provider": "openai", "model": model, "output_path": t.rel(output)})), nil
		},
	}
}

func (t *ImageTools) editImageOpenAITool() mcp.Tool {
	return mcp.Tool{
		Name:        "edit_image_openai",
		Description: "Edit an existing workspace image with OpenAI Images API and save the result into the workspace.",
		InputSchema: schema(map[string]any{
			"input_path":  stringProp("Workspace-relative source image path."),
			"output_path": stringProp("Workspace-relative edited image path."),
			"prompt":      stringProp("Edit instruction."),
			"mask_path":   stringProp("Optional workspace-relative mask image path."),
			"model":       stringProp("OpenAI image edit model. Default: gpt-image-1."),
			"size":        stringProp("Output size such as 1024x1024 or auto."),
		}, "input_path", "output_path", "prompt"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			key := os.Getenv("OPENAI_API_KEY")
			if key == "" {
				return mcp.ToolResult{}, fmt.Errorf("OPENAI_API_KEY is not set")
			}
			input, err := t.resolve(textArg(args, "input_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			output, err := t.resolveOutput(textArg(args, "output_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			model := textArg(args, "model")
			if model == "" {
				model = "gpt-image-1"
			}
			data, err := postOpenAIImageEdit(ctx, key, input, t.optionalPath(textArg(args, "mask_path")), model, textArg(args, "prompt"), textArg(args, "size"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			if err := saveProviderImage(ctx, output, data); err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"provider": "openai", "model": model, "output_path": t.rel(output)})), nil
		},
	}
}

func (t *ImageTools) generateImageOpenRouterTool() mcp.Tool {
	return mcp.Tool{
		Name:        "generate_image_openrouter",
		Description: "Generate an image through OpenRouter chat completions with an image-output model and save it into the workspace.",
		InputSchema: schema(map[string]any{
			"prompt":      stringProp("Image prompt."),
			"output_path": stringProp("Workspace-relative path to save the generated image."),
			"model":       stringProp("OpenRouter model id that supports image output."),
			"site_url":    stringProp("Optional HTTP-Referer header."),
			"app_name":    stringProp("Optional X-Title header."),
		}, "prompt", "output_path", "model"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			key := os.Getenv("OPENROUTER_API_KEY")
			if key == "" {
				return mcp.ToolResult{}, fmt.Errorf("OPENROUTER_API_KEY is not set")
			}
			output, err := t.resolveOutput(textArg(args, "output_path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			headers := map[string]string{}
			if siteURL := textArg(args, "site_url"); siteURL != "" {
				headers["HTTP-Referer"] = siteURL
			}
			if appName := textArg(args, "app_name"); appName != "" {
				headers["X-Title"] = appName
			}
			body := map[string]any{
				"model":      textArg(args, "model"),
				"modalities": []string{"image", "text"},
				"messages": []map[string]any{{
					"role":    "user",
					"content": textArg(args, "prompt"),
				}},
			}
			data, err := postJSON(ctx, openRouterChatURL, key, body, headers)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			if err := saveOpenRouterImage(ctx, output, data); err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"provider": "openrouter", "model": textArg(args, "model"), "output_path": t.rel(output)})), nil
		},
	}
}

func postJSON(ctx context.Context, endpoint, key string, body any, headers map[string]string) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := (&http.Client{Timeout: providerClientTimeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider status %d: %s", resp.StatusCode, string(out))
	}
	return out, nil
}

func postOpenAIImageEdit(ctx context.Context, key, inputPath, maskPath, model, prompt, size string) ([]byte, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("model", model)
	_ = writer.WriteField("prompt", prompt)
	if size != "" {
		_ = writer.WriteField("size", size)
	}
	if err := addMultipartFile(writer, "image", inputPath); err != nil {
		return nil, err
	}
	if maskPath != "" {
		if err := addMultipartFile(writer, "mask", maskPath); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIImageEditsURL, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := (&http.Client{Timeout: providerClientTimeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai image edit status %d: %s", resp.StatusCode, string(out))
	}
	return out, nil
}

func addMultipartFile(writer *multipart.Writer, field, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	part, err := writer.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

func (t *ImageTools) optionalPath(path string) string {
	if path == "" {
		return ""
	}
	full, err := t.resolve(path)
	if err != nil {
		return ""
	}
	return full
}

func saveProviderImage(ctx context.Context, output string, data []byte) error {
	var resp struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if len(resp.Data) == 0 {
		return fmt.Errorf("provider response contained no images")
	}
	if resp.Data[0].B64JSON != "" {
		decoded, err := base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
		if err != nil {
			return err
		}
		return os.WriteFile(output, decoded, 0644)
	}
	if resp.Data[0].URL != "" {
		return downloadURL(ctx, output, resp.Data[0].URL)
	}
	return fmt.Errorf("provider image had neither b64_json nor url")
}

func saveOpenRouterImage(ctx context.Context, output string, data []byte) error {
	candidates := collectImageURLs(data)
	if len(candidates) == 0 {
		return fmt.Errorf("openrouter response contained no image URLs or data URLs")
	}
	return saveImageURL(ctx, output, candidates[0])
}

func collectImageURLs(data []byte) []string {
	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil
	}
	var urls []string
	seen := map[string]bool{}
	add := func(text string) {
		if seen[text] {
			return
		}
		seen[text] = true
		urls = append(urls, text)
	}
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if strings.EqualFold(key, "url") {
					if text, ok := child.(string); ok && (strings.HasPrefix(text, "data:image/") || looksLikeImageURL(text)) {
						add(text)
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		case string:
			if strings.HasPrefix(typed, "data:image/") || looksLikeImageURL(typed) {
				add(typed)
			}
		}
	}
	walk(generic)
	return urls
}

func looksLikeImageURL(text string) bool {
	parsed, err := url.Parse(text)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	lower := strings.ToLower(parsed.Path)
	return strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".webp")
}

func saveImageURL(ctx context.Context, output, rawURL string) error {
	if strings.HasPrefix(rawURL, "data:image/") {
		parts := strings.SplitN(rawURL, ",", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid data URL")
		}
		data, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return err
		}
		return os.WriteFile(output, data, 0644)
	}
	return downloadURL(ctx, output, rawURL)
}

func downloadURL(ctx context.Context, output, rawURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: providerClientTimeout}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download status %d", resp.StatusCode)
	}
	return copyToFile(output, resp.Body)
}

func fetchOpenRouterImageModels(ctx context.Context) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openRouterModelsURL, nil)
	if err != nil {
		return nil, err
	}
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter models status %d: %s", resp.StatusCode, string(data))
	}
	var parsed struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, model := range parsed.Data {
		if hasImageOutput(model) {
			out = append(out, map[string]any{
				"id":          model["id"],
				"name":        model["name"],
				"description": model["description"],
			})
		}
	}
	return out, nil
}

func hasImageOutput(model map[string]any) bool {
	for _, key := range []string{"output_modalities", "supported_output_modalities", "modalities"} {
		if raw, ok := model[key].([]any); ok {
			for _, item := range raw {
				if text, ok := item.(string); ok && strings.Contains(strings.ToLower(text), "image") {
					return true
				}
			}
		}
	}
	if arch, ok := model["architecture"].(map[string]any); ok {
		return hasImageOutput(arch)
	}
	return false
}
