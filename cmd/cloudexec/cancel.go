package main

import (
	"fmt"
	"strings"

	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/crytic/cloudexec/pkg/s3"
	"github.com/crytic/cloudexec/pkg/ssh"
	"github.com/crytic/cloudexec/pkg/state"
)

func CancelJob(job state.Job) error {
	if job.Status != Provisioning && job.Status != Running {
		return fmt.Errorf("Job %s is not running, it is %s", job.ID, job.Status)
	}

	fmt.Printf("Droplet %s associated with job %s: IP=%v | CreatedAt=%v\n", job.droplet.Name, jobId, job.droplet.IP, job.droplet.Created)
	fmt.Println("Destroy this droplet? (y/n)")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) == "y" {
		fmt.Printf("Destroying droplet %v...\n", job.droplet.ID)
		err = do.DeleteDroplet(config, job.droplet.ID)
		if err != nil {
			return fmt.Errorf("Failed to destroy droplet: %w", err)
		}
		fmt.Printf("Removing ssh config for droplet %v...\n", job.droplet.ID)
		err = ssh.DeleteSSHConfig(job.ID)
		if err != nil {
			return fmt.Errorf("Failed to delete ssh config: %w", err)
		}
		fmt.Printf("Marking job %v as cancelled...\n", job.droplet.ID)
		err = existingState.CancelRunningJobs(config, slug)
		if err != nil {
			return fmt.Errorf("Failed to mark job as cancelled: %w", err)
		}
		fmt.Println("Done")
	} else {
		fmt.Printf("Job %v was not cancelled\n", jobId)
	}
	return nil
}
