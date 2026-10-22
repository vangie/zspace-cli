// zspace 是极空间 NAS 的命令行。
package main

import (
	"os"

	"github.com/vangie/zspace-cli/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
