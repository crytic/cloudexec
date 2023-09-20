package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/s3"
)

func UploadDirectoryToSpaces(config config.Config, bucketName string, sourcePath string, destPath string) error {
	// Walk the directory and upload files recursively
	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Read the file
			fileBytes, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("Failed to read file %s: %w", path, err)
			}

			// Compute the destination key (path) in the bucket
			relativePath, _ := filepath.Rel(sourcePath, path)
			destinationKey := filepath.Join(destPath, "input", relativePath)

			err = s3.PutObject(config, bucketName, destinationKey, fileBytes)
			if err != nil {
				return err
			}

			fmt.Printf("Successfully uploaded %s to %s/%s\n", path, bucketName, destinationKey)
		}
		return nil
	})

}
