package main

import (
	"fmt"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/s3"
)

func Init(username string, config config.Config) error {
	bucketName := fmt.Sprintf("cloudexec-%s-trailofbits", username)
	// Create a new bucket (or get an existing one)
	err := s3.GetOrCreateBucket(config, username)
	if err != nil {
		return fmt.Errorf("Failed to get bucket for %s: %w", username, err)
	}
	fmt.Printf("Using bucket: %v\n", bucketName)

	// Create the state directory
	err = s3.PutObject(config, bucketName, "state/", []byte{})
	if err != nil {
		return fmt.Errorf("Failed to create state directory in bucket %s: %w", bucketName, err)
	}

	// Create the initial state file
	err = s3.PutObject(config, bucketName, "state/state.json", []byte("{}"))
	if err != nil {
		return fmt.Errorf("Failed to create state file in bucket %s: %w", bucketName, err)
	}

	return nil
}
