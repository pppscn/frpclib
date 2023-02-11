// Copyright 2018 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sub

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/auth"
	"github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/pkg/util/log"
	"github.com/fatedier/frp/pkg/util/version"
)

const (
	CfgFileTypeIni = iota
	CfgFileTypeCmd
)

var (
	cfgFile     string
	cfgDir      string
	showVersion bool

	serverAddr      string
	user            string
	protocol        string
	token           string
	logLevel        string
	logFile         string
	logMaxDays      int
	disableLogColor bool

	proxyName          string
	localIP            string
	localPort          int
	remotePort         int
	useEncryption      bool
	useCompression     bool
	bandwidthLimit     string
	bandwidthLimitMode string
	customDomains      string
	subDomain          string
	httpUser           string
	httpPwd            string
	locations          string
	hostHeaderRewrite  string
	role               string
	sk                 string
	multiplexer        string
	serverName         string
	bindAddr           string
	bindPort           int

	tlsEnable bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./frpc.ini", "config file of frpc")
	rootCmd.PersistentFlags().StringVarP(&cfgDir, "config_dir", "", "", "config directory, run one frpc service for each file in config directory")
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "version of frpc")
}

func RegisterCommonFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&serverAddr, "server_addr", "s", "127.0.0.1:7000", "frp server's address")
	cmd.PersistentFlags().StringVarP(&user, "user", "u", "", "user")
	cmd.PersistentFlags().StringVarP(&protocol, "protocol", "p", "tcp", "tcp or kcp or websocket")
	cmd.PersistentFlags().StringVarP(&token, "token", "t", "", "auth token")
	cmd.PersistentFlags().StringVarP(&logLevel, "log_level", "", "info", "log level")
	cmd.PersistentFlags().StringVarP(&logFile, "log_file", "", "console", "console or file path")
	cmd.PersistentFlags().IntVarP(&logMaxDays, "log_max_days", "", 3, "log file reversed days")
	cmd.PersistentFlags().BoolVarP(&disableLogColor, "disable_log_color", "", false, "disable log color in console")
	cmd.PersistentFlags().BoolVarP(&tlsEnable, "tls_enable", "", false, "enable frpc tls")
}

var rootCmd = &cobra.Command{
	Use:   "frpc",
	Short: "frpc is the client of frp (https://github.com/fatedier/frp)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(version.Full())
			return nil
		}

		// If cfgDir is not empty, run multiple frpc service for each config file in cfgDir.
		// Note that it's only designed for testing. It's not guaranteed to be stable.
		if cfgDir != "" {
			var wg sync.WaitGroup
			err := filepath.WalkDir(cfgDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				wg.Add(1)
				time.Sleep(time.Millisecond)
				go func() {
					defer wg.Done()
					err := runClient(path)
					if err != nil {
						fmt.Printf("frpc service error for config file [%s]\n", path)
					}
				}()
				return nil
			})
			if err != nil {
				return err
			}
			wg.Wait()
			return nil
		}

		// Do not show command usage here.
		err := runClient(cfgFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func handleSignal(svr *client.Service, doneCh chan struct{}) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	svr.GracefulClose(500 * time.Millisecond)
	close(doneCh)
}

func parseClientCommonCfgFromCmd() (cfg config.ClientCommonConf, err error) {
	cfg = config.GetDefaultClientConf()

	ipStr, portStr, err := net.SplitHostPort(serverAddr)
	if err != nil {
		err = fmt.Errorf("invalid server_addr: %v", err)
		return
	}

	cfg.ServerAddr = ipStr
	cfg.ServerPort, err = strconv.Atoi(portStr)
	if err != nil {
		err = fmt.Errorf("invalid server_addr: %v", err)
		return
	}

	cfg.User = user
	cfg.Protocol = protocol
	cfg.LogLevel = logLevel
	cfg.LogFile = logFile
	cfg.LogMaxDays = int64(logMaxDays)
	cfg.DisableLogColor = disableLogColor

	// Only token authentication is supported in cmd mode
	cfg.ClientConfig = auth.GetDefaultClientConf()
	cfg.Token = token
	cfg.TLSEnable = tlsEnable

	cfg.Complete()
	if err = cfg.Validate(); err != nil {
		err = fmt.Errorf("parse config error: %v", err)
		return
	}
	return
}

func runClient(cfgFilePath string) error {
	cfg, pxyCfgs, visitorCfgs, err := config.ParseClientConfig(cfgFilePath)
	if err != nil {
		return err
	}
	//return startService(cfg, pxyCfgs, visitorCfgs, cfgFilePath)
	//FrpcLib增加返回
	err, _ = startService(cfg, pxyCfgs, visitorCfgs, cfgFilePath)
	return err
}

