package ssh

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/kevinburke/ssh_config"
	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/ssh"

	"github.com/crytic/cloudexec/pkg/log"
)

const HostConfigTemplate = `
# Added by cloudexec
Host cloudexec-{{.JobID}}
  HostName {{.IPAddress}}
  User root
  IdentityFile {{.IdentityFile}}
  IdentitiesOnly yes
  ForwardAgent yes
  StrictHostKeyChecking no
  UserKnownHostsFile=/dev/null
  GlobalKnownHostsFile=/dev/null
  Port 22
`

type HostConfig struct {
	JobID        int64
	IPAddress    string
	IdentityFile string
}

func getSSHDir() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("Failed to get current user: %w", err)
	}
	return filepath.Join(user.HomeDir, ".ssh"), nil
}

// Verify that the $HOME/.ssh/config file imports everything from the config.d dir
// We'll add/remove config for each server by adding/removing files in this dir
func EnsureSSHIncludeConfig() error {
	commentString := "# Added by cloudexec\n"
	includeString := "Include config.d/*\n"
	sshDir, err := getSSHDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(sshDir, "config")
	// Create the SSH directory if it does not exist
	err = os.MkdirAll(sshDir, 0700)
	if err != nil {
		return fmt.Errorf("Failed to create SSH directory: %w", err)
	}
	var configFileContent string
	// Check if the config file exists
	if _, err = os.Stat(configPath); os.IsNotExist(err) {
		// If the config file does not exist, create it with the includeString line
		configFileContent = commentString + includeString
	} else {
		// If the config file exists, read its content
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("Failed to read main SSH config file: %w", err)
		}
		configFileContent = string(content)
		// Check if the includeString line is present
		if !strings.Contains(configFileContent, includeString) {
			// If not present, add the line to the top of the file
			configFileContent = commentString + includeString + configFileContent
		} else {
			// If the line is already present, no further action is required
			return nil
		}
	}
	// Write the updated content to the config file
	err = os.WriteFile(configPath, []byte(configFileContent), 0600)
	if err != nil {
		return fmt.Errorf("Failed to write main SSH config file: %w", err)
	}
	return nil
}

// Creates a new keypair that will be used to auth with all servers
func GetOrCreateSSHKeyPair() (string, error) {
	err := EnsureSSHIncludeConfig()
	if err != nil {
		return "", fmt.Errorf("Failed to validate SSH config: %w", err)
	}
	sshDir, err := getSSHDir()
	if err != nil {
		return "", err
	}

	privateKeyPath := filepath.Join(sshDir, "cloudexec-key")
	publicKeyPath := filepath.Join(sshDir, "cloudexec-key.pub")

	// Check if the key pair already exists
	if _, err := os.Stat(privateKeyPath); err == nil {
		// If the key pair exists, read and return the public key
		publicKeyBytes, err := os.ReadFile(publicKeyPath)
		if err != nil {
			return "", fmt.Errorf("Failed to read SSH public key file: %w", err)
		}
		return string(publicKeyBytes), nil
	}
	log.Wait("Creating new ssh keypair")

	// Generate an ed25519 key pair
	edPubKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("Failed to generate SSH ed25519 key pair: %w", err)
	}

	// Encode and save the private key
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: edkey.MarshalED25519PrivateKey(privateKey),
	})

	err = os.WriteFile(privateKeyPath, privateKeyPEM, 0600)
	if err != nil {
		return "", fmt.Errorf("Failed to save SSH private key file: %w", err)
	}

	// Convert the ed25519.PublicKey to ssh.PublicKey
	publicKey, err := ssh.NewPublicKey(edPubKey)
	if err != nil {
		return "", fmt.Errorf("Failed to create SSH public key: %w", err)
	}

	// Save the public key
	publicKeySSHFormat := ssh.MarshalAuthorizedKey(publicKey)
	err = os.WriteFile(publicKeyPath, publicKeySSHFormat, 0644)
	if err != nil {
		return "", fmt.Errorf("Failed to save SSH public key file: %w", err)
	}

	log.Good("Created new ssh keypair at %s(.pub)", privateKeyPath)
	return string(publicKeySSHFormat), nil
}

