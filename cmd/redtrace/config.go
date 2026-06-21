package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config is the resolved runtime configuration assembled from defaults, the
// config file, environment variables (REDTRACE_*), and command-line flags.
type Config struct {
	DataDir       string
	DBPath        string
	CADir         string
	ProxyListen   string
	APIListen     string
	UpstreamProxy string
	Token         string
	LogLevel      string
	OOBDomain     string
	OOBPublicIP   string
	OOBHTTPListen string
	OOBDNSListen  string
}

func loadConfig() Config {
	dataDir := viper.GetString("data.dir")
	if dataDir == "" {
		dataDir = defaultDataDir()
	}
	return Config{
		DataDir:       dataDir,
		DBPath:        firstNonEmpty(viper.GetString("db.path"), filepath.Join(dataDir, "redtrace.db")),
		CADir:         firstNonEmpty(viper.GetString("ca.dir"), filepath.Join(dataDir, "ca")),
		ProxyListen:   viper.GetString("proxy.listen"),
		APIListen:     viper.GetString("api.listen"),
		UpstreamProxy: viper.GetString("upstream.proxy"),
		Token:         viper.GetString("auth.token"),
		LogLevel:      viper.GetString("log.level"),
		OOBDomain:     viper.GetString("oob.domain"),
		OOBPublicIP:   viper.GetString("oob.public-ip"),
		OOBHTTPListen: firstNonEmpty(viper.GetString("oob.http-listen"), "0.0.0.0:8888"),
		OOBDNSListen:  firstNonEmpty(viper.GetString("oob.dns-listen"), "0.0.0.0:5353"),
	}
}

// defaultDataDir returns ~/.config/redtrace, falling back to ./.redtrace if the
// home directory cannot be determined.
func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".redtrace"
	}
	return filepath.Join(home, ".config", "redtrace")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
