// Copyright 2015 The SSN Authors
// This file is part of SSN.
//
// SSN is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// SSN is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with SSN. If not, see <http://www.gnu.org/licenses/>.

// Package utils contains internal helper functions for SSN commands.
package utils

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	cli "gopkg.in/urfave/cli.v1"
)

var (
	CommandHelpTemplate = `{{.cmd.Name}}{{if .cmd.Subcommands}} command{{end}}{{if .cmd.Flags}} [command options]{{end}} [arguments...]
{{if .cmd.Description}}{{.cmd.Description}}
{{end}}{{if .cmd.Subcommands}}
SUBCOMMANDS:
	{{range .cmd.Subcommands}}{{.cmd.Name}}{{with .cmd.ShortName}}, {{.cmd}}{{end}}{{ "\t" }}{{.cmd.Usage}}
	{{end}}{{end}}{{if .categorizedFlags}}
{{range $idx, $categorized := .categorizedFlags}}{{$categorized.Name}} OPTIONS:
{{range $categorized.Flags}}{{"\t"}}{{.}}
{{end}}
{{end}}{{end}}`
)

func init() {
	cli.AppHelpTemplate = `{{.Name}} {{if .Flags}}[global options] {{end}}command{{if .Flags}} [command options]{{end}} [arguments...]

VERSION:
   {{.Version}}

COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}{{if .Flags}}
GLOBAL OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}{{end}}
`

	cli.CommandHelpTemplate = CommandHelpTemplate
}

// NewApp creates an app with default values.
func NewApp(gitCommit, usage string) *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = "SSN tech"
	app.Email = "info@moac.io"
	app.Version = ""
	if gitCommit != "" {
		app.Version += "-" + gitCommit[:8]
	}
	app.Usage = usage
	return app
}

// These are all the command line flags we support.
// If you add to this list, please remember to include the
// flag in the appropriate command definition.
//
// The flags are defined here so their names and help texts
// are the same for all commands.
// added the following command line flags:
// vnodeconfig - For input vnode config file
//

