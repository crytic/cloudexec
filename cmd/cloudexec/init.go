package main

import (
	"fmt"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/s3"
)

func Init(config config.Config, bucket string) error {
	// Get a list of existing buckets
	listBucketsOutput, err := s3.ListBuckets(config)
	if err != nil {
		return fmt.Errorf("Failed to list buckets: %w", err)
	}

	// Return if the desired bucket already exists
	bucketExists := false
	for _, thisBucket := range listBucketsOutput {
		if thisBucket == bucket {
			bucketExists = true
		}
	}

	if !bucketExists {
		// Create a new bucket
		fmt.Printf("Creating new %s bucket...\n", bucket)
		err = s3.CreateBucket(config, bucket)
		if err != nil {
			return err
		}
	}

	// Ensure versioning is enabled, necessary if bucket creation was interrupted
	err = s3.SetVersioning(config, bucket)
	if err != nil {
		return err
	}

	// Initialize bucket state if not already present
	err = initState(config, bucket)
	if err != nil {
		return fmt.Errorf("Failed to initialize state for bucket %s: %w", bucket, err)
	}

	return nil
}

func initState(config config.Config, bucket string) error {
	// Check if the state directory already exists
	stateDir := "state/"
	stateDirExists, err := s3.ObjectExists(config, bucket, stateDir)
	if err != nil {
		return fmt.Errorf("Failed to check whether the state directory exists: %w", err)
	}
	// Create the state directory if it does not already exist
	if !stateDirExists {
		fmt.Printf("Creating new state directory at %s/%s\n", bucket, stateDir)
		err = s3.PutObject(config, bucket, stateDir, []byte{})
		if err != nil {
			return fmt.Errorf("Failed to create state directory at %s/%s: %w", bucket, stateDir, err)
		}
	}

	// Check if the state file already exists
	statePath := "state/state.json"
	statePathExists, err := s3.ObjectExists(config, bucket, statePath)
	if err != nil {
		return fmt.Errorf("Failed to check whether the state file exists: %w", err)
	}
	// Create the initial state file if it does not already exist
	if !statePathExists {
		fmt.Printf("Creating new state file at %s/%s\n", bucket, statePath)
		err = s3.PutObject(config, bucket, statePath, []byte("{}"))
		if err != nil {
			return fmt.Errorf("Failed to create state file in bucket %s: %w", bucket, err)
		}
	}

	return nil
}
