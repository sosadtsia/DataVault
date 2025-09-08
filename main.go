package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type Config struct {
	SourceFolder    string
	BackupInterval  time.Duration
	ConfigFile      string
	GoogleDriveAuth string
	PCloudAuth      string
	DryRun          bool
	Verbose         bool
}

func main() {
	var config Config

	// CLI flags using standard library
	flag.StringVar(&config.SourceFolder, "source", "", "Source folder to backup (required)")
	flag.DurationVar(&config.BackupInterval, "interval", time.Hour, "Backup interval (default: 1h)")
	flag.StringVar(&config.ConfigFile, "config", "datavault.json", "Configuration file path")
	flag.StringVar(&config.GoogleDriveAuth, "gdrive-auth", "", "Google Drive authentication JSON file path")
	flag.StringVar(&config.PCloudAuth, "pcloud-auth", "", "pCloud authentication token")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Show what would be backed up without actually doing it")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "DataVault - CLI tool for seamless data backup to multiple cloud drives\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -source ~/Documents -gdrive-auth ./auth.json -pcloud-auth token123\n", os.Args[0])
	}

	flag.Parse()

	// Load configuration file
	configFile, err := LoadConfig(config.ConfigFile)
	if err != nil {
		log.Printf("Warning: Failed to load config file: %v", err)
		configFile = &ConfigFile{} // Use empty config
	}

	// Merge config file with command line flags
	config = MergeConfigWithFlags(configFile, config)

	// Validate configuration
	if err := ValidateConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Convert source folder to absolute path
	absPath, err := filepath.Abs(config.SourceFolder)
	if err != nil {
		log.Fatalf("Error resolving source path: %v", err)
	}
	config.SourceFolder = absPath

	// Setup logging
	if config.Verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	log.Printf("DataVault starting...")
	log.Printf("Source folder: %s", config.SourceFolder)
	log.Printf("Backup interval: %v", config.BackupInterval)
	log.Printf("Dry run: %v", config.DryRun)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()
	}()

	// Initialize backup manager
	backupManager := NewBackupManager(config)

	// Run initial backup
	if err := backupManager.RunBackup(ctx); err != nil {
		log.Printf("Initial backup failed: %v", err)
	}

	// Start scheduled backups
	if err := backupManager.StartScheduler(ctx); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	log.Printf("DataVault shutdown complete")
}
