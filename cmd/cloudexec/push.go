package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/s3"
)

func UploadDirectoryToSpaces(config config.Config, bucket string, sourcePath string, destPath string) error {
	// Compute the path for the zipped archive of sourcePath
	zipFileName := "input.zip"
	zipFilePath := filepath.Join(os.TempDir(), zipFileName)

	// Create a file where we will write the zipped archive
	fmt.Printf("Creating zipped archive at %s\n", zipFilePath)
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	// Create a new zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk the directory and recursively add files to the zipped archive
	err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		target := path
		if err != nil {
			return err
		}

		// If it's a symbolic link, resolve the target
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			target, err = os.Readlink(path)
			fmt.Printf("Resolved link from %s to %s\n", path, target)
			if err != nil {
				return err
			}
		}

		// If this is a subdirectory, make sure the path ends with a trailing slash before we create it
		// See https://pkg.go.dev/archive/zip#Writer.Create for details
		targetInfo, err := os.Stat(target)
		if err != nil {
			return err
		}

		if targetInfo.IsDir() {
			cleanPath := filepath.Clean(path) + string(filepath.Separator)
			fmt.Printf("Creating directory %s in the zipped archive\n", cleanPath)
			_, err = zipWriter.Create(cleanPath)
			if err != nil {
				return err
			}
			return nil
		}

		// Don't recursively add this zipped archive
		if filepath.Base(path) == zipFileName {
			return nil
		}

		fmt.Printf("Adding %s to the zipped archive\n", target)

		// Create a new file entry in the zipped archive
		zipFileEntry, err := zipWriter.Create(path)
		if err != nil {
			return err
		}

		// Open the file we're adding to the zipped archive
		file, err := os.Open(target)
		if err != nil {
			return err
		}

		// Write this file to the zipped archive
		_, err = io.Copy(zipFileEntry, file)
		if err != nil {
			return err
		}

		// Explicitly close the file once we're done to prevent a "too many open files" error
		err = file.Close()
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("Successfully added all files from %s to zipped archive at %s\n", sourcePath, zipFilePath)

	// Make sure all prior writes are sync'd to the filesystem
	// This is necessary bc we're going to read the file right after writing it
	err = zipWriter.Flush()
	if err != nil {
		return err
	}
	err = zipFile.Sync()
	if err != nil {
		return err
	}

	// Manually Closing is necessary to prevent zip file corruption during upload
	err = zipWriter.Close()
	if err != nil {
		return err
	}
	err = zipFile.Close()
	if err != nil {
		return err
	}

	// Read the zipped archive
	fileBytes, err := os.ReadFile(zipFilePath)
	if err != nil {
		return fmt.Errorf("Failed to read zipped archive %s: %w", zipFilePath, err)
	}
	if len(fileBytes) == 0 {
		return fmt.Errorf("Failed to read zipped archive at %s: read zero bytes of data", zipFilePath)
	}

	// Upload the zipped archive
	destKey := filepath.Join(destPath, "input.zip")
	fmt.Printf("Uploading archive (%v bytes) to %s\n", len(fileBytes), destKey)
	err = s3.PutObject(config, bucket, destKey, fileBytes)
	if err != nil {
		return err
	}

	return nil
}