var (
	// General settings
	VerbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Usage: "sets the verbosity level, override loglevel in userconfig.json",
		Value: 2,
	}
	InstIdFlag = cli.StringFlag{
		Name:  "instid",
		Usage: "okex product id",
		Value: "",
	}
	ConfigDirFlag = DirectoryFlag{
		Name:  "configdir",
		Usage: "Data directory for the microchain databases",
		Value: DirectoryString{DefaultConfigDir()},
	}
	//
	PasswordFlag = cli.StringFlag{
		Name:  "password",
		Usage: "password of the SCS keystore",
		Value: "ssndefaultpwd",
	}
	VssPasswordFlag = cli.StringFlag{
		Name:  "vsspassword",
		Usage: "password of the VSS key file",
		Value: "ssndefaultvsspwd",
	}
	RmoveDataFlag = cli.BoolFlag{
		Name:  "removedata",
		Usage: "Remove leveldb history data for control monitor",
	}
	P2PListenPortFlag = cli.UintFlag{
		Name:  "p2pport",
		Usage: "P2P server listening port",
		Value: 30383,
	}
	// RPC settings
	RPCEnabledFlag = cli.BoolFlag{
		Name:  "rpc",
		Usage: "Enable the HTTP JSON-RPC server",
	}
	RPC1EnabledFlag = cli.BoolFlag{
		Name:  "rpcdebug",
		Usage: "Enable the HTTP-RPC Debug server",
	}
	RPCListenAddrFlag = cli.StringFlag{
		Name:  "rpcaddr",
		Usage: "HTTP-RPC server listening interface",
		Value: "127.0.0.1",
	}
	RPCPortFlag = cli.IntFlag{
		Name:  "rpcport",
		Usage: "HTTP-RPC server listening port",
		Value: 8548,
	}
	RPCCORSDomainFlag = cli.StringFlag{
		Name:  "rpccorsdomain",
		Usage: "Comma separated list of domains from which to accept cross origin requests (browser enforced)",
		Value: "",
	}
	RPCApiFlag = cli.StringFlag{
		Name:  "rpcapi",
		Usage: "API's offered over the HTTP-RPC interface",
		Value: "",
	}
	WSEnabledFlag = cli.BoolFlag{
		Name:  "ws",
		Usage: "Enable the WS-RPC server",
	}
	WSListenAddrFlag = cli.StringFlag{
		Name:  "wsaddr",
		Usage: "WS-RPC server listening interface",
		Value: "127.0.0.1",
	}
	WSPortFlag = cli.IntFlag{
		Name:  "wsport",
		Usage: "WS-RPC server listening port",
		Value: 8549,
	}
	WSApiFlag = cli.StringFlag{
		Name:  "wsapi",
		Usage: "API's offered over the WS-RPC interface",
		Value: "",
	}
	WSAllowedOriginsFlag = cli.StringFlag{
		Name:  "wsorigins",
		Usage: "Origins from which to accept websockets requests",
		Value: "",
	}
	// Network Settings
	// MaxPeersFlag = cli.IntFlag{
	// 	Name:  "maxpeers",
	// 	Usage: "Maximum number of network peers (network disabled if set to 0)",
	// 	Value: 25,
	// }
	TestnetFlag = cli.BoolFlag{
		Name:  "testnet",
		Usage: "SSN test network: SCS connect with SSN test network 101",
	}

	DevModeFlag = cli.BoolFlag{
		Name:  "dev",
		Usage: "Developer mode: SCS connect with private network 100",
	}
	// MaxPendingPeersFlag = cli.IntFlag{
	// 	Name:  "maxpendpeers",
	// 	Usage: "Maximum number of pending connection attempts (defaults used if set to 0)",
	// 	Value: 0,
	// }

	ExecFlag = cli.StringFlag{
		Name:  "exec",
		Usage: "Execute JavaScript statement",
	}
	PreloadJSFlag = cli.StringFlag{
		Name:  "preload",
		Usage: "Comma separated list of JavaScript files to preload into the console",
	}
	// ATM the url is left to the user and deployment to
	JSpathFlag = cli.StringFlag{
		Name:  "jspath",
		Usage: "JavaScript root path for `loadScript`",
		Value: ".",
	}
	// Configration setting for SCS service
	UserConfig = cli.StringFlag{
		Name:  "userconfig",
		Usage: "userconfig [file path]",
		Value: "",
	}

	// Gas price oracle settings
	GpoBlocksFlag = cli.IntFlag{
		Name:  "gpoblocks",
		Usage: "Number of recent blocks to check for gas prices",
		Value: 10,
	}
	GpoPercentileFlag = cli.IntFlag{
		Name:  "gpopercentile",
		Usage: "Suggested gas price is the given percentile of a set of recent transaction gas prices",
		Value: 50,
	}
)

// MakeConsolePreloads retrieves the absolute paths for the console JavaScript
// scripts to preload before starting.
// func MakeConsolePreloads(ctx *cli.Context) []string {
// 	// Skip preloading if there's nothing to preload
// 	if ctx.GlobalString(PreloadJSFlag.Name) == "" {
// 		return nil
// 	}
// 	// Otherwise resolve absolute paths and return them
// 	preloads := []string{}

// 	assets := ctx.GlobalString(JSpathFlag.Name)
// 	for _, file := range strings.Split(ctx.GlobalString(PreloadJSFlag.Name), ",") {
// 		preloads = append(preloads, common.AbsolutePath(assets, strings.TrimSpace(file)))
// 	}
// 	return preloads
// }

// SetScsConfig applies SCS-related command line flags to the config.
// The commandline flag values override userconfig.json
//
// func SetScsConfig(ctx *cli.Context, cfg *config.Configuration) {

// 	// Avoid conflicting network flags
// 	checkExclusive(ctx, DevModeFlag, TestnetFlag)

// 	fmt.Printf("setHTTP for Data dir: %v\n", cfg.DataDir)
// 	setHTTP(ctx, cfg)
// 	// TODO
// 	setWS(ctx, cfg)
// 	setGPO(ctx, cfg)
// 	// setNodeUserIdent(ctx, cfg)

