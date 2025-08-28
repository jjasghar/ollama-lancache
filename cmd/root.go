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
		Long: `Ollama LanCache is a comprehensive model distribution system for efficiently sharing 
Ollama models across a local network. 

Features:
- High-performance HTTP server with session tracking
- Cross-platform client scripts (Windows PowerShell, Linux/macOS Bash)
- Real-time monitoring with web interface and REST API
- File downloads server for additional resources
- Multi-client support with concurrent download tracking

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
