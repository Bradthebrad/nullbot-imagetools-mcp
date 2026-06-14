module github.com/Bradthebrad/nullbot-imagetools-mcp

go 1.26.4

replace tinychain/mcp => ../tinychain/mcp

require (
	github.com/jung-kurt/gofpdf v1.16.2
	golang.org/x/image v0.42.0
	tinychain/mcp v0.0.0
)

require (
	tinychain v0.0.0 // indirect
	tinychain/agent v0.0.0 // indirect
)

replace tinychain => ../tinychain/client

replace tinychain/agent => ../tinychain/agent