// 	//Use input DataDir
// 	switch {
// 	case ctx.GlobalIsSet(DataDirFlag.Name):
// 		cfg.DataDir = ctx.GlobalString(DataDirFlag.Name)
// 	case ctx.GlobalIsSet(PasswordFlag.Name):
// 		cfg.VssPassword = ctx.GlobalString(VssPasswordFlag.Name)
// 	case ctx.GlobalBool(DevModeFlag.Name):
// 		cfg.DataDir = filepath.Join(DefaultConfigDir(), "devnet")
// 	case ctx.GlobalBool(TestnetFlag.Name):
// 		cfg.DataDir = filepath.Join(DefaultConfigDir(), "testnet")
// 	}

// 	fmt.Printf("Check Data dir: %v\n", cfg.DataDir)

// 	// Setup the log level with commandline if verbosity flag is set.
// 	if ctx.GlobalIsSet(VerbosityFlag.Name) {
// 		cfg.LogLevel = ctx.GlobalInt(VerbosityFlag.Name)
// 	}

// }

// splitAndTrim splits input separated by a comma
// and trims excessive white space from the substrings.
func splitAndTrim(input string) []string {
	result := strings.Split(input, ",")
	for i, r := range result {
		result[i] = strings.TrimSpace(r)
	}
	return result
}

// setHTTP creates the HTTP RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
// func setHTTP(ctx *cli.Context, cfg *config.Configuration) {

// 	//When RPC is enabled, put in the default value
// 	//
// 	if ctx.GlobalBool(RPCEnabledFlag.Name) || ctx.GlobalBool(RPC1EnabledFlag.Name) {

// 		cfg.HTTPHost = "127.0.0.1"
// 		cfg.HTTPPort = 8548
// 		if ctx.GlobalIsSet(RPCListenAddrFlag.Name) {
// 			cfg.HTTPHost = ctx.GlobalString(RPCListenAddrFlag.Name)
// 		}

// 	}

// 	if ctx.GlobalIsSet(RPCPortFlag.Name) {
// 		cfg.HTTPPort = ctx.GlobalInt(RPCPortFlag.Name)
// 	}
// 	if ctx.GlobalIsSet(RPCCORSDomainFlag.Name) {
// 		cfg.HTTPCors = splitAndTrim(ctx.GlobalString(RPCCORSDomainFlag.Name))
// 	}
// 	if ctx.GlobalIsSet(RPCApiFlag.Name) {
// 		cfg.HTTPModules = splitAndTrim(ctx.GlobalString(RPCApiFlag.Name))
// 	}
// 	// fmt.Printf("===============setHTTP %v\n", cfg.HTTPHost)
// }

// setWS creates the WebSocket RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
// func setWS(ctx *cli.Context, cfg *config.Configuration) {
// 	if ctx.GlobalBool(WSEnabledFlag.Name) && cfg.WSHost == "" {
// 		cfg.WSHost = "127.0.0.1"
// 		if ctx.GlobalIsSet(WSListenAddrFlag.Name) {
// 			cfg.WSHost = ctx.GlobalString(WSListenAddrFlag.Name)
// 		}
// 	}

// 	if ctx.GlobalIsSet(WSPortFlag.Name) {
// 		cfg.WSPort = ctx.GlobalInt(WSPortFlag.Name)
// 	}
// 	if ctx.GlobalIsSet(WSAllowedOriginsFlag.Name) {
// 		cfg.WSOrigins = splitAndTrim(ctx.GlobalString(WSAllowedOriginsFlag.Name))
// 	}
// 	if ctx.GlobalIsSet(WSApiFlag.Name) {
// 		cfg.WSModules = splitAndTrim(ctx.GlobalString(WSApiFlag.Name))
// 	}
// }

// func setGPO(ctx *cli.Context, cfg *config.Configuration) {
// 	if ctx.GlobalIsSet(GpoBlocksFlag.Name) {
// 		cfg.GPO.Blocks = ctx.GlobalInt(GpoBlocksFlag.Name)
// 	}
// 	if ctx.GlobalIsSet(GpoPercentileFlag.Name) {
// 		cfg.GPO.Percentile = ctx.GlobalInt(GpoPercentileFlag.Name)
// 	}
// }

// DefaultConfigDir is the default data directory to use for the databases and other
// persistence requirements.
// ./ssndata
func DefaultConfigDir() string {
	cur, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err == nil {
		return filepath.Join(cur, "config.json")
	}
	return ""
}

