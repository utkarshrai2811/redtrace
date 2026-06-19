package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:           "redtrace",
	Short:         "RedTrace — open-source web security testing platform",
	Long:          "RedTrace is an intercepting proxy and web-application security toolkit,\na free and open-source alternative to Burp Suite Pro.",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command, exiting non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfgFile, "config", "", "config file (default: <data-dir>/redtrace.yaml)")
	pf.String("data-dir", "", "data directory for the database and CA (default: ~/.config/redtrace)")
	pf.String("log-level", "", "log level: debug, info, warn, error (default: info)")
	_ = viper.BindPFlag("data.dir", pf.Lookup("data-dir"))
	_ = viper.BindPFlag("log.level", pf.Lookup("log-level"))

	rootCmd.AddCommand(serveCmd, caCmd)
}

func initConfig() {
	viper.SetDefault("proxy.listen", "127.0.0.1:8080")
	viper.SetDefault("api.listen", "127.0.0.1:9090")
	viper.SetDefault("data.dir", defaultDataDir())
	viper.SetDefault("log.level", "info")

	viper.SetEnvPrefix("REDTRACE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(viper.GetString("data.dir"))
		viper.AddConfigPath(".")
		viper.SetConfigName("redtrace")
		viper.SetConfigType("yaml")
	}
	// A missing config file is fine; other read errors are reported on use.
	_ = viper.ReadInConfig()
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
