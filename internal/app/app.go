package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"xraytool/internal/api"
	"xraytool/internal/auth"
	"xraytool/internal/config"
	"xraytool/internal/db"
	"xraytool/internal/service"
	"xraytool/internal/store"

	"go.uber.org/zap"
)

func Run() error {
	cfg := config.Load()
	if err := config.EnsurePaths(cfg); err != nil {
		return err
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}

	st := store.New(database)
	if err := st.EnsureDefaultSettings(cfg.DefaultInboundPort, cfg.BarkBaseURLFallback); err != nil {
		return err
	}
	hash, err := auth.HashPassword(cfg.DefaultAdminPass)
	if err != nil {
		return err
	}
	if err := st.EnsureAdmin(cfg.DefaultAdminUser, hash); err != nil {
		return err
	}

	hostSvc := service.NewHostIPService(database)
	_, _ = hostSvc.ScanAndSync()

	if cfg.ManagedXrayEnabled {
		resolvedAPIServer, err := resolveManagedXrayAPIServer(cfg.XrayAPIServer)
		if err != nil {
			return err
		}
		cfg.XrayAPIServer = resolvedAPIServer
	}

	xrayManager := service.NewXrayManager(cfg, database, logger)
	if err := xrayManager.StartManaged(); err != nil {
		st.AddTaskLog("error", "start managed xray failed", err.Error())
		return fmt.Errorf("start managed xray failed: %w", err)
	}

	orderSvc := service.NewOrderService(database, xrayManager, logger)
	barkSvc := service.NewBarkService(database)
	runtimeSvc := service.NewRuntimeStatsService(database, xrayManager)
	backupSvc := service.NewBackupService(cfg, database, logger)
	scheduler := service.NewScheduler(database, orderSvc, barkSvc, logger, cfg.SchedulerInterval)

	engine := api.New(database, st, orderSvc, hostSvc, backupSvc, barkSvc, runtimeSvc, cfg, logger).Router()
	if err := ensureListenAddrAvailable(cfg.ListenAddr); err != nil {
		return err
	}
	httpServer := &http.Server{Addr: cfg.ListenAddr, Handler: engine}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go scheduler.Start(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		xrayManager.StopManaged()
	}()

	logger.Info("xraytool started", zap.String("listen", cfg.ListenAddr), zap.String("db", cfg.DBPath))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

func ensureListenAddrAvailable(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen address unavailable %s: %w", addr, err)
	}
	return ln.Close()
}

func resolveManagedXrayAPIServer(raw string) (string, error) {
	apiAddr := strings.TrimSpace(raw)
	if apiAddr == "" {
		apiAddr = "127.0.0.1:10085"
	}
	if strings.EqualFold(apiAddr, "auto") || strings.HasSuffix(apiAddr, ":0") {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", fmt.Errorf("allocate managed xray api port failed: %w", err)
		}
		addr := ln.Addr().String()
		_ = ln.Close()
		return addr, nil
	}
	if err := ensureListenAddrAvailable(apiAddr); err != nil {
		return "", fmt.Errorf("managed xray api address unavailable %s: %w", apiAddr, err)
	}
	return apiAddr, nil
}
