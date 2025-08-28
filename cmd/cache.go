package cmd

import (
	"fmt"
	"os"

	"github.com/jjasghar/ollama-lancache/internal/cache"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Cache management commands",
	Long:  `Commands for managing the Ollama model cache.`,
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	Long:  `Display statistics about the current cache including number of models, disk usage, etc.`,
	RunE:  runCacheStats,
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the cache",
	Long:  `Remove all cached models and manifests from the cache directory.`,
	RunE:  runCacheClear,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheStatsCmd)
	cacheCmd.AddCommand(cacheClearCmd)
}

func runCacheStats(cmd *cobra.Command, args []string) error {
	cacheDir := viper.GetString("cache-dir")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = home + "/.ollama-lancache"
	}

	modelCache, err := cache.New(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	stats, err := modelCache.GetCacheStats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats: %w", err)
	}

	fmt.Printf("Cache Statistics:\n")
	fmt.Printf("  Directory: %s\n", stats["cache_directory"])
	fmt.Printf("  Blobs: %d\n", stats["blob_count"])
	fmt.Printf("  Manifests: %d\n", stats["manifest_count"])
	fmt.Printf("  Total Size: %d MB (%d bytes)\n", stats["total_blob_size_mb"], stats["total_blob_size_bytes"])
	fmt.Printf("  Last Updated: %s\n", stats["last_updated"])

	return nil
}

func runCacheClear(cmd *cobra.Command, args []string) error {
	cacheDir := viper.GetString("cache-dir")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = home + "/.ollama-lancache"
	}

	fmt.Printf("Are you sure you want to clear the cache directory %s? (y/N): ", cacheDir)
	var response string
	fmt.Scanln(&response)

	if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
		fmt.Println("Cache clear cancelled.")
		return nil
	}

	// Remove the entire cache directory
	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	fmt.Printf("Cache cleared successfully.\n")
	return nil
}
