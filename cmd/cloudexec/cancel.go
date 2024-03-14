package main

import (
	"fmt"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/crytic/cloudexec/pkg/ssh"
	"github.com/crytic/cloudexec/pkg/state"
)

func ConfirmCancelJob(config config.Config, existingState *state.State, job *state.Job) error {
	if job.Status != state.Provisioning && job.Status != state.Running {
		return fmt.Errorf("Job %v is not running, it is %s", job.ID, job.Status)
	}
	fmt.Printf("Droplet %s associated with job %v: IP=%v | CreatedAt=%s\n", job.Droplet.Name, job.ID, job.Droplet.IP, job.Droplet.Created)
	fmt.Println("Destroy this droplet? (y/n)")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) == "y" {
		fmt.Printf("Destroying droplet %v...\n", job.Droplet.ID)
		err := do.DeleteDroplet(config, job.Droplet.ID)
		if err != nil {
			return fmt.Errorf("Failed to destroy droplet: %w", err)
		}
		fmt.Printf("Removing ssh config for droplet %v...\n", job.Droplet.ID)
		err = ssh.DeleteSSHConfig(job.ID)
		if err != nil {
			return fmt.Errorf("Failed to delete ssh config: %w", err)
		}
		fmt.Printf("Marking job %v as cancelled...\n", job.Droplet.ID)
		err = existingState.CancelRunningJob(config, job.ID)
		if err != nil {
			return fmt.Errorf("Failed to mark job as cancelled: %w", err)
		}
		fmt.Println("Done")
	} else {
		fmt.Printf("Job %v was not cancelled\n", job.ID)
	}
	return nil
}

func ConfirmCancelAll(config config.Config, existingState *state.State) error {
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
		err = ConfirmCancelJob(config, existingState, &job)
		if err != nil {
			fmt.Printf("Failed to cancel job %v", job.ID)
		}
	}
	return nil
}
