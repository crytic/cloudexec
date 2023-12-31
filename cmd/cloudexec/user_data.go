package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/crytic/cloudexec/pkg/config"
)

type UserData struct {
	SpacesAccessKey   string
	SpacesSecretKey   string
	SpacesRegion      string
	DigitalOceanToken string
	SetupCommands     string
	RunCommand        string
	Timeout           string
	InputDirectory    string
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
	// double quotes are escaped so the command strings can be safely contained by double quotes in bash
	data := UserData{
		SpacesAccessKey:   config.DigitalOcean.SpacesAccessKey,
		SpacesSecretKey:   config.DigitalOcean.SpacesSecretKey,
		SpacesRegion:      config.DigitalOcean.SpacesRegion,
		DigitalOceanToken: config.DigitalOcean.ApiKey,
		SetupCommands:     strings.ReplaceAll(lc.Commands.Setup, `"`, `\"`),
		RunCommand:        strings.ReplaceAll(lc.Commands.Run, `"`, `\"`),
		Timeout:           timeoutStr,
		InputDirectory:    lc.Input.Directory,
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
