package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	Address   string
	Database  string
	SelfCheck bool
}

func parseConfig(args []string, getenv func(string) string) (config, error) {
	flags := flag.NewFlagSet("bioacoustic-corpus-release", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	address := flags.String("addr", defaultAddress, "HTTP 监听地址")
	database := flags.String("db", "bioacoustic-corpus.db", "SQLite 数据库路径")
	selfCheck := flags.Bool("self-check", false, "执行端到端自检后退出")
	if err := flags.Parse(args); err != nil {
		return config{}, fmt.Errorf("解析参数: %w", err)
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("存在无法识别的位置参数")
	}
	addrExplicit := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "addr" {
			addrExplicit = true
		}
	})
	if !addrExplicit {
		if rawPort := strings.TrimSpace(getenv("PORT")); rawPort != "" {
			port, err := strconv.Atoi(rawPort)
			if err != nil || port < 1024 || port > 65535 {
				return config{}, fmt.Errorf("PORT 须为 1024 至 65535 的端口号")
			}
			*address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		}
	}
	if err := validateAddress(*address); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*database) == "" {
		return config{}, fmt.Errorf("数据库路径不能为空")
	}
	return config{Address: *address, Database: *database, SelfCheck: *selfCheck}, nil
}

func validateAddress(address string) error {
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 须为 host:port: %w", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1024 || port > 65535 {
		return fmt.Errorf("-addr 端口须在 1024 至 65535 之间")
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("-addr 必须绑定回环地址")
		}
	}
	return nil
}

func environment(name string) string { return os.Getenv(name) }
