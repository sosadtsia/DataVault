package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

type BackupManager struct {
	config  Config
	gdrive  *GoogleDriveClient
	pcloud  *PCloudClient
	tempDir string
}

type BackupResult struct {
	Success   bool
	Message   string
	Error     error
	Timestamp time.Time
}

func NewBackupManager(config Config) *BackupManager {
	// Create temporary directory for backups
	tempDir := filepath.Join(os.TempDir(), "datavault_backups")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("Warning: Failed to create temp directory: %v", err)
	}

	bm := &BackupManager{
		config:  config,
		tempDir: tempDir,
	}

	// Initialize cloud clients
	if config.GoogleDriveAuth != "" {
		bm.gdrive = NewGoogleDriveClient(config.GoogleDriveAuth)
	}
	if config.PCloudAuth != "" {
		bm.pcloud = NewPCloudClient(config.PCloudAuth)
	}

	return bm
}

func (bm *BackupManager) RunBackup(ctx context.Context) error {
	log.Printf("Starting backup of: %s", bm.config.SourceFolder)

	// Create timestamp for this backup
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupName := fmt.Sprintf("backup_%s", timestamp)

	// Create backup directory
	backupPath := filepath.Join(bm.tempDir, backupName)
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	defer bm.cleanup(backupPath)

	// Copy source folder to backup directory
	destPath := filepath.Join(backupPath, filepath.Base(bm.config.SourceFolder))
	if err := bm.copyDirectory(bm.config.SourceFolder, destPath); err != nil {
		return fmt.Errorf("failed to copy source directory: %w", err)
	}

	log.Printf("Successfully copied %s to %s", bm.config.SourceFolder, destPath)

	if bm.config.DryRun {
		log.Printf("Dry run: Would upload backup to cloud drives")
		return nil
	}

	// Upload to cloud drives
	results := make(chan BackupResult, 2)

	// Google Drive upload
	if bm.gdrive != nil {
		go func() {
			result := BackupResult{Timestamp: time.Now()}
			err := bm.gdrive.UploadFolder(ctx, destPath, backupName)
			if err != nil {
				result.Error = err
				result.Message = "Google Drive upload failed"
				log.Printf("Google Drive upload failed: %v", err)
			} else {
				result.Success = true
				result.Message = "Google Drive upload successful"
				log.Printf("Successfully uploaded to Google Drive")
			}
			results <- result
		}()
	} else {
		results <- BackupResult{
			Success:   false,
			Message:   "Google Drive not configured",
			Timestamp: time.Now(),
		}
	}

	// pCloud upload
	if bm.pcloud != nil {
		go func() {
			result := BackupResult{Timestamp: time.Now()}
			err := bm.pcloud.UploadFolder(ctx, destPath, backupName)
			if err != nil {
				result.Error = err
				result.Message = "pCloud upload failed"
				log.Printf("pCloud upload failed: %v", err)
			} else {
				result.Success = true
				result.Message = "pCloud upload successful"
				log.Printf("Successfully uploaded to pCloud")
			}
			results <- result
		}()
	} else {
		results <- BackupResult{
			Success:   false,
			Message:   "pCloud not configured",
			Timestamp: time.Now(),
		}
	}

	// Wait for both uploads to complete
	successCount := 0
	for i := 0; i < 2; i++ {
		result := <-results
		if result.Success {
			successCount++
		}
		if bm.config.Verbose {
			log.Printf("Upload result: %s", result.Message)
		}
	}

	if successCount == 0 {
		return fmt.Errorf("all uploads failed")
	}

	log.Printf("Backup completed successfully (%d/2 uploads succeeded)", successCount)
	return nil
}

func (bm *BackupManager) StartScheduler(ctx context.Context) error {
	log.Printf("Starting scheduler with interval: %v", bm.config.BackupInterval)

	ticker := time.NewTicker(bm.config.BackupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Scheduler stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := bm.RunBackup(ctx); err != nil {
				log.Printf("Scheduled backup failed: %v", err)
			}
		}
	}
}

// copyDirectory recursively copies a directory using standard library
func (bm *BackupManager) copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return bm.copyFile(path, dstPath, info.Mode())
	})
}

// copyFile copies a single file using standard library
func (bm *BackupManager) copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return os.Chmod(dst, mode)
}

func (bm *BackupManager) cleanup(path string) {
	if err := os.RemoveAll(path); err != nil {
		log.Printf("Warning: Failed to cleanup temp directory %s: %v", path, err)
	}
}
