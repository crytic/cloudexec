package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/log"
	"github.com/crytic/cloudexec/pkg/s3"
)

func DownloadJobOutput(config config.Config, jobID int64, localPath string) error {

	bucketPrefix := fmt.Sprintf("job-%v/output", jobID)
	objectKeys, err := s3.ListObjects(config, bucketPrefix)
	if err != nil {
		return fmt.Errorf("Failed to list bucket objects: %w", err)
	}

	var downloadObjects func(objectKeys []string, prefix string) error
	downloadObjects = func(objectKeys []string, prefix string) error {
		for _, objectKey := range objectKeys {

			if strings.HasSuffix(objectKey, "/") {
				// It's a directory, list objects inside this directory and download them
				subdirObjects, err := s3.ListObjects(config, objectKey)
				if err != nil {
					return fmt.Errorf("Failed to list objects in %s subdirectory: %w", objectKey, err)
				}
				err = downloadObjects(subdirObjects, objectKey)
				if err != nil {
					return err
				}
			} else {
				// It's a file, download it
				body, err := s3.GetObject(config, objectKey)
				if err != nil {
					return fmt.Errorf("Failed to get %s object: %w", objectKey, err)
				}

				localFilePath := filepath.Join(localPath, strings.TrimPrefix(objectKey, prefix))
				if err := os.MkdirAll(filepath.Dir(localFilePath), 0755); err != nil {
					return fmt.Errorf("Failed to create local directory at %s: %w", localPath, err)
				}

				err = os.WriteFile(localFilePath, body, 0644)
				if err != nil {
					return fmt.Errorf("Failed to write object content to file: %w", err)
				}

				log.Good("Downloaded %s to %s", objectKey, localFilePath)
			}
		}
		return nil
	}

	// Add the logs to the output dir
	body, logErr := s3.GetObject(config, fmt.Sprintf("job-%v/cloudexec.log", jobID))

	if len(objectKeys) == 0 && logErr != nil {
		log.Info("No output or logs are available for job %v", jobID)
		return nil
	} else if len(objectKeys) == 0 {
		log.Info("No output is available for job %v", jobID)
	} else if logErr != nil {
		log.Info("No logs are available for job %v", jobID)
	}

	// Check if the local path is a directory, if not, create it
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		err = os.MkdirAll(localPath, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create local directory at %s: %w", localPath, err)
		}
	}

	// Download output, if any
	if len(objectKeys) > 0 {
		err = downloadObjects(objectKeys, bucketPrefix)
		if err != nil {
			return err
		}
	}

	// Write logs to file, if available
	if logErr == nil {
		localFilePath := filepath.Join(localPath, "cloudexec.log")
		err = os.WriteFile(localFilePath, body, 0644)
		if err != nil {
			return fmt.Errorf("Failed to write object content to file: %w", err)
		}
		log.Good("Downloaded job %v logs to %s", jobID, localFilePath)
	}

	return nil
}
