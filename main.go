package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	"github.com/kerwinzlb/GridTradingServer/server"
	"github.com/kerwinzlb/GridTradingServer/utils"
	"gopkg.in/urfave/cli.v1"
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	// The app that holds all commands and flags.
	app = utils.NewApp(gitCommit, "The SSN Smart Contract Server(SCS) command line application")

	mFlags = []cli.Flag{
		utils.ConfigDirFlag,
		utils.VerbosityFlag,
	}
)

func waitToExit(server *server.Server) {
	c := make(chan os.Signal)
	signal.Notify(c)
	for {
		s := <-c
		if s == syscall.SIGINT || s == syscall.SIGKILL || s == syscall.SIGTERM {
			log.Debug("get signal:", s)
			server.Stop()
			// os.Exit(1)
		}
	}
}

func gridTradingServer(ctx *cli.Context) {
	go initLog(ctx)
	configFilePath := ctx.GlobalString(utils.ConfigDirFlag.Name)
	config, err := okex.GetConfiguration(configFilePath)
	if err != nil {
		fmt.Errorf("GetConfiguration error%v\n", err)
		return
	}
	gridServer, err := server.New(config)
	if err != nil {
		fmt.Errorf("server.New error%v\n", err)
		return
	}
	go waitToExit(gridServer)
	gridServer.Start()
	gridServer.Wait()
}

/*
 * Main
 * program to start Smart Contract Server
 * using urfave/cli framework.
 */
func init() {
	app.Name = os.Args[0]

	// Initialize the CLI app and start Moac
	app.Action = gridTradingServer
	app.Copyright = "Copyright 2017-2018 SSN Tech Inc."
	app.Commands = []cli.Command{}

	// Set all commandline flags
	// These flags need to be defined in utils/flags.go
	app.Flags = append(app.Flags, mFlags...)
	sort.Sort(cli.FlagsByName(app.Flags))
}

/*
 * Add build directory for the log file
 * if '_logs' directory not exists.
 */
func initLog(ctx *cli.Context) error {
	absLogPath, _ := filepath.Abs("./_logs")
	absLogPath = absLogPath + "/gridTradingServer"
	if err := os.MkdirAll(filepath.Dir(absLogPath), os.ModePerm); err != nil {
		log.Errorf("Error %v in create %s", err, absLogPath)
		return err
	}

	lvl := log.Lvl(ctx.GlobalInt(utils.VerbosityFlag.Name))
	log.LogRotate(absLogPath, lvl)

	return nil
}

func main() {
	//Start the SCS server
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
