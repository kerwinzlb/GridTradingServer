package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
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
		utils.InstIdFlag,
	}
)

func waitToExit(server *Server) {
	c := make(chan os.Signal)
	signal.Notify(c)
	for {
		s := <-c
		if s == syscall.SIGINT || s == syscall.SIGKILL || s == syscall.SIGTERM {
			log.Info("waitToExit", "get signal:", s)
			server.Exit()
		}
	}
}

func gridServer(ctx *cli.Context) {
	go initLog(ctx)
	configFilePath := ctx.GlobalString(utils.ConfigDirFlag.Name)
	config, err := okex.GetConfiguration(configFilePath)
	if err != nil {
		log.Errorf("GetConfiguration error:%v\n", err)
		return
	}
	log.Info("GetConfiguration success")
	var instId string
	if ctx.GlobalIsSet(utils.InstIdFlag.Name) {
		if instId = ctx.GlobalString(utils.InstIdFlag.Name); instId == "" {
			return
		}
	} else {
		log.Error("There is no InstIdFlag")
		return
	}
	gridServer, err := New(instId, config)
	if err != nil {
		log.Errorf("New error:%v\n", err)
		return
	}
	log.Info("New success")
	go waitToExit(gridServer)
	go gridServer.WsRecvLoop()
	time.Sleep(5 * time.Second)
	err = gridServer.Start()
	if err != nil {
		log.Errorf("Start error:%v\n", err)
		return
	}
	log.Info("Start success")
	go gridServer.MonitorLoop()
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
	app.Action = gridServer
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
	absLogPath = absLogPath + "/gridServer"
	if err := os.MkdirAll(filepath.Dir(absLogPath), os.ModePerm); err != nil {
		fmt.Errorf("Error %v in create %s", err, absLogPath)
		return err
	}
	lvl := log.Lvl(ctx.GlobalInt(utils.VerbosityFlag.Name))
	fmt.Println("lvl:", lvl)
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
