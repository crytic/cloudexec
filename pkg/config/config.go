package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DigitalOcean struct {
		ApiKey          string `toml:"apiKey"`
		SpacesAccessKey string `toml:"spacesAccessKey"`
		SpacesSecretKey string `toml:"spacesSecretKey"`
		SpacesRegion    string `toml:"spacesRegion"`
	} `toml:"DigitalOcean"`
}

func Create(configValues Config) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "cloudexec")
	configFile := filepath.Join(configDir, "config.toml")

	err := os.MkdirAll(configDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("Failed to create configuration directory at %s: %w", configDir, err)
	}

	file, err := os.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Failed to create configuration file at %s: %w", configFile, err)
	}
	defer file.Close()

	// Write the configuration values to the file
	encoder := toml.NewEncoder(file)
	err = encoder.Encode(configValues)
	if err != nil {
		return fmt.Errorf("Failed to encode configuration values: %w", err)
	}

	fmt.Printf("Configuration file created at: %s\n", configFile)
	return nil
}

func Load(configFilePath string) (Config, error) {
	var config Config

	// Any configuration value can be overridden by environment variables
	// This is useful for CI/CD pipelines
	doApiKey := os.Getenv("DIGITALOCEAN_API_KEY")
	doSpacesAccessKey := os.Getenv("DIGITALOCEAN_SPACES_ACCESS_KEY")
	doSpacesSecretKey := os.Getenv("DIGITALOCEAN_SPACES_SECRET_ACCESS_KEY")
	doSpacesRegion := os.Getenv("DIGITALOCEAN_SPACES_REGION")

	// If all environment variables are set, use them and skip loading the config file
	if doApiKey != "" && doSpacesAccessKey != "" && doSpacesSecretKey != "" && doSpacesRegion != "" {
		config.DigitalOcean.ApiKey = doApiKey
		config.DigitalOcean.SpacesAccessKey = doSpacesAccessKey
		config.DigitalOcean.SpacesSecretKey = doSpacesSecretKey
		config.DigitalOcean.SpacesRegion = doSpacesRegion
		return config, nil
	}

	// Load the configuration file
	configFile, err := os.Open(configFilePath)
	if err != nil {
		return config, fmt.Errorf("Failed to open configuration file at %s: %w", configFilePath, err)
	}
	defer configFile.Close()

	// Decode the configuration file
	decoder := toml.NewDecoder(configFile)
	_, err = decoder.Decode(&config)
	if err != nil {
		return config, fmt.Errorf("Failed to decode configuration file: %w", err)
	}

	// Override config values with environment variables if they are set
	if doApiKey != "" {
		config.DigitalOcean.ApiKey = doApiKey
	}
	if doSpacesAccessKey != "" {
		config.DigitalOcean.SpacesAccessKey = doSpacesAccessKey
	}
	if doSpacesSecretKey != "" {
		config.DigitalOcean.SpacesSecretKey = doSpacesSecretKey
	}
	if doSpacesRegion != "" {
		config.DigitalOcean.SpacesRegion = doSpacesRegion
	}

	return config, nil
}