func checkExclusive(ctx *cli.Context, flags ...cli.Flag) {
	set := make([]string, 0, 1)
	for _, flag := range flags {
		if ctx.GlobalIsSet(flag.GetName()) {
			set = append(set, "--"+flag.GetName())
		}
	}
	if len(set) > 1 {
		Fatalf("flags %v can't be used at the same time", strings.Join(set, ", "))
	}
}

// Fatalf formats a message to standard error and exits the program.
// The message is also printed to standard output if standard error
// is redirected to a different file.
func Fatalf(format string, args ...interface{}) {
	w := io.MultiWriter(os.Stdout, os.Stderr)
	if runtime.GOOS == "windows" {
		// The SameFile check below doesn't work on Windows.
		// stdout is unlikely to get redirected though, so just print there.
		w = os.Stdout
	} else {
		outf, _ := os.Stdout.Stat()
		errf, _ := os.Stderr.Stat()
		if outf != nil && errf != nil && os.SameFile(outf, errf) {
			w = os.Stderr
		}
	}
	fmt.Fprintf(w, "Fatal: "+format+"\n", args...)
	os.Exit(1)
}

// Custom type which is registered in the flags library which cli uses for
// argument parsing. This allows us to expand Value to an absolute path when
// the argument is parsed
type DirectoryString struct {
	Value string
}

func (self *DirectoryString) String() string {
	return self.Value
}

func (self *DirectoryString) Set(value string) error {
	self.Value = expandPath(value)
	return nil
}

// Custom cli.Flag type which expand the received string to an absolute path.
// e.g. ~/.moac -> /home/username/.moac
type DirectoryFlag struct {
	Name  string
	Value DirectoryString
	Usage string
}

func (self DirectoryFlag) String() string {
	fmtString := "%s %v\t%v"
	if len(self.Value.Value) > 0 {
		fmtString = "%s \"%v\"\t%v"
	}
	return fmt.Sprintf(fmtString, prefixedNames(self.Name), self.Value.Value, self.Usage)
}

func eachName(longName string, fn func(string)) {
	parts := strings.Split(longName, ",")
	for _, name := range parts {
		name = strings.Trim(name, " ")
		fn(name)
	}
}

// called by cli library, grabs variable from environment (if in env)
// and adds variable to flag set for parsing.
func (self DirectoryFlag) Apply(set *flag.FlagSet) {
	eachName(self.Name, func(name string) {
		set.Var(&self.Value, self.Name, self.Usage)
	})
}

func prefixedNames(fullName string) (prefixed string) {
	parts := strings.Split(fullName, ",")
	for i, name := range parts {
		name = strings.Trim(name, " ")
		prefixed += prefixFor(name) + name
		if i < len(parts)-1 {
			prefixed += ", "
		}
	}
	return
}

func (self DirectoryFlag) GetName() string {
	return self.Name
}

func (self *DirectoryFlag) Set(value string) {
	self.Value.Value = value
}

// Expands a file path
// 1. replace tilde with users home dir
// 2. expands embedded environment variables
// 3. cleans the path, e.g. /a/b/../c -> /a/c
// Note, it has limitations, e.g. ~someuser/tmp will not be expanded
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		if home := homeDir(); home != "" {
			p = home + p[1:]
		}
	}
	return path.Clean(os.ExpandEnv(p))
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

func prefixFor(name string) (prefix string) {
	if len(name) == 1 {
		prefix = "-"
	} else {
		prefix = "--"
	}

	return
}

// MigrateFlags sets the global flag from a local flag when it's set.
// This is a temporary function used for migrating old command/flags to the
// new format.
//
// e.g. moac account new --keystore /tmp/mykeystore --lightkdf
//
// is equivalent after calling this method with:
//
// moac --keystore /tmp/mykeystore --lightkdf account new
//
// This allows the use of the existing configuration functionality.
// When all flags are migrated this function can be removed and the existing
// configuration functionality must be changed that is uses local flags
func MigrateFlags(action func(ctx *cli.Context) error) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		for _, name := range ctx.FlagNames() {
			if ctx.IsSet(name) {
				ctx.GlobalSet(name, ctx.String(name))
			}
		}
		return action(ctx)
	}
}
