package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type GoogleDriveClient struct {
	service      *drive.Service
	authFile     string
	rootFolderID string
}

func NewGoogleDriveClient(authFile string) *GoogleDriveClient {
	client := &GoogleDriveClient{
		authFile: authFile,
	}

	if err := client.initialize(); err != nil {
		log.Printf("Failed to initialize Google Drive client: %v", err)
		return nil
	}

	return client
}

func (gdc *GoogleDriveClient) initialize() error {
	// Read credentials file
	credentials, err := os.ReadFile(gdc.authFile)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Parse credentials and create config
	config, err := google.ConfigFromJSON(credentials, drive.DriveFileScope)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	// Get HTTP client using credentials
	client := gdc.getClient(config)

	// Create Drive service
	ctx := context.Background()
	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create Drive service: %w", err)
	}

	gdc.service = srv

	// Create or find DataVault root folder
	if err := gdc.ensureRootFolder(); err != nil {
		return fmt.Errorf("failed to setup root folder: %w", err)
	}

	return nil
}

func (gdc *GoogleDriveClient) getClient(config *oauth2.Config) *http.Client {
	// This is a simplified version - in a real application you would
	// implement proper OAuth2 flow with token storage
	tok := &oauth2.Token{}
	tokFile := "token.json"

	file, err := os.Open(tokFile)
	if err != nil {
		// If no token file exists, you would need to implement OAuth flow
		log.Printf("Warning: No token file found. You need to implement OAuth flow.")
		return nil
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(tok); err != nil {
		log.Printf("Failed to decode token: %v", err)
		return nil
	}

	return config.Client(context.Background(), tok)
}

func (gdc *GoogleDriveClient) ensureRootFolder() error {
	// Search for existing DataVault folder
	query := "name='DataVault' and mimeType='application/vnd.google-apps.folder' and trashed=false"
	fileList, err := gdc.service.Files.List().Q(query).Do()
	if err != nil {
		return fmt.Errorf("failed to search for root folder: %w", err)
	}

	if len(fileList.Files) > 0 {
		gdc.rootFolderID = fileList.Files[0].Id
		log.Printf("Found existing DataVault folder: %s", gdc.rootFolderID)
		return nil
	}

	// Create DataVault folder
	folder := &drive.File{
		Name:     "DataVault",
		MimeType: "application/vnd.google-apps.folder",
	}

	file, err := gdc.service.Files.Create(folder).Do()
	if err != nil {
		return fmt.Errorf("failed to create root folder: %w", err)
	}

	gdc.rootFolderID = file.Id
	log.Printf("Created DataVault folder: %s", gdc.rootFolderID)
	return nil
}

func (gdc *GoogleDriveClient) UploadFolder(ctx context.Context, localPath, backupName string) error {
	if gdc.service == nil {
		return fmt.Errorf("Google Drive service not initialized")
	}

	log.Printf("Uploading %s to Google Drive as %s", localPath, backupName)

	// Create backup folder
	backupFolder := &drive.File{
		Name:     backupName,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{gdc.rootFolderID},
	}

	folder, err := gdc.service.Files.Create(backupFolder).Do()
	if err != nil {
		return fmt.Errorf("failed to create backup folder: %w", err)
	}

	backupFolderID := folder.Id
	log.Printf("Created backup folder: %s", backupFolderID)

	// Upload files recursively
	return gdc.uploadDirectoryRecursive(ctx, localPath, backupFolderID, "")
}

func (gdc *GoogleDriveClient) uploadDirectoryRecursive(ctx context.Context, localPath, parentID, relativePath string) error {
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fullPath := filepath.Join(localPath, entry.Name())
		currentRelativePath := filepath.Join(relativePath, entry.Name())

		if entry.IsDir() {
			// Create subdirectory
			subFolder := &drive.File{
				Name:     entry.Name(),
				MimeType: "application/vnd.google-apps.folder",
				Parents:  []string{parentID},
			}

			createdFolder, err := gdc.service.Files.Create(subFolder).Do()
			if err != nil {
				log.Printf("Failed to create folder %s: %v", currentRelativePath, err)
				continue
			}

			// Recursively upload subdirectory
			if err := gdc.uploadDirectoryRecursive(ctx, fullPath, createdFolder.Id, currentRelativePath); err != nil {
				log.Printf("Failed to upload subdirectory %s: %v", currentRelativePath, err)
			}
		} else {
			// Upload file
			if err := gdc.uploadFile(ctx, fullPath, entry.Name(), parentID); err != nil {
				log.Printf("Failed to upload file %s: %v", currentRelativePath, err)
			}
		}
	}

	return nil
}

func (gdc *GoogleDriveClient) uploadFile(ctx context.Context, localPath, fileName, parentID string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	driveFile := &drive.File{
		Name:    fileName,
		Parents: []string{parentID},
	}

	_, err = gdc.service.Files.Create(driveFile).Media(file).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	log.Printf("Uploaded file: %s", fileName)
	return nil
}

func (gdc *GoogleDriveClient) detectMimeType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".pdf":
		return "application/pdf"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
