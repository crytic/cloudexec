package main

import (
	"fmt"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/s3"
)

func Init(config config.Config) error {
	bucketName := fmt.Sprintf("cloudexec-%s", config.Username)
	// Get a list of existing buckets
	listBucketsOutput, err := s3.ListBuckets(config)
	if err != nil {
		return fmt.Errorf("Failed to list buckets: %w", err)
	}

	// Return if the desired bucket already exists
	bucketExists := false
	for _, thisBucket := range listBucketsOutput {
		if thisBucket == bucketName {
			bucketExists = true
		}
	}

	if !bucketExists {
		// Create a new bucket
		fmt.Printf("Creating new %s bucket...\n", bucketName)
		err = s3.CreateBucket(config)
		if err != nil {
			return err
		}
	}

	// Ensure versioning is enabled, necessary if bucket creation was interrupted
	err = s3.SetVersioning(config)
	if err != nil {
		return err
	}

	// Initialize bucket state if not already present
	err = initState(config, bucketName)
	if err != nil {
		return fmt.Errorf("Failed to initialize state for bucket %s: %w", bucketName, err)
	}

	return nil
}

func initState(config config.Config, bucketName string) error {
	// Check if the state directory already exists
	stateDir := "state/"
	stateDirExists, err := s3.ObjectExists(config, stateDir)
	if err != nil {
		return fmt.Errorf("Failed to check whether the state directory exists: %w", err)
	}
	// Create the state directory if it does not already exist
	if !stateDirExists {
		fmt.Printf("Creating new state directory at %s/%s\n", bucketName, stateDir)
		err = s3.PutObject(config, stateDir, []byte{})
		if err != nil {
			return fmt.Errorf("Failed to create state directory at %s/%s: %w", bucketName, stateDir, err)
		}
	}

	// Check if the state file already exists
	statePath := "state/state.json"
	statePathExists, err := s3.ObjectExists(config, statePath)
	if err != nil {
		return fmt.Errorf("Failed to check whether the state file exists: %w", err)
	}
	// Create the initial state file if it does not already exist
	if !statePathExists {
		fmt.Printf("Creating new state file at %s/%s\n", bucketName, statePath)
		err = s3.PutObject(config, statePath, []byte("{}"))
		if err != nil {
			return fmt.Errorf("Failed to create state file in bucket %s: %w", bucketName, err)
		}
	}

	return nil
}
