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
	droplets, err := do.GetAllDroplets(config)
	if err != nil {
		return fmt.Errorf("Failed to get all running servers: %w", err)
	}
	if len(droplets) == 0 {
		fmt.Printf("Zero servers found\n")
		return nil
	}
	fmt.Printf("Found %v running server(s):\n", len(droplets))
	for _, job := range existingState.Jobs {
		if job.Status != state.Provisioning && job.Status != state.Running {
			continue // skip jobs that aren't running
		}
		err = CancelJob(&job, existingState, config)
		if err != nil {
			fmt.Printf("Failed to cancel job %v", job.ID)
		}
	}
	return nil
}

func ResetBucket(config config.Config) error {
	objects, err := s3.ListObjects(config, "")
	if err != nil {
		return fmt.Errorf("Failed to list objects in bucket: %w", err)
	}

	// Confirm bucket deletion
	var numToRm int = len(objects)
	if numToRm == 0 {
		fmt.Printf("Bucket is already empty.\n")
		return nil
	} else {
		fmt.Printf("Removing the first %d items from bucket...\n", numToRm)
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
			fmt.Printf("Deleted %d objects from bucket...\n", numToRm)
		}
	}
	return nil
}
