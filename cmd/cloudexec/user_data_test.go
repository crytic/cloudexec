package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/crytic/cloudexec/pkg/config"
)

func getLaunchConfig(duration string) LaunchConfig {
	launchConfig := LaunchConfig{
		Commands: Commands{
			Setup: "echo 'lets go'",
			Run:   "echo 'lets run'",
		},
		Input: struct {
			Directory string
			Timeout   string
		}{
			Directory: "./input",
			Timeout:   duration,
		},
	}
	return launchConfig
}

func TestUserDataGeneration(t *testing.T) {
	config := config.Config{
		DigitalOcean: struct {
			ApiKey          string `toml:"apiKey"`
			SpacesAccessKey string `toml:"spacesAccessKey"`
			SpacesSecretKey string `toml:"spacesSecretKey"`
			SpacesRegion    string `toml:"spacesRegion"`
		}{
			ApiKey:          "dop_v1_abc123",
			SpacesAccessKey: "abc123",
			SpacesSecretKey: "abc123",
			SpacesRegion:    "abc3",
		},
	}

	var testTable = []struct {
		name           string
		durationString string
		secondsString  string
	}{
		{"It should parse >60 seconds", "123s", "123"},
		{"It should parse minutes", "3m", "180"},
		{"It should parse a combination of time units", "1h2m3s", "3723"},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {

			launchConfig := getLaunchConfig(tt.durationString)

			result, err := GenerateUserData(config, launchConfig)
			if err != nil {
				t.Errorf("Failed to generate user data: %v", err)
			}

			substring := fmt.Sprintf("export TIMEOUT=\"%v\"", tt.secondsString)
			if !strings.Contains(result, substring) {
				t.Errorf("Expected result to contain substring %q, but it did not", substring)
			}

		})
	}

}
