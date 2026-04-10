package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/puper/apiretry/internal/config"
	"github.com/puper/apiretry/internal/observe"
	"github.com/puper/apiretry/internal/proxy"
	"github.com/puper/apiretry/internal/retry"
	"github.com/puper/apiretry/internal/server"
	"github.com/puper/apiretry/internal/stream"
	"github.com/puper/apiretry/internal/upstream"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	logger := observe.NewLogger(&cfg.Logging)

	upstreamClient := upstream.NewClient(&cfg.Upstream)
	retryPolicy := retry.NewPolicy(&cfg.Retry)
	probe := &stream.DefaultProbe{}

	handler := proxy.NewHandler(upstreamClient, retryPolicy, probe, cfg, logger)
	router := server.NewRouter(handler, cfg, logger)

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.Info("代理服务器启动", "addr", cfg.Server.Addr, "upstream", cfg.Upstream.BaseURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("服务器异常退出", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("服务器关闭出错", "error", err)
	}

	logger.Info("服务器已停止")
}
