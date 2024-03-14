package main

import (
	"fmt"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/s3"
	"github.com/crytic/cloudexec/pkg/state"
)

func CleanBucketJob(config config.Config, existingState *state.State, jobID int64) error {
	prefix := fmt.Sprintf("job-%v", jobID)
	objects, err := s3.ListObjects(config, prefix)
	if err != nil {
		return fmt.Errorf("Failed to list objects in bucket with prefix %s: %w", prefix, err)
	}
	// Confirm job data deletion
	var numToRm int = len(objects)
	if numToRm == 0 {
		fmt.Printf("Bucket is already empty.\n")
		return nil
	} else {
		fmt.Printf("Removing ALL input, output, and logs associated with %s...\n", prefix)
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
			existingState.DeleteJob(jobID)
			err = state.UpdateState(config, existingState)
			if err != nil {
				return fmt.Errorf("Error removing %s from state file: %w\n", prefix, err)
			}
		}
	}
	return nil
}

func CleanBucketAll(config config.Config, existingState *state.State) error {
	for _, job := range existingState.Jobs {
		err := CleanBucketJob(config, existingState, job.ID)
		if err != nil {
			return err
		}
	}
	return nil
}
