package main

import (
	"fmt"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/crytic/cloudexec/pkg/log"
	"github.com/crytic/cloudexec/pkg/state"
)

func CancelJob(config config.Config, existingState *state.State, job *state.Job, force bool) error {
	if job.Status != state.Provisioning && job.Status != state.Running {
		log.Info("Job %v is not running, it is %s", job.ID, job.Status)
    return nil
	}
	log.Info("Destroying droplet %s associated with job %v: IP=%v | CreatedAt=%s", job.Droplet.Name, job.ID, job.Droplet.IP, job.Droplet.Created)
	if !force { // Ask for confirmation before cleaning this job if no force flag
		log.Warn("Confirm? (y/n)")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			log.Info("Droplet %s was not destroyed", job.Droplet.Name)
			return nil
		}
	}
	err := do.DeleteDroplet(config, job.Droplet.ID)
	if err != nil {
		return fmt.Errorf("Failed to destroy droplet: %w", err)
	}
	log.Good("Droplet %v destroyed", job.Droplet.ID)
	err = existingState.CancelRunningJob(config, job.ID)
	if err != nil {
		return fmt.Errorf("Failed to change job status to cancelled: %w", err)
	}
	log.Good("Job %v status changed to cancelled", job.Droplet.ID)
	return nil
}

func CancelAll(config config.Config, existingState *state.State, force bool) error {
	droplets, err := do.GetAllDroplets(config)
	if err != nil {
		return fmt.Errorf("Failed to get all running servers: %w", err)
	}
	if len(droplets) == 0 {
		log.Info("No running servers found")
		return nil
	}
	log.Info("Found %v running server(s):", len(droplets))
	for _, job := range existingState.Jobs {
		if job.Status != state.Provisioning && job.Status != state.Running {
			continue // skip jobs that aren't running
		}
		err = CancelJob(config, existingState, &job, force)
		if err != nil {
			log.Warn("Failed to cancel job %v", job.ID)
		}
	}
	return nil
}
