package main

import (
	"io"
	"fmt"
	"os"
	"path/filepath"
  "archive/zip"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/s3"
)

func UploadDirectoryToSpaces(config config.Config, bucketName string, sourcePath string, destPath string) error {
  // Create a file to store the compressed archive of the input sourcePath
  zipFilePath := filepath.Join(filepath.Dir(sourcePath), filepath.Base(sourcePath) + ".zip")
  zipFile, err := os.Create(zipFilePath)
  if err != nil {
      return err
  }
  defer zipFile.Close()

  archive := zip.NewWriter(zipFile)
  defer archive.Close()

	// Walk the directory and recursively add files to the zip archive
  err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
    fmt.Printf("Adding %s to the zipped archive", path)

    // Create a new ZIP file entry
    zipFileEntry, err := archive.Create(path[len(sourcePath):])
    if err != nil {
        return err
    }

    // If this is a subdirectory, we're done. Move on.
		if info.IsDir() {
      return nil
    }

    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()

    _, err = io.Copy(zipFileEntry, file)
    if err != nil {
        return err
    }

		return nil
	})
  if err != nil {
      return err
  }
  fmt.Printf("Successfully added all files from %s to zipped archive at %s\n", sourcePath, zipFilePath)

  // Read the zipped archive
  fileBytes, err := os.ReadFile(zipFilePath)
  if err != nil {
    return fmt.Errorf("Failed to read zipped archive %s: %w", zipFilePath, err)
  }

  // Compute the destination key in the bucket
  relativePath, _ := filepath.Rel(sourcePath, zipFilePath)
  destinationKey := filepath.Join(destPath, "input", relativePath)

  err = s3.PutObject(config, bucketName, destinationKey, fileBytes)
  if err != nil {
      return err
  }

  fmt.Printf("Successfully uploaded %s to %s/%s\n", zipFilePath, bucketName, destinationKey)
  return nil
}
