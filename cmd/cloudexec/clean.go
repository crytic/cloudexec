package main

import (
	"fmt"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/log"
	"github.com/crytic/cloudexec/pkg/s3"
	"github.com/crytic/cloudexec/pkg/ssh"
	"github.com/crytic/cloudexec/pkg/state"
)

func CleanBucketJob(config config.Config, existingState *state.State, jobID int64, force bool) error {
	prefix := fmt.Sprintf("job-%v", jobID)
	objects, err := s3.ListObjects(config, prefix)
	if err != nil {
		return fmt.Errorf("Failed to list objects in bucket with prefix %s: %w", prefix, err)
	}
	// Confirm job data deletion
	var numToRm int = len(objects)
	if numToRm == 0 {
		log.Info("Bucket is already empty.")
		return nil
	}
	log.Warn("Removing all input, output, logs, and configuration associated with %s", prefix)
	if !force { // Ask for confirmation before cleaning this job if no force flag
		log.Warn("Confirm? (y/n)")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			log.Info("Job %v was not cleaned", jobID)
			return nil
		}
	}
	log.Wait("Deleting bucket contents")
	// Delete all objects in the bucket
	for _, object := range objects {
		log.Info("Deleting object: %s", object)
		err = s3.DeleteObject(config, object)
		if err != nil {
			return err
		}
	}
	log.Good("Bucket is clean")
	newState := &state.State{}
	deleteJob := state.Job{
		ID:     jobID,
		Delete: true,
	}
	newState.CreateJob(deleteJob)
	err = state.MergeAndSave(config, newState)
	log.Good("Removed job %v from state file", jobID)
	if err != nil {
		return fmt.Errorf("Error removing %s from state file: %w", prefix, err)
	}
	err = ssh.DeleteSSHConfig(jobID)
	if err != nil {
		return fmt.Errorf("Failed to delete ssh config: %w", err)
	}
	return nil
}

func CleanBucketAll(config config.Config, existingState *state.State, force bool) error {
	if len(existingState.Jobs) == 0 {
		log.Info("No jobs are available")
		return nil
	}
	for _, job := range existingState.Jobs {
		err := CleanBucketJob(config, existingState, job.ID, force)
		if err != nil {
			return err
		}
	}
	return nil
}
