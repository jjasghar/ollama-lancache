package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	version     = "dev"
	commit      = "unknown"
	buildTime   = "unknown"
	
	rootCmd = &cobra.Command{
		Use:   "ollama-lancache",
		Short: "Model distribution system for Ollama",
		Long: `Ollama LanCache is a model distribution system that provides efficient sharing 
of Ollama models across a local network. It offers two approaches:

1. ollama-lancache (Recommended): Simple HTTP server with client scripts
2. Registry Proxy (Advanced): Transparent proxy for Ollama registry requests

This reduces bandwidth usage by allowing clients to download models from a local 
server instead of the internet.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildTime),
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersionInfo sets the version information for the CLI
func SetVersionInfo(v, c, bt string) {
	version = v
	commit = c
	buildTime = bt
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildTime)
}

func init() {
	cobra.OnInitialize(initConfig)
	
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ollama-lancache.yaml)")
	rootCmd.PersistentFlags().String("cache-dir", "", "directory to store cached models (default: ~/.ollama-lancache)")
	rootCmd.PersistentFlags().String("listen-addr", "0.0.0.0", "address to listen on")
	rootCmd.PersistentFlags().Int("http-port", 443, "HTTP proxy port")
	rootCmd.PersistentFlags().Int("dns-port", 53, "DNS server port")
	rootCmd.PersistentFlags().Bool("dns-enabled", true, "enable DNS server")
	rootCmd.PersistentFlags().Bool("http-enabled", true, "enable HTTP proxy")
	rootCmd.PersistentFlags().String("upstream-dns", "8.8.8.8:53", "upstream DNS server")
	rootCmd.PersistentFlags().String("ollama-registry", "registry.ollama.ai", "Ollama registry hostname to intercept")
	
	viper.BindPFlags(rootCmd.PersistentFlags())
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".ollama-lancache")
	}
	
	viper.AutomaticEnv()
	
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
