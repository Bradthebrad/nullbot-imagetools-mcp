# NullBot Image Tools MCP

![NullBot image tools](https://img.shields.io/badge/NullBot-image%20tools-gold?style=for-the-badge)

`nullbot-imagetools-mcp` is a small Go MCP server for image generation, image editing, thumbnail composition, contact sheets, and image-based PDFs. It is designed for NullBot first, but it speaks plain MCP over `stdio`, HTTP, streamable HTTP, and legacy SSE, so it can be used by other MCP-capable clients too.

The philosophy is the same as the rest of the NullBot tool family: keep the binary portable, keep the API surface descriptive, and keep all filesystem writes inside a caller-controlled workspace.

## What It Does

Provider tools:

| Tool | Purpose |
| --- | --- |
| `list_image_models` | Lists known OpenAI image models and OpenRouter image-output models when available. |
| `generate_image_openai` | Calls the OpenAI Images API and saves the result into the workspace. |
| `edit_image_openai` | Sends an input image plus edit prompt to the OpenAI Images API. |
| `generate_image_openrouter` | Calls OpenRouter chat completions with an image-output model and saves the returned image. |

Local tools:

| Tool | Purpose |
| --- | --- |
| `image_workspace_info` | Shows workspace path policy, provider key status, and available tools. |
| `inspect_image` | Returns dimensions, format, MIME type, and byte size. |
| `resize_image` | Resizes using `contain`, `cover`, or `stretch`. |
| `crop_image` | Crops a rectangular region. |
| `add_title_text` | Adds title/subtitle bands for thumbnails, posters, and social art. |
| `compose_thumbnail` | Creates a 16:9-style thumbnail canvas from a background and title text. |
| `make_contact_sheet` | Builds a visual contact sheet from many images. |
| `make_image_pdf` | Creates PDFs from images for coloring books, workbooks, proofs, and packets. |

## Build

```powershell
go build -trimpath -ldflags "-s -w" -o dist/nullbot-imagetools-mcp.exe ./cmd/nullbot-imagetools-mcp
```

The server depends on `tinychain/mcp`. In this repo, local development uses:

```go
replace tinychain/mcp => ../tinychain/mcp
```

## Run

Default MCP transport is `stdio`, which is what NullBot and most local MCP clients should use.

```powershell
.\dist\nullbot-imagetools-mcp.exe --workspace "C:\Users\you\Pictures"
```

HTTP transport is available when an MCP client wants to connect over localhost or a local network.

```powershell
.\dist\nullbot-imagetools-mcp.exe --transport http --addr 127.0.0.1:8775 --path /mcp --workspace "C:\work\images"
```

Legacy SSE is also supported:

```powershell
.\dist\nullbot-imagetools-mcp.exe --transport sse --addr 127.0.0.1:8775 --sse-path /sse --message-path /message
```

## Provider Keys

The server reads keys from environment variables. NullBot can pass these from its local key store when it launches MCP servers.

| Variable | Used By |
| --- | --- |
| `OPENAI_API_KEY` | `generate_image_openai`, `edit_image_openai` |
| `OPENROUTER_API_KEY` | `generate_image_openrouter`, OpenRouter model discovery |

You can also pass keys manually when launching the server outside NullBot:

```powershell
.\nullbot-imagetools-mcp.exe --workspace "C:\work\images" --openai-api-key "sk-..." --openrouter-api-key "sk-or-..."
```

When launched by NullBot, the server expects NullBot to inject saved keys from `~/.nullbot/api/keys.json` into the MCP process environment. You should not need to set shell environment variables for the normal NullBot marketplace flow.

OpenAI image model access varies by account. The tool defaults to `gpt-image-1`, but supports `gpt-image-2`, `dall-e-3`, and other compatible image models when the account supports them.

OpenRouter image generation is model-dependent. Use `list_image_models` to discover models that advertise image output, then pass the selected model id to `generate_image_openrouter`.

## Workspace Safety

All file paths are workspace-relative. Absolute paths are rejected, and paths cannot escape the workspace via `..`.

This is intentional. A client such as NullBot, Claude Desktop, or another MCP host should decide the workspace root before starting the server:

```powershell
.\nullbot-imagetools-mcp.exe --workspace "C:\Users\you\.nullbot\artifacts\images"
```

Example output path:

```json
{
  "output_path": "thumbnails/video-001.png"
}
```

The server will create parent directories for output files.

## Example Tool Calls

Generate an OpenAI image:

```json
{
  "prompt": "retro terminal robot mascot, bold gold pixels, black background",
  "output_path": "generated/nullbot-mascot.png",
  "model": "gpt-image-1",
  "size": "1024x1024"
}
```

Create a YouTube-style thumbnail:

```json
{
  "background_path": "generated/nullbot-mascot.png",
  "output_path": "thumbs/demo-thumbnail.png",
  "title": "NO MAGIC HERE",
  "subtitle": "Just a client with tools",
  "width": 1280,
  "height": 720,
  "background_fit": "cover",
  "title_band_color": "#000000",
  "title_text_color": "#ffffff"
}
```

Build a contact sheet:

```json
{
  "image_paths": [
    "generated/a.png",
    "generated/b.png",
    "generated/c.png"
  ],
  "output_path": "sheets/contact.png",
  "columns": 3,
  "captions": true
}
```

Create a PDF:

```json
{
  "image_paths": [
    "pages/page-01.png",
    "pages/page-02.png"
  ],
  "output_path": "pdfs/coloring-book.pdf",
  "title": "Tiny Robot Coloring Book",
  "page_size": "letter",
  "captions": false
}
```

## Intended NullBot Marketplace Metadata

Suggested package metadata for a NullBot market manifest:

```json
{
  "id": "nullbot-imagetools-mcp",
  "kind": "mcp_server",
  "transport": "stdio",
  "repo": "Bradthebrad/nullbot-imagetools-mcp",
  "description": "Generate, edit, compose, inspect, and package images with OpenAI/OpenRouter plus local Go image tools.",
  "permissions": ["image_generation", "workspace_files"],
  "args": ["--workspace", "{{workspace}}"]
}
```

## Roadmap

- Add provider adapters for FAL, Replicate, Stability, Leonardo, Ideogram, and local ComfyUI.
- Add richer text rendering with configurable fonts.
- Add masks and multi-image edit helpers.
- Add page templates for workbooks, coloring books, thumbnails, cards, and social batches.
- Add optional OCR/vision-assisted image critique tools through the parsers MCP or shared provider helpers.
