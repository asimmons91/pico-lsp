module example.com/pico-lsp-publicapi-lsp

go 1.27.4

require (
	github.com/asimmons91/pico-lsp v0.0.0
	go.lsp.dev/jsonrpc2 v1.0.1
	go.lsp.dev/protocol v1.0.1
	go.lsp.dev/uri v1.0.1
)

require github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect

replace github.com/asimmons91/pico-lsp => ../../..
