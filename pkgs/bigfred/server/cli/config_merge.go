package cli

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/keskad/loco/pkgs/bigfred/server/cli/config"
)

func mergeConfigFile(cmd *cobra.Command, f *Flags, log *logrus.Logger) error {
	cfg, err := config.LoadOrCreate(config.DefaultPath, log)
	if err != nil {
		return err
	}
	changed := func(name string) bool {
		return cmd.Flags().Changed(name)
	}
	applyConfig(f, cfg, changed)
	return nil
}

func applyConfig(f *Flags, cfg *config.File, changed func(string) bool) {
	if cfg == nil {
		return
	}
	if !changed("http") && cfg.HTTP != "" {
		f.HTTPAddr = cfg.HTTP
	}
	if !changed("db") && cfg.DB != "" {
		f.DBPath = cfg.DB
	}
	if !changed("jwt-secret") {
		f.JWTSecret = cfg.JWTSecret
	}
	if !changed("cors-origin") && len(cfg.CorsOrigins) > 0 {
		f.AllowedOrigins = cfg.CorsOrigins
	}
	if !changed("secure-cookie") && cfg.SecureCookie != nil {
		f.SecureCookie = *cfg.SecureCookie
	}
	if !changed("no-supervisor") && cfg.NoSupervisor != nil {
		f.NoSupervisor = *cfg.NoSupervisor
	}
	if !changed("log-level") && cfg.LogLevel != "" {
		f.LogLevel = cfg.LogLevel
	}
	if !changed("redis-bin") && cfg.RedisBin != "" {
		f.RedisBin = cfg.RedisBin
	}
	if !changed("redis-bind") && cfg.RedisBindAddr != "" {
		f.RedisBindAddr = cfg.RedisBindAddr
	}
	if !changed("redis-port") && cfg.RedisPort != nil {
		f.RedisPort = *cfg.RedisPort
	}
	if !changed("redis-data-dir") && cfg.RedisDataDir != "" {
		f.RedisDataDir = cfg.RedisDataDir
	}
	if !changed("redis-addr") && cfg.RedisAddr != "" {
		f.RedisAddr = cfg.RedisAddr
	}
	if !changed("redis-external") && cfg.RedisExternal != nil {
		f.RedisExternal = *cfg.RedisExternal
	}
	if !changed("redis-auto-detect") && cfg.RedisAutoDetect != nil {
		f.RedisAutoDetect = *cfg.RedisAutoDetect
	}
	if !changed("redis-rdb-save") && cfg.RedisRDBSave != nil {
		f.RedisRDBSave = cfg.RedisRDBSave
	}
	if !changed("redis-no-persist") && cfg.RedisNoPersist != nil {
		f.RedisNoPersist = *cfg.RedisNoPersist
	}
	if !changed("enable-telemetry") && cfg.EnableTelemetry != nil {
		f.EnableTelemetry = *cfg.EnableTelemetry
	}
	if !changed("telemetry-config") && cfg.TelemetryConfig != "" {
		f.TelemetryConfig = cfg.TelemetryConfig
	}
}
