package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"

	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	pb "github.com/kerwinzlb/GridTradingServer/proto"
	"github.com/kerwinzlb/GridTradingServer/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

func waitToExit() {
	c := make(chan os.Signal)
	signal.Notify(c)
	for {
		s := <-c
		if s == syscall.SIGINT || s == syscall.SIGKILL || s == syscall.SIGTERM {
			log.Info("waitToExit", "get signal:", s)
			server.Stop()
			os.Exit(1)
		}
	}
}

func wsServer(ctx *cli.Context) {
	go initLog(ctx)
	go waitToExit()
	configFilePath := ctx.GlobalString(utils.ConfigDirFlag.Name)
	config, err := okex.GetConfiguration(configFilePath)
	if err != nil {
		log.Errorf("GetConfiguration error:%v\n", err)
		return
	}
	log.Info("GetConfiguration success")

	NewServer(config)
	server.Start()

	srvPort := ctx.GlobalInt(utils.PortFlag.Name)
	listen, err := net.Listen("tcp", ":"+strconv.Itoa(srvPort))
	if err != nil {
		log.Error("tcp port error", "port", srvPort)
	}

	s := grpc.NewServer()

	//  注册服务
	// 	TODO:--具体服务根据pb.go修改
	//
	pb.RegisterWsServer(s, server)
	reflection.Register(s)
	if err := s.Serve(listen); err != nil {
		log.Error("ServiceStartFailed", "error", err)
	}

}

/*
 * Main
 * program to start Smart Contract Server
 * using urfave/cli framework.
 */
func init() {
	app.Name = os.Args[0]

	// Initialize the CLI app and start Moac
	app.Action = wsServer
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