func startService(
	cfg config.ClientCommonConf,
	pxyCfgs map[string]config.ProxyConf,
	visitorCfgs map[string]config.VisitorConf,
	cfgFile string,
) (err error, svr *client.Service) { //FrpcLib增加出参svr

	log.InitLog(cfg.LogWay, cfg.LogFile, cfg.LogLevel,
		cfg.LogMaxDays, cfg.DisableLogColor)

	if cfgFile != "" {
		log.Trace("start frpc service for config file [%s]", cfgFile)
		defer log.Trace("frpc service for config file [%s] stopped", cfgFile)
	}
	svr, errRet := client.NewService(cfg, pxyCfgs, visitorCfgs, cfgFile)
	if errRet != nil {
		err = errRet
		return
	}

	closedDoneCh := make(chan struct{})
	shouldGracefulClose := cfg.Protocol == "kcp" || cfg.Protocol == "quic"
	// Capture the exit signal if we use kcp or quic.
	if shouldGracefulClose {
		go handleSignal(svr, closedDoneCh)
	}

	err = svr.Run()
	if err == nil && shouldGracefulClose {
		<-closedDoneCh
	}
	return
}

//FrpcLib增加部分
var services = make(map[string]*client.Service)

func GetUids() (uids []string) {
	keys := make([]string, 0, len(services))
	for k := range services {
		keys = append(keys, k)
	}
	return keys
}

func IsRunning(uid string) (running bool) {
	return services[uid] != nil
}

func getServiceByUid(uid string) (svr *client.Service) {
	return services[uid]
}

func putServiceByUid(uid string, svr *client.Service) {
	services[uid] = svr
}

func delServiceByUid(uid string) {
	delete(services, uid)
}

func Close(uid string) (ret bool) {
	svr := getServiceByUid(uid)
	if svr != nil {
		svr.Close()
		delServiceByUid(uid)
		return true
	}
	return false
}

func parseClientCommonCfg(fileType int, source []byte) (cfg config.ClientCommonConf, err error) {
	if fileType == CfgFileTypeIni {
		cfg, err = config.UnmarshalClientConfFromIni(source)
	} else if fileType == CfgFileTypeCmd {
		cfg, err = parseClientCommonCfgFromCmd()
	}
	if err != nil {
		return
	}

	err = cfg.Validate()
	if err != nil {
		return
	}
	return
}

//goland:noinspection GoUnusedExportedFunction
func RunClient(cfgFilePath string) (err error, svr *client.Service) {
	var content []byte
	content, err = config.GetRenderedConfFromFile(cfgFilePath)
	if err != nil {
		return
	}

	cfg, err := parseClientCommonCfg(CfgFileTypeIni, content)
	if err != nil {
		return
	}

	pxyCfgs, visitorCfgs, err := config.LoadAllProxyConfsFromIni(cfg.User, content, cfg.Start)
	if err != nil {
		return
	}

	return startService(cfg, pxyCfgs, visitorCfgs, cfgFilePath)
}

func RunClientWithUid(uid string, cfgFilePath string) (err error, svr *client.Service) {
	var content []byte
	content, err = config.GetRenderedConfFromFile(cfgFilePath)
	if err != nil {
		return
	}

	cfg, err := parseClientCommonCfg(CfgFileTypeIni, content)
	if err != nil {
		return
	}

	pxyCfgs, visitorCfgs, err := config.LoadAllProxyConfsFromIni(cfg.User, content, cfg.Start)
	if err != nil {
		return
	}

	return startServiceWithUid(uid, cfg, pxyCfgs, visitorCfgs)
}

func RunClientByContent(uid string, cfgContent string) (err error, svr *client.Service) {
	content, err := config.RenderContent([]byte(cfgContent))

	if err != nil {
		return
	}

	cfg, err := parseClientCommonCfg(CfgFileTypeIni, content)
	if err != nil {
		return
	}

	pxyCfgs, visitorCfgs, err := config.LoadAllProxyConfsFromIni(cfg.User, content, cfg.Start)
	if err != nil {
		return
	}

	return startServiceWithUid(uid, cfg, pxyCfgs, visitorCfgs)
}

func startServiceWithUid(
	uid string,
	cfg config.ClientCommonConf,
	pxyCfgs map[string]config.ProxyConf,
	visitorCfgs map[string]config.VisitorConf,
) (err error, svr *client.Service) {
	defer delServiceByUid(uid)
	log.InitLog(cfg.LogWay, cfg.LogFile, cfg.LogLevel,
		cfg.LogMaxDays, cfg.DisableLogColor)

	if cfg.DNSServer != "" {
		s := cfg.DNSServer
		if !strings.Contains(s, ":") {
			s += ":53"
		}
		// Change default dns server for frpc
		net.DefaultResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return net.Dial("udp", s)
			},
		}
	}
	svr, errRet := client.NewService(cfg, pxyCfgs, visitorCfgs, cfgFile)
	if errRet != nil {
		err = errRet
		return
	}
	// Capture the exit signal if we use kcp.
	kcpDoneCh := make(chan struct{})
	if cfg.Protocol == "kcp" {
		go handleSignal(svr, kcpDoneCh)
	}
	putServiceByUid(uid, svr)

	err = svr.Run()
	if err == nil && cfg.Protocol == "kcp" {
		<-kcpDoneCh
	}

	return err, svr
}
