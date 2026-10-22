// zspace-mcp 是极空间 NAS 的 MCP stdio 服务。
package main

import (
	"os"

	"github.com/vangie/zspace-cli/internal/mcp"
)

func main() {
	os.Exit(mcp.Run(os.Args[1:]))
}
