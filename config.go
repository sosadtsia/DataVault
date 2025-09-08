package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type ConfigFile struct {
	SourceFolder    string   `json:"source_folder"`
	BackupInterval  string   `json:"backup_interval"` // e.g., "1h", "30m", "2h30m"
	GoogleDriveAuth string   `json:"google_drive_auth"`
	PCloudAuth      string   `json:"pcloud_auth"`
	Excludes        []string `json:"excludes,omitempty"`
	DryRun          bool     `json:"dry_run,omitempty"`
	Verbose         bool     `json:"verbose,omitempty"`
	MaxBackups      int      `json:"max_backups,omitempty"` // Max number of backups to keep
}

func LoadConfig(configPath string) (*ConfigFile, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config file if it doesn't exist
			return createDefaultConfig(configPath)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func SaveConfig(config *ConfigFile, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func createDefaultConfig(configPath string) (*ConfigFile, error) {
	config := &ConfigFile{
		SourceFolder:    "",
		BackupInterval:  "1h",
		GoogleDriveAuth: "",
		PCloudAuth:      "",
		Excludes: []string{
			".git",
			".DS_Store",
			"Thumbs.db",
			"*.tmp",
			"*.log",
		},
		DryRun:     false,
		Verbose:    false,
		MaxBackups: 30, // Keep last 30 backups
	}

	if err := SaveConfig(config, configPath); err != nil {
		return nil, fmt.Errorf("failed to create default config: %w", err)
	}

	fmt.Printf("Created default configuration file at: %s\n", configPath)
	fmt.Printf("Please edit the configuration file to set your source folder and authentication details.\n")

	return config, nil
}

func MergeConfigWithFlags(config *ConfigFile, flags Config) Config {
	result := flags

	// Override with config file values if flags are not set
	if result.SourceFolder == "" && config.SourceFolder != "" {
		result.SourceFolder = config.SourceFolder
	}

	if result.GoogleDriveAuth == "" && config.GoogleDriveAuth != "" {
		result.GoogleDriveAuth = config.GoogleDriveAuth
	}

	if result.PCloudAuth == "" && config.PCloudAuth != "" {
		result.PCloudAuth = config.PCloudAuth
	}

	// Parse backup interval from config if not set via flag
	if result.BackupInterval == time.Hour && config.BackupInterval != "" {
		if interval, err := time.ParseDuration(config.BackupInterval); err == nil {
			result.BackupInterval = interval
		}
	}

	// Use config file boolean values if not explicitly set via flags
	if !flags.DryRun && config.DryRun {
		result.DryRun = config.DryRun
	}

	if !flags.Verbose && config.Verbose {
		result.Verbose = config.Verbose
	}

	return result
}

func ValidateConfig(config Config) error {
	if config.SourceFolder == "" {
		return fmt.Errorf("source folder must be specified")
	}

	if _, err := os.Stat(config.SourceFolder); os.IsNotExist(err) {
		return fmt.Errorf("source folder does not exist: %s", config.SourceFolder)
	}

	if config.GoogleDriveAuth == "" && config.PCloudAuth == "" {
		return fmt.Errorf("at least one cloud storage authentication must be configured")
	}

	if config.GoogleDriveAuth != "" {
		if _, err := os.Stat(config.GoogleDriveAuth); os.IsNotExist(err) {
			return fmt.Errorf("Google Drive auth file does not exist: %s", config.GoogleDriveAuth)
		}
	}

	if config.BackupInterval < time.Minute {
		return fmt.Errorf("backup interval must be at least 1 minute")
	}

	return nil
}