func AddSSHConfig(jobID int64, ipAddress string) error {
	err := EnsureSSHIncludeConfig()
	if err != nil {
		return fmt.Errorf("Failed to validate main SSH config: %w", err)
	}
	sshDir, err := getSSHDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(sshDir, "config.d")
	hostname := fmt.Sprintf("cloudexec-%v", jobID)
	configPath := filepath.Join(configDir, hostname)
	identityFile := filepath.Join(sshDir, "cloudexec-key")
	// Create the SSH config directory if it does not exist
	err = os.MkdirAll(configDir, 0700)
	if err != nil {
		return fmt.Errorf("Failed to create SSH config directory: %w", err)
	}
	// If the config file does not exist, create it
	configFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("Failed to create SSH config file: %w", err)
	}
	defer configFile.Close()
	// Write the templated host config to file
	config := HostConfig{
		JobID:        jobID,
		IPAddress:    ipAddress,
		IdentityFile: identityFile,
	}
	tmpl, err := template.New("hostConfig").Parse(HostConfigTemplate)
	if err != nil {
		return fmt.Errorf("Failed to parse SSH config template: %w", err)
	}
	// Execute the template and write to the file
	err = tmpl.Execute(configFile, config)
	if err != nil {
		return fmt.Errorf("Failed to write SSH config to file: %w", err)
	}
	return nil
}

func DeleteSSHConfig(jobID int64) error {
	err := EnsureSSHIncludeConfig()
	if err != nil {
		return fmt.Errorf("Failed to validate SSH config: %w", err)
	}
	sshDir, err := getSSHDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(sshDir, "config.d")
	hostname := fmt.Sprintf("cloudexec-%v", jobID)
	configPath := filepath.Join(configDir, hostname)
	err = os.Remove(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to remove config file from config.d: %w", err)
	}
	// If there's no error, the file was deleted successfully
	if err == nil {
		log.Good("Deleted SSH config for cloudexec-%v", jobID)
	}
	return nil
}

func WaitForSSHConnection(jobID int64) error {
	sshDir, err := getSSHDir()
	if err != nil {
		return err
	}
	hostname := fmt.Sprintf("cloudexec-%v", jobID)
	hostname = strings.ReplaceAll(hostname, ".", "-")
	sshConfigPath := filepath.Join(sshDir, "config.d", hostname)

	timeout := 60 * time.Second
	retryInterval := 10 * time.Second

	sshConfigBytes, err := os.ReadFile(sshConfigPath)
	if err != nil {
		return fmt.Errorf("Failed to read SSH config file: %w", err)
	}
	// Decode the SSH config file into an io reader
	sshConfig := bytes.NewReader(sshConfigBytes)
	cfg, err := ssh_config.Decode(sshConfig)
	if err != nil {
		return fmt.Errorf("Failed to load SSH config: %w", err)
	}
	ipAddress, _ := cfg.Get(hostname, "HostName")
	port, _ := cfg.Get(hostname, "Port")
	user, _ := cfg.Get(hostname, "User")
	identityFile, _ := cfg.Get(hostname, "IdentityFile")

	// Encode the identity file to bytes for use with the SSH client

	identityFileBytes, err := os.ReadFile(identityFile)
	if err != nil {
		return fmt.Errorf("Failed to read identity file: %w", err)
	}

	// Parse the identity file bytes into an ssh.Signer
	signer, err := ssh.ParsePrivateKey(identityFileBytes)
	if err != nil {
		return fmt.Errorf("Failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         retryInterval,
	}

	start := time.Now()
	for {
		conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", ipAddress, port), config)
		if err == nil {
			conn.Close()
			return nil
		}

		elapsed := time.Since(start)
		if elapsed >= timeout {
			return fmt.Errorf("Timed out waiting for SSH connection: %w", err)
		}

		time.Sleep(retryInterval)
	}
}

func StreamLogs(jobID int64) error {
	hostname := fmt.Sprintf("cloudexec-%v", jobID)
	// Stream the logs from the server with tail -f
	sshCmd := exec.Command("ssh", hostname, "tail", "-f", "/var/log/cloud-init-output.log")
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	err := sshCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to stream logs: %w", err)
	}
	return nil
}

func AttachToTmuxSession(jobID int64) error {
	hostname := fmt.Sprintf("cloudexec-%v", jobID)
	sshCmd := exec.Command("ssh", "-t", hostname, "tmux", "attach-session", "-t", "cloudexec")

	// Connect the SSH command to the current terminal
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	// Run the SSH command
	err := sshCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to attach to tmux session: %w", err)
	}
	return nil
}
