package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Bradthebrad/nullbot-imagetools-mcp/internal/imagetools"
	"tinychain/mcp"
)

const version = "0.1.0"

func main() {
	transport := flag.String("transport", "stdio", "Transport: stdio, streamable-http, http, or sse.")
	addr := flag.String("addr", "127.0.0.1:8775", "HTTP/SSE listen address.")
	path := flag.String("path", "/mcp", "Streamable HTTP endpoint path.")
	ssePath := flag.String("sse-path", "/sse", "Legacy SSE endpoint path.")
	messagePath := flag.String("message-path", "/message", "Legacy SSE message endpoint path.")
	workspace := flag.String("workspace", ".", "Workspace root. Image tools cannot write outside this directory.")
	maxImageBytes := flag.Int64("max-image-bytes", 30*1024*1024, "Maximum input image bytes for local/provider tools.")
	showVersion := flag.Bool("version", false, "Print version and exit.")
	flag.Parse()

	if *showVersion {
		fmt.Println("nullbot-imagetools-mcp", version)
		return
	}

	imageTools, err := imagetools.New(imagetools.Config{
		Workspace:     *workspace,
		MaxImageBytes: *maxImageBytes,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "nullbot-imagetools-mcp:", err)
		os.Exit(2)
	}

	server := mcp.NewServer("nullbot-imagetools-mcp")
	server.Version = version
	for _, tool := range imageTools.Tools() {
		server.AddTool(tool)
	}

	if *transport != "stdio" {
		fmt.Fprintf(os.Stderr, "nullbot-imagetools-mcp serving %s on %s\n", *transport, *addr)
	}
	err = server.Run(
		context.Background(),
		mcp.WithTransport(*transport),
		mcp.WithAddr(*addr),
		mcp.WithPath(*path),
		mcp.WithSSEPaths(*ssePath, *messagePath),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nullbot-imagetools-mcp:", err)
		os.Exit(1)
	}
}
