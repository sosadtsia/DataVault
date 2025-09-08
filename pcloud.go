package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type PCloudClient struct {
	authToken    string
	baseURL      string
	client       *http.Client
	rootFolderID int64
}

type PCloudResponse struct {
	Result    int    `json:"result"`
	Error     string `json:"error,omitempty"`
	ErrorCode int    `json:"errorcode,omitempty"`
}

type PCloudFolder struct {
	PCloudResponse
	Metadata struct {
		Name     string `json:"name"`
		FolderID int64  `json:"folderid"`
		ParentID int64  `json:"parentfolderid"`
	} `json:"metadata"`
}

type PCloudFile struct {
	PCloudResponse
	Metadata []struct {
		Name   string `json:"name"`
		FileID int64  `json:"fileid"`
		Size   int64  `json:"size"`
	} `json:"metadata"`
}

type PCloudListFolder struct {
	PCloudResponse
	Metadata struct {
		Contents []struct {
			Name     string `json:"name"`
			FolderID int64  `json:"folderid,omitempty"`
			FileID   int64  `json:"fileid,omitempty"`
			IsFolder bool   `json:"isfolder"`
		} `json:"contents"`
	} `json:"metadata"`
}

func NewPCloudClient(authToken string) *PCloudClient {
	client := &PCloudClient{
		authToken: authToken,
		baseURL:   "https://api.pcloud.com",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if err := client.initialize(); err != nil {
		log.Printf("Failed to initialize pCloud client: %v", err)
		return nil
	}

	return client
}

func (pc *PCloudClient) initialize() error {
	// Find or create DataVault root folder
	if err := pc.ensureRootFolder(); err != nil {
		return fmt.Errorf("failed to setup root folder: %w", err)
	}

	return nil
}

func (pc *PCloudClient) makeRequest(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	url := pc.baseURL + "/" + endpoint

	// Add auth token to params
	if params == nil {
		params = make(map[string]string)
	}
	params["access_token"] = pc.authToken

	// Build query parameters
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	for key, value := range params {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := pc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (pc *PCloudClient) ensureRootFolder() error {
	// List contents of root folder to find DataVault folder
	body, err := pc.makeRequest(context.Background(), "listfolder", map[string]string{
		"folderid": "0", // Root folder
	})
	if err != nil {
		return fmt.Errorf("failed to list root folder: %w", err)
	}

	var listResp PCloudListFolder
	if err := json.Unmarshal(body, &listResp); err != nil {
		return fmt.Errorf("failed to parse list response: %w", err)
	}

	if listResp.Result != 0 {
		return fmt.Errorf("pCloud API error: %s", listResp.Error)
	}

	// Look for existing DataVault folder
	for _, item := range listResp.Metadata.Contents {
		if item.IsFolder && item.Name == "DataVault" {
			pc.rootFolderID = item.FolderID
			log.Printf("Found existing DataVault folder: %d", pc.rootFolderID)
			return nil
		}
	}

	// Create DataVault folder if it doesn't exist
	body, err = pc.makeRequest(context.Background(), "createfolder", map[string]string{
		"folderid": "0",
		"name":     "DataVault",
	})
	if err != nil {
		return fmt.Errorf("failed to create DataVault folder: %w", err)
	}

	var folderResp PCloudFolder
	if err := json.Unmarshal(body, &folderResp); err != nil {
		return fmt.Errorf("failed to parse folder response: %w", err)
	}

	if folderResp.Result != 0 {
		return fmt.Errorf("pCloud API error: %s", folderResp.Error)
	}

	pc.rootFolderID = folderResp.Metadata.FolderID
	log.Printf("Created DataVault folder: %d", pc.rootFolderID)
	return nil
}

func (pc *PCloudClient) UploadFolder(ctx context.Context, localPath, backupName string) error {
	log.Printf("Uploading %s to pCloud as %s", localPath, backupName)

	// Create backup folder
	body, err := pc.makeRequest(ctx, "createfolder", map[string]string{
		"folderid": strconv.FormatInt(pc.rootFolderID, 10),
		"name":     backupName,
	})
	if err != nil {
		return fmt.Errorf("failed to create backup folder: %w", err)
	}

	var folderResp PCloudFolder
	if err := json.Unmarshal(body, &folderResp); err != nil {
		return fmt.Errorf("failed to parse folder response: %w", err)
	}

	if folderResp.Result != 0 {
		return fmt.Errorf("pCloud API error: %s", folderResp.Error)
	}

	backupFolderID := folderResp.Metadata.FolderID
	log.Printf("Created backup folder: %d", backupFolderID)

	// Upload files recursively
	return pc.uploadDirectoryRecursive(ctx, localPath, backupFolderID, "")
}

func (pc *PCloudClient) uploadDirectoryRecursive(ctx context.Context, localPath string, parentFolderID int64, relativePath string) error {
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
			body, err := pc.makeRequest(ctx, "createfolder", map[string]string{
				"folderid": strconv.FormatInt(parentFolderID, 10),
				"name":     entry.Name(),
			})
			if err != nil {
				log.Printf("Failed to create folder %s: %v", currentRelativePath, err)
				continue
			}

			var folderResp PCloudFolder
			if err := json.Unmarshal(body, &folderResp); err != nil {
				log.Printf("Failed to parse folder response for %s: %v", currentRelativePath, err)
				continue
			}

			if folderResp.Result != 0 {
				log.Printf("pCloud API error for folder %s: %s", currentRelativePath, folderResp.Error)
				continue
			}

			// Recursively upload subdirectory
			if err := pc.uploadDirectoryRecursive(ctx, fullPath, folderResp.Metadata.FolderID, currentRelativePath); err != nil {
				log.Printf("Failed to upload subdirectory %s: %v", currentRelativePath, err)
			}
		} else {
			// Upload file
			if err := pc.uploadFile(ctx, fullPath, entry.Name(), parentFolderID); err != nil {
				log.Printf("Failed to upload file %s: %v", currentRelativePath, err)
			}
		}
	}

	return nil
}

func (pc *PCloudClient) uploadFile(ctx context.Context, localPath, fileName string, parentFolderID int64) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add form fields
	writer.WriteField("access_token", pc.authToken)
	writer.WriteField("folderid", strconv.FormatInt(parentFolderID, 10))

	// Add file
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	contentType := writer.FormDataContentType()
	writer.Close()

	// Create upload request
	url := pc.baseURL + "/uploadfile"
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	resp, err := pc.client.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read upload response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with HTTP %d: %s", resp.StatusCode, string(body))
	}

	var fileResp PCloudFile
	if err := json.Unmarshal(body, &fileResp); err != nil {
		return fmt.Errorf("failed to parse upload response: %w", err)
	}

	if fileResp.Result != 0 {
		return fmt.Errorf("pCloud upload error: %s", fileResp.Error)
	}

	log.Printf("Uploaded file: %s", fileName)
	return nil
}
