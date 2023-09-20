package main

import (
	"fmt"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/crytic/cloudexec/pkg/s3"
)

func ConfirmDeleteDroplets(config config.Config, userName string, instanceToJobs map[int64][]int64) error {
	dropletName := fmt.Sprintf("cloudexec-%v", userName)
	instances, err := do.GetDropletsByName(config, dropletName)
	if err != nil {
		return fmt.Errorf("Failed to get droplet by name: %w", err)
	}
	if len(instances) > 0 {
		// TODO: support multiple droplets
		fmt.Println("existing instance(s) found:")
		for _, instance := range instances {
			// get a pretty string describing the jobs associated with this instance
			if instanceToJobs == nil {
				return fmt.Errorf("Given instanceToJobs argument must not be nil")
			}
			jobs := instanceToJobs[int64(instance.ID)]
			var prettyJobs string
			if len(jobs) == 0 {
				prettyJobs = "none"
			} else {
				jobStrings := make([]string, len(jobs))
				for i, job := range jobs {
					jobStrings[i] = fmt.Sprint(job)
				}
				prettyJobs = strings.Join(jobStrings, ", ")
			}

			fmt.Printf("  - %v (IP: %v) (Jobs: %s) created at %v\n", instance.Name, instance.IP, prettyJobs, instance.Created)
			fmt.Println("destroy this droplet? (y/n)")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) == "y" {
				fmt.Printf("Destroying droplet %v...\n", instance.ID)
				err = do.DeleteDroplet(config, instance.ID)
				if err != nil {
					return fmt.Errorf("Failed to destroy droplet: %w", err)
				}
			}
		}
	} else {
		fmt.Println("zero instances found")
	}
	return nil
}

func ResetBucket(config config.Config, bucketName string, spacesAccessKey string, spacesSecretKey string, spacesRegion string) error {
	objects, err := s3.ListObjects(config, bucketName, "")
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
				err = s3.DeleteObject(config, bucketName, object)
				if err != nil {
					return err
				}
			}
			fmt.Printf("Deleted %d objects in bucket '%s'...\n", numToRm, bucketName)
		}
	}
	return nil
}
