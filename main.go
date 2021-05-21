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

const (
	clientIdentifier = "moac" // Client identifier to advertise over the network
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	// The app that holds all commands and flags.
	app = utils.NewApp(gitCommit, "The SSN Smart Contract Server(SCS) command line application")

	mFlags = []cli.Flag{
		utils.ConfigDirFlag,
	}
)

func waitToExit() {
	c := make(chan os.Signal)
	signal.Notify(c)
	for {
		s := <-c
		if s == syscall.SIGINT || s == syscall.SIGKILL || s == syscall.SIGTERM {
			fmt.Println("get signal:", s)
			os.Exit(1)
		}
	}
}

func gridTradingServer(ctx *cli.Context) {
	go waitToExit()
	go initLog(ctx)
	configFilePath := ctx.GlobalString(utils.ConfigDirFlag.Name)
	config, err := okex.GetConfiguration(configFilePath)
	if err != nil {
		fmt.Errorf("can not get runnning directory\n")
		return
	}
	// var config okex.Config
	// config.Endpoint = "https://www.okex.com/"
	// config.WSEndpoint = "wss://ws.okex.com:8443/ws/v5/private"
	// config.ApiKey = "bd954f95-f255-4963-b931-49eb998d8bea"
	// config.SecretKey = "A2249B004D0A85893FC4D51147CEA6AE"
	// config.Passphrase = "ZHou0037Bo228"
	// config.TimeoutSecond = 45
	// config.IsPrint = false
	// config.I18n = okex.ENGLISH
	// client := okex.NewClient(*config)

	// acc, err := client.GetAccountBalance("")
	// fmt.Println("=================================================================================")
	// fmt.Println("err = ", err, "GetAccountBalance = ", acc)
	// fmt.Println("=================================================================================")

	// tics, err := client.GetMarketTickers("SPOT", "")
	// fmt.Println("=================================================================================")
	// fmt.Println("err = ", err, "GetMarketTickers = ", tics)
	// fmt.Println("=================================================================================")

	// tic, err := client.GetMarketTicker("ETH-USDT")
	// fmt.Println("=================================================================================")
	// fmt.Println("err = ", err, "GetMarketTicker = ", tic)
	// fmt.Println("=================================================================================")

	// req := okex.NewParams()
	// req["instId"] = "ETH-USDT"
	// req["tdMode"] = "cash"
	// req["side"] = "buy"
	// req["ordType"] = "post_only"
	// req["px"] = "100"
	// req["sz"] = "1"
	// res, err := client.PostTradeOrder(&req)
	// fmt.Println("=================================================================================")
	// fmt.Println("err = ", err, "PostTradeOrder = ", res)
	// fmt.Println("=================================================================================")

	// req := okex.NewParams()
	// req["instId"] = "ETH-USDT"
	// req["tdMode"] = "cash"
	// req["side"] = "buy"
	// req["ordType"] = "post_only"
	// req["px"] = "1000"
	// req["sz"] = "1"
	// reqs := make([]map[string]string, 0)
	// reqs = append(reqs, req)
	// reqs = append(reqs, req)
	// res, err := client.PostTradeBatchOrders(&reqs)
	// fmt.Println("=================================================================================")
	// fmt.Println("err = ", err, "PostTradeBatchOrders = ", res)
	// fmt.Println("=================================================================================")

	// res, err := client.PostTradeCancelOrder("ETH-USDT", "313382483228250114", "")
	// fmt.Println("=================================================================================")
	// fmt.Println("err = ", err, "PostTradeCancelOrder = ", res)
	// fmt.Println("=================================================================================")

	// req := okex.NewParams()
	// req["instId"] = "ETH-USDT"
	// req["ordId"] = "313387643539181569"
	// reqs := make([]map[string]string, 0)
	// reqs = append(reqs, req)
	// req = okex.NewParams()
	// req["instId"] = "ETH-USDT"
	// req["ordId"] = "313393969610772484"
	// reqs = append(reqs, req)
	// res, err := client.PostTradeCancelBatchOrders(&reqs)
	// fmt.Println("=================================================================================")
	// fmt.Println("err = ", err, "PostTradeCancelBatchOrders = ", res)
	// fmt.Println("=================================================================================")

	// trdp, err := client.GetTradeOrdersPending(okex.NewReqParams())
	// fmt.Println("=================================================================================")
	// fmt.Println("err = ", err, "GetTradeOrdersPending = ", trdp)
	// fmt.Println("=================================================================================")

	agent := new(okex.OKWSAgent)
	agent.Start(config)

	err = agent.Login(config.ApiKey, config.Passphrase)
	fmt.Println("=================================================================================")
	fmt.Println("err = ", err, "Login = ")
	fmt.Println("=================================================================================")

	agent.Subscribe(okex.CHNL_OEDERS, "SPOT", okex.DefaultDataCallBack)

	time.Sleep(1 * time.Hour)
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
