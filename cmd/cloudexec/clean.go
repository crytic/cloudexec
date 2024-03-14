package main

import (
	"fmt"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/crytic/cloudexec/pkg/s3"
	"github.com/crytic/cloudexec/pkg/state"
)

func ConfirmCancelAll(config config.Config, existingState *state.State) error {
	bucketName := fmt.Sprintf("cloudexec-%s", config.Username)
	droplets, err := do.GetAllDroplets(config)
	if err != nil {
		return fmt.Errorf("Failed to get droplets by name: %w", err)
	}
	if len(droplets) == 0 {
		fmt.Printf("Zero %s droplets found\n", bucketName)
		return nil
	}
	fmt.Printf("Existing %s droplet(s) found:\n", bucketName)
	for _, job := range existingState.Jobs {
		err = CancelJob(&job, existingState, config)
		if err != nil {
			fmt.Printf("Failed to cancel job %v", job.ID)
		}
	}
	return nil
}

func ResetBucket(config config.Config) error {
	bucketName := fmt.Sprintf("cloudexec-%s", config.Username)
	objects, err := s3.ListObjects(config, "")
	if err != nil {
		return fmt.Errorf("Failed to list objects in bucket '%s': %w", bucketName, err)
	}

	// Confirm bucket deletion
	var numToRm int = len(objects)
	if numToRm == 0 {
		fmt.Printf("Bucket '%s' is already empty.\n", bucketName)
		return nil
	} else {
		fmt.Printf("Removing the first %d items from bucket %s...\n", numToRm, bucketName)
		fmt.Println("Confirm? (y/n)")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "y" {
			fmt.Printf("Deleting bucket contents...\n")
			// Delete all objects in the bucket
			for _, object := range objects {
				fmt.Println("Deleting object: ", object)
				err = s3.DeleteObject(config, object)
				if err != nil {
					return err
				}
			}
			fmt.Printf("Deleted %d objects in bucket '%s'...\n", numToRm, bucketName)
		}
	}
	return nil
}
