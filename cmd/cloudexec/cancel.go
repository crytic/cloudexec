package main

import (
	"fmt"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/crytic/cloudexec/pkg/state"
)

func CancelJob(config config.Config, existingState *state.State, job *state.Job, force bool) error {
	if job.Status != state.Provisioning && job.Status != state.Running {
		return fmt.Errorf("Job %v is not running, it is %s", job.ID, job.Status)
	}
	fmt.Printf("Destroying droplet %s associated with job %v: IP=%v | CreatedAt=%s\n", job.Droplet.Name, job.ID, job.Droplet.IP, job.Droplet.Created)
	if !force { // Ask for confirmation before cleaning this job if no force flag
		fmt.Println("Confirm? (y/n)")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Printf("Droplet %s was not destroyed\n", job.Droplet.Name)
			return nil
		}
	}
	fmt.Printf("Destroying droplet %v...\n", job.Droplet.ID)
	err := do.DeleteDroplet(config, job.Droplet.ID)
	if err != nil {
		return fmt.Errorf("Failed to destroy droplet: %w", err)
	}
	fmt.Printf("Marking job %v as cancelled...\n", job.Droplet.ID)
	err = existingState.CancelRunningJob(config, job.ID)
	if err != nil {
		return fmt.Errorf("Failed to mark job as cancelled: %w", err)
	}
	return nil
}

func CancelAll(config config.Config, existingState *state.State, force bool) error {
	droplets, err := do.GetAllDroplets(config)
	if err != nil {
		return fmt.Errorf("Failed to get all running servers: %w", err)
	}
	if len(droplets) == 0 {
		fmt.Println("No running servers found")
		return nil
	}
	fmt.Printf("Found %v running server(s):\n", len(droplets))
	for _, job := range existingState.Jobs {
		if job.Status != state.Provisioning && job.Status != state.Running {
			continue // skip jobs that aren't running
		}
		err = CancelJob(config, existingState, &job, force)
		if err != nil {
			fmt.Printf("Failed to cancel job %v", job.ID)
		}
	}
	return nil
}
