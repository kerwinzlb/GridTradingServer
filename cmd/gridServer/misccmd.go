package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/kerwinzlb/GridTradingServer/params"
	"github.com/kerwinzlb/GridTradingServer/utils"
	"gopkg.in/urfave/cli.v1"
)

var (
	versionCommand = cli.Command{
		Action:    utils.MigrateFlags(version),
		Name:      "version",
		Usage:     "Print version numbers",
		ArgsUsage: " ",
		Category:  "MISCELLANEOUS COMMANDS",
		Description: `
The output of this command is supposed to be machine-readable.
`,
	}
)

func version(ctx *cli.Context) error {
	fmt.Println(strings.Title(os.Args[0]))
	fmt.Println("Version:Termination Date", time.Unix(params.TerminationTimeStamp, 0).String())
	if gitCommit != "" {
		fmt.Println("Git Commit:", gitCommit)
	}
	fmt.Println("Architecture:", runtime.GOARCH)
	fmt.Println("Go Version:", runtime.Version())
	fmt.Println("Operating System:", runtime.GOOS)
	fmt.Printf("GOPATH=%s\n", os.Getenv("GOPATH"))
	fmt.Printf("GOROOT=%s\n", runtime.GOROOT())
	return nil
}
