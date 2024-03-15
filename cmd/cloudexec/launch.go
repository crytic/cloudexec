package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/crytic/cloudexec/pkg/config"
	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/crytic/cloudexec/pkg/log"
	"github.com/crytic/cloudexec/pkg/ssh"
	"github.com/crytic/cloudexec/pkg/state"
)

type Commands struct {
	Setup string `toml:"setup"`
	Run   string `toml:"run"`
}

type LaunchConfig struct {
	Commands Commands `toml:"commands"`
	Input    struct {
		JobName   string
		Directory string
		Timeout   string
	} `toml:"input"`
}

func InitLaunchConfig() error {
	// Create a new launch config file in the current directory
	launchConfigPath := filepath.Join(".", "cloudexec.toml")
	if _, err := os.Stat(launchConfigPath); err == nil {
		return fmt.Errorf("File %v already exists", launchConfigPath)
	}

	// Create a new launch config file
	launchConfigFile, err := os.Create(launchConfigPath)
	if err != nil {
		return fmt.Errorf("Failed to create launch config file: %w", err)
	}
	defer launchConfigFile.Close()

	// Write the default launch config to the file
	_, err = launchConfigFile.WriteString(`
# Set the directory to upload to the droplet.
[input]
directory = ""
timeout = "48h"

[commands]
setup = '''
# Install dependencies here.
# This string is interpreted as a multi-line bash script
# see cloudexec/example/cloudexec.toml for example usage

'''

# This command is run from the input directory
# after the setup script completes.
run = ""
`)

	if err != nil {
		return fmt.Errorf("Failed to write launch config file: %w", err)
	}

	return nil
}

func LoadLaunchConfig(launchConfigPath string) (LaunchConfig, error) {
	var lc LaunchConfig

	tomlData, err := os.ReadFile(launchConfigPath)
	if err != nil {
		return lc, fmt.Errorf("Failed to read launch config file at %s: %w", launchConfigPath, err)
	}

	if _, err := toml.Decode(string(tomlData), &lc); err != nil {
		return lc, fmt.Errorf("Failed to decode launch config file at %s: %w", launchConfigPath, err)
	}

	return lc, nil
}

func Launch(config config.Config, dropletSize string, dropletRegion string, lc LaunchConfig) error {
	// get existing state
	existingState, err := state.GetState(config)
	if err != nil {
		return fmt.Errorf("Failed to get S3 state: %w", err)
	}
	// get the latest job
	latestJob := existingState.GetLatestJob()
	var latestJobId int64
	if latestJob == nil {
		latestJobId = 0
	} else {
		latestJobId = latestJob.ID
	}
	thisJobId := latestJobId + 1

	// update state struct with a new job
	newState := &state.State{}
	startedAt := time.Now().Unix()

	newJob := state.Job{
		Name:      lc.Input.JobName,
		ID:        thisJobId,
		Status:    state.Provisioning,
		StartedAt: startedAt,
	}
	newState.CreateJob(newJob)
	// sync state to bucket
	err = state.MergeAndSave(config, newState)
	log.Info("Registered new job with id %v", thisJobId)
	if err != nil {
		return fmt.Errorf("Failed to update S3 state: %w", err)
	}

	// upload local files to the bucket
	sourcePath := lc.Input.Directory // TODO: verify that this path exists & throw informative error if not
	destPath := fmt.Sprintf("job-%v", thisJobId)
	err = UploadDirectoryToSpaces(config, sourcePath, destPath)
	if err != nil {
		return fmt.Errorf("Failed to upload files: %w", err)
	}

	// Get or create an SSH key
	publicKey, err := ssh.GetOrCreateSSHKeyPair()
	if err != nil {
		return fmt.Errorf("Failed to get or creating SSH key pair: %w", err)
	}

	// Prepare user data
	userData, err := GenerateUserData(config, lc)
	if err != nil {
		return fmt.Errorf("Failed to generate user data: %w", err)
	}

	log.Wait("Creating new %s droplet in %s for job %d", dropletSize, config.DigitalOcean.SpacesRegion, thisJobId)
	droplet, err := do.CreateDroplet(config, config.DigitalOcean.SpacesRegion, dropletSize, userData, thisJobId, publicKey)
	if err != nil {
		return fmt.Errorf("Failed to create droplet: %w", err)
	}
	log.Good("Droplet created with IP: %v", droplet.IP)

	// Add the droplet info to state
	updatedAt := time.Now().Unix()
	for i, job := range newState.Jobs {
		if job.ID == thisJobId {
			newState.Jobs[i].Droplet = droplet
			newState.Jobs[i].UpdatedAt = updatedAt
		}
	}
	err = state.MergeAndSave(config, newState)
	if err != nil {
		return fmt.Errorf("Failed to update S3 state: %w", err)
	}
	log.Info("Saved new droplet info to state")

	// Add the droplet to the SSH config file
	err = ssh.AddSSHConfig(thisJobId, droplet.IP)
	if err != nil {
		return fmt.Errorf("Failed to add droplet to SSH config file: %w", err)
	}
	log.Info("Saved droplet info to SSH config")

	// Ensure we can SSH into the droplet
	log.Wait("Waiting until our new droplet wakes up")
	err = ssh.WaitForSSHConnection(thisJobId)
	if err != nil {
		return fmt.Errorf("Failed to SSH into the droplet: %w", err)
	}
	log.Good("SSH connection established!")
	log.Good("Launch complete")
	log.Info("You can now attach to the running job with: cloudexec attach")
	log.Info("Stream logs from the droplet with: cloudexec logs")
	log.Info("SSH to your droplet with: ssh cloudexec-%v", thisJobId)

	return nil
}
