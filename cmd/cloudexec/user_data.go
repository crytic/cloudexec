package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
	"time"

	"github.com/trailofbits/cloudexec/pkg/config"
)

type UserData struct {
	SpacesAccessKey   string
	SpacesSecretKey   string
	SpacesRegion      string
	DigitalOceanToken string
	SetupCommands     string
	RunCommand        string
	Timeout           string
}

//go:embed user_data.sh.tmpl
var userDataTemplate string

func GenerateUserData(config config.Config, lc LaunchConfig) (string, error) {
	// Load the embeded user data template
	tmpl := template.Must(template.New("user_data").Parse(userDataTemplate))

	// turn the time duration string from config into a number of seconds
	timeout, err := time.ParseDuration(lc.Input.Timeout)
	if err != nil {
		return "", fmt.Errorf("Failed to parse timeout of %s: %w", lc.Input.Timeout, err)
	}

	timeoutStr := fmt.Sprintf("%d", int(timeout.Seconds()))

	// Set the values for the template
	data := UserData{
		SpacesAccessKey:   config.DigitalOcean.SpacesAccessKey,
		SpacesSecretKey:   config.DigitalOcean.SpacesSecretKey,
		SpacesRegion:      config.DigitalOcean.SpacesRegion,
		DigitalOceanToken: config.DigitalOcean.ApiKey,
		SetupCommands:     lc.Commands.Setup,
		RunCommand:        lc.Commands.Run,
		Timeout:           timeoutStr,
	}

	// Execute the template script with provided user data
	var script bytes.Buffer
	err = tmpl.Execute(&script, data)
	if err != nil {
		return "", fmt.Errorf("Failed to execute user data script template: %w", err)
	}

	userData := script.String()
	return userData, nil
}
