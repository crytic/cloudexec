package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/crytic/cloudexec/pkg/config"
	do "github.com/crytic/cloudexec/pkg/digitalocean"
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

func Launch(user *user.User, config config.Config, dropletSize string, dropletRegion string, lc LaunchConfig) error {
	username := user.Username
	bucketName := fmt.Sprintf("cloudexec-%s", username)

	// get existing state from bucket
	fmt.Printf("Getting existing state from bucket %s...\n", bucketName)
	existingState, err := state.GetState(config, bucketName)
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

	newJob := state.JobInfo{
		ID:        thisJobId,
		Status:    state.Provisioning,
		StartedAt: startedAt,
	}
	newState.CreateJob(newJob)
	// sync state to bucket
	fmt.Printf("Updating state in bucket %s...\n", bucketName)
	err = state.UpdateState(config, bucketName, newState)
	if err != nil {
		return fmt.Errorf("Failed to update S3 state: %w", err)
	}

	// upload local files to the bucket
	sourcePath := lc.Input.Directory // TODO: verify that this path exists & throw informative error if not
	destPath := fmt.Sprintf("job-%v", thisJobId)
	fmt.Printf("Compressing and uploading contents of directory %s to bucket %s/%s...\n", sourcePath, bucketName, destPath)
	err = UploadDirectoryToSpaces(config, bucketName, sourcePath, destPath)
	if err != nil {
		return fmt.Errorf("Failed to upload files: %w", err)
	}

	// Get or create an SSH key
	fmt.Println("Getting or creating SSH key pair...")
	publicKey, err := ssh.GetOrCreateSSHKeyPair(user)
	if err != nil {
		return fmt.Errorf("Failed to get or creating SSH key pair: %w", err)
	}

	// Prepare user data
	fmt.Println("Generating user data...")
	userData, err := GenerateUserData(config, lc)
	if err != nil {
		return fmt.Errorf("Failed to generate user data: %w", err)
	}

	fmt.Printf("Creating new %s droplet in %s for job %d...\n", dropletSize, config.DigitalOcean.SpacesRegion, thisJobId)
	droplet, err := do.CreateDroplet(config, username, config.DigitalOcean.SpacesRegion, dropletSize, userData, thisJobId, publicKey)
	if err != nil {
		return fmt.Errorf("Failed to create droplet: %w", err)
	}

	fmt.Printf("Droplet created with IP: %v\n", droplet.IP)

	// Add the droplet info to state
	fmt.Println("Adding new droplet info to state...")
	updatedAt := time.Now().Unix()
	for i, job := range newState.Jobs {
		if job.ID == thisJobId {
			newState.Jobs[i].Droplet = droplet
			newState.Jobs[i].UpdatedAt = updatedAt
		}
	}
	fmt.Printf("Uploading new state to %s\n", bucketName)
	err = state.UpdateState(config, bucketName, newState)
	if err != nil {
		return fmt.Errorf("Failed to update S3 state: %w", err)
	}

	// Add the droplet to the SSH config file
	// TODO: improve this for multi droplet support
	fmt.Println("Deleting old cloudexec instance from SSH config file...")
	err = ssh.DeleteSSHConfig(user, "cloudexec")
	if err != nil {
		return fmt.Errorf("Failed to delete old cloudexec entry from SSH config file: %w", err)
	}
	fmt.Println("Adding droplet to SSH config file...")
	err = ssh.AddSSHConfig(user, droplet.IP)
	if err != nil {
		return fmt.Errorf("Failed to add droplet to SSH config file: %w", err)
	}

	// Ensure we can SSH into the droplet
	fmt.Println("Ensuring we can SSH into the droplet...")
	// TODO: improve this for multi droplet support
	// sshConfigName := fmt.Sprintf("cloudexec-%v", dropletIp)
	sshConfigName := "cloudexec"
	sshConfigName = strings.ReplaceAll(sshConfigName, ".", "-")
	sshConfigPath := filepath.Join(user.HomeDir, ".ssh", "config.d", sshConfigName)
	err = ssh.WaitForSSHConnection(sshConfigPath)
	if err != nil {
		return fmt.Errorf("Failed to SSH into the droplet: %w", err)
	}
	fmt.Println("SSH connection established!")
	fmt.Println("Launch complete")
	fmt.Println("You can now attach to the running job with: cloudexec attach")
	fmt.Println("Stream logs from the droplet with: cloudexec logs")
	fmt.Println("SSH to your droplet with: ssh cloudexec")

	return nil
}
