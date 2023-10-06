package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	"golang.org/x/term"
)

func Configure() error {
	spacesRegion, err := promptUserInput("Digital Ocean Spaces region", "nyc3")
	if err != nil {
		return err
	}
	apiKey, err := promptSecretInput("Digital Ocean API key or reference", "op://private/DigitalOcean/api token")
	if err != nil {
		return err
	}
	spacesAccessKey, err := promptSecretInput("Digital Ocean Spaces access key ID or reference", "op://private/DigitalOcean/spaces access key id")
	if err != nil {
		return err
	}
	spacesSecretKey, err := promptSecretInput("Digital Ocean Spaces secret access key or reference", "op://private/DigitalOcean/spaces secret access key")
	if err != nil {
		return err
	}

	configValues := config.Config{
		DigitalOcean: struct {
			ApiKey          string `toml:"apiKey"`
			SpacesAccessKey string `toml:"spacesAccessKey"`
			SpacesSecretKey string `toml:"spacesSecretKey"`
			SpacesRegion    string `toml:"spacesRegion"`
		}{
			ApiKey:          apiKey,
			SpacesAccessKey: spacesAccessKey,
			SpacesSecretKey: spacesSecretKey,
			SpacesRegion:    spacesRegion,
		},
	}

	err = config.Create(configValues)
	if err != nil {
		return fmt.Errorf("failed to create configuration: %w", err)
	}
	return nil
}

func promptSecretInput(prompt, defaultValue string) (string, error) {
	fmt.Printf("%s [%s]: ", prompt, defaultValue)
	rawInput, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("Failed to read input: %w", err)
	}
	fmt.Println()

	input := strings.TrimSpace(string(rawInput))

	if input == "" {
		return defaultValue, nil
	}

	return input, nil
}

func promptUserInput(prompt, defaultValue string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [%s]: ", prompt, defaultValue)

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("Failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue, nil
	}

	return input, nil
}

func LoadConfig(configFilePath string) (config.Config, error) {
	config, err := config.Load(configFilePath)
	if err != nil {
		return config, err
	}

	value, err := processOpValue(config.DigitalOcean.ApiKey)
	if err != nil {
		return config, err
	}
	config.DigitalOcean.ApiKey = value

	value, err = processOpValue(config.DigitalOcean.SpacesAccessKey)
	if err != nil {
		return config, err
	}
	config.DigitalOcean.SpacesAccessKey = value

	value, err = processOpValue(config.DigitalOcean.SpacesSecretKey)
	if err != nil {
		return config, err
	}
	config.DigitalOcean.SpacesSecretKey = value

	return config, nil
}

func processOpValue(value string) (string, error) {
	if strings.HasPrefix(value, "op://") {
		// Process the value using `op`
		var stderr bytes.Buffer
		cmd := exec.Command("op", "read", "--no-newline", value)
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			// err says "exit status 1" so not very helpful, omit it from the following message
			return "", fmt.Errorf("Failed to process 1password reference for %s: %s", value, stderr.String())
		}
		return strings.TrimSpace(string(output)), nil
	}
	return value, nil
}
