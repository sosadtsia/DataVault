# DataVault

A powerful CLI tool for seamless data backup to multiple cloud drives (Google Drive and pCloud). DataVault automatically clones your selected folder and uploads it to both cloud services every hour, ensuring your data is always protected.

## Features

- 🔄 **Automated Backups**: Scheduled backups every hour (configurable)
- ☁️ **Multi-Cloud Support**: Simultaneous backup to Google Drive and pCloud
- 📁 **Complete Folder Cloning**: Preserves directory structure and file permissions
- ⚙️ **Flexible Configuration**: Command-line flags and JSON configuration file support
- 🛡️ **Graceful Shutdown**: Handles interruption signals properly
- 🔍 **Verbose Logging**: Detailed logging for monitoring and troubleshooting
- 🧪 **Dry Run Mode**: Test your backup configuration without actually uploading
- 🔐 **Secure Authentication**: Uses standard OAuth2 for Google Drive and API tokens for pCloud

## Installation

### Prerequisites

- Go 1.21 or later
- Google Cloud Platform account (for Google Drive)
- pCloud account (for pCloud storage)

### Building from Source

```bash
git clone <repository-url>
cd DataVault
go mod download
go build -o datavault .
```

## Quick Start

1. **Create a configuration file**:
   ```bash
   ./datavault -config datavault.json
   ```
   This will create a default configuration file that you can edit.

2. **Edit the configuration file** (`datavault.json`):
   ```json
   {
     "source_folder": "/path/to/your/documents",
     "backup_interval": "1h",
     "google_drive_auth": "/path/to/google-credentials.json",
     "pcloud_auth": "your-pcloud-token",
     "excludes": [
       ".git",
       ".DS_Store",
       "Thumbs.db",
       "*.tmp",
       "*.log"
     ],
     "dry_run": false,
     "verbose": false,
     "max_backups": 30
   }
   ```

3. **Run DataVault**:
   ```bash
   ./datavault
   ```

## Configuration

### Command Line Flags

```bash
./datavault [OPTIONS]

Options:
  -source string
        Source folder to backup (required)
  -interval duration
        Backup interval (default: 1h)
  -config string
        Configuration file path (default: "datavault.json")
  -gdrive-auth string
        Google Drive authentication JSON file path
  -pcloud-auth string
        pCloud authentication token
  -dry-run
        Show what would be backed up without actually doing it
  -verbose
        Enable verbose logging
```

### Configuration File

DataVault supports JSON configuration files with the following options:

| Field | Type | Description |
|-------|------|-------------|
| `source_folder` | string | Path to the folder you want to backup |
| `backup_interval` | string | Backup frequency (e.g., "1h", "30m", "2h30m") |
| `google_drive_auth` | string | Path to Google Drive credentials JSON file |
| `pcloud_auth` | string | pCloud API access token |
| `excludes` | []string | File/folder patterns to exclude from backup |
| `dry_run` | boolean | Enable dry run mode |
| `verbose` | boolean | Enable verbose logging |
| `max_backups` | int | Maximum number of backups to keep |

## Authentication Setup

### Google Drive Setup

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Drive API
4. Create credentials (OAuth 2.0 Client ID) for a desktop application
5. Download the credentials JSON file
6. Set the path to this file in your configuration

**Note**: The first time you run DataVault with Google Drive, you'll need to complete the OAuth flow in your browser.

### pCloud Setup

1. Log in to your pCloud account
2. Go to Account Settings > Security
3. Generate an API access token
4. Use this token in your configuration

## Usage Examples

### Basic Usage
```bash
# Backup ~/Documents every hour to both cloud services
./datavault -source ~/Documents -gdrive-auth ./credentials.json -pcloud-auth mytoken123
```

### Custom Interval
```bash
# Backup every 30 minutes
./datavault -source ~/Documents -interval 30m -gdrive-auth ./credentials.json -pcloud-auth mytoken123
```

### Dry Run
```bash
# Test your configuration without actually uploading
./datavault -source ~/Documents -gdrive-auth ./credentials.json -pcloud-auth mytoken123 -dry-run
```

### Using Configuration File
```bash
# Use configuration file (recommended)
./datavault -config ./my-backup-config.json
```

### Verbose Logging
```bash
# Enable detailed logging
./datavault -verbose
```

## How It Works

1. **Folder Cloning**: DataVault creates a complete copy of your source folder in a temporary directory
2. **Timestamp Creation**: Each backup is tagged with a timestamp (e.g., `backup_2024-01-15_14-30-25`)
3. **Parallel Upload**: The backup is uploaded simultaneously to both Google Drive and pCloud
4. **Cleanup**: Temporary files are automatically cleaned up after upload
5. **Scheduling**: The process repeats according to your specified interval

## Folder Structure

DataVault creates the following structure in your cloud drives:

```
DataVault/
├── backup_2024-01-15_13-00-00/
│   └── [Your folder contents]
├── backup_2024-01-15_14-00-00/
│   └── [Your folder contents]
└── backup_2024-01-15_15-00-00/
    └── [Your folder contents]
```

## Error Handling

DataVault includes comprehensive error handling:

- Network connectivity issues are logged and retried
- Partial upload failures are reported but don't stop the entire backup
- Authentication errors are clearly reported
- File system errors are handled gracefully

## Performance Considerations

- Large folders may take time to backup initially
- Subsequent backups are full copies (incremental backup is not yet supported)
- Upload speed depends on your internet connection and cloud service limits
- Temporary storage space required equals the size of your source folder

## Troubleshooting

### Common Issues

1. **"Source folder does not exist"**
   - Check that the path in your configuration is correct
   - Use absolute paths for reliability

2. **"Google Drive authentication failed"**
   - Verify your credentials JSON file is valid
   - Complete the OAuth flow in your browser
   - Check that the Google Drive API is enabled

3. **"pCloud upload failed"**
   - Verify your API token is correct and active
   - Check your pCloud account storage limits

4. **"Permission denied"**
   - Ensure DataVault has read access to your source folder
   - Check file system permissions

### Debug Mode

Run with `-verbose` flag to see detailed logs:

```bash
./datavault -verbose
```

## Security Notes

- Credentials are stored locally and never transmitted to unauthorized services
- Google Drive uses OAuth2 with secure token refresh
- pCloud API tokens should be kept secure
- All uploads use HTTPS encryption

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for bugs and feature requests.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Changelog

### v1.0.0
- Initial release
- Google Drive integration
- pCloud integration
- Scheduled backups
- Configuration file support
- Graceful shutdown handling
