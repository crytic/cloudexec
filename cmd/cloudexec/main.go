package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/crytic/cloudexec/pkg/ssh"
	"github.com/crytic/cloudexec/pkg/state"
	"github.com/urfave/cli/v2"
)

var (
	Version              = "dev"
	Commit               = "none"
	Date                 = "unknown"
	ConfigFilePath       = fmt.Sprintf("%s/.config/cloudexec/config.toml", os.Getenv("HOME"))
	LaunchConfigFilePath = "./cloudexec.toml"
)

func main() {
	// Attempt to load the configuration
	config, configErr := LoadConfig(ConfigFilePath)

	app := &cli.App{
		Name:  "cloudexec",
		Usage: "easily run cloud based jobs",
		Commands: []*cli.Command{

			{
				Name:    "version",
				Usage:   "Gets the version of the app",
				Aliases: []string{"v"},
				Action: func(*cli.Context) error {
					fmt.Printf("cloudexec %s, commit %s, built at %s", Version, Commit, Date)
					return nil
				},
			},

			{
				Name:  "configure",
				Usage: "Configure credentials",
				Action: func(*cli.Context) error {
					err := Configure()
					if err != nil {
						return err
					}
					return nil
				},
			},

			{
				Name:  "init",
				Usage: "Create a new cloudexec.toml launch configuration in the current directory",
				Action: func(c *cli.Context) error {
					err := InitLaunchConfig()
					if err != nil {
						return err
					}
					return nil
				},
			},

			{
				Name:    "check",
				Usage:   "Verifies cloud authentication",
				Aliases: []string{"c"},
				Action: func(*cli.Context) error {
					// Abort on configuration error
					if configErr != nil {
						return configErr
					}
					resp, err := do.CheckAuth(config)
					if err != nil {
						return err
					}
					fmt.Println(resp)
					snap, err := do.GetLatestSnapshot(config)
					if err != nil {
						return err
					}
					fmt.Printf("Using CloudExec image: %s\n", snap.Name)
					return nil
				},
			},

			{
				Name:    "launch",
				Usage:   "Launch a droplet and start a job",
				Aliases: []string{"l"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config",
						Usage: "cloudexec.toml file path",
					},
					&cli.StringFlag{
						Name:  "size",
						Value: "c-2", // Default droplet size
						Usage: "Optional droplet size",
					},
					&cli.StringFlag{
						Name:  "region",
						Value: "nyc3", // Default droplet region
						Usage: "Optional droplet region",
					},
				},
				Action: func(c *cli.Context) error {
					// Abort on configuration error
					if configErr != nil {
						return configErr
					}
					slug := fmt.Sprintf("cloudexec-%s", config.Username)
					// Check if a local cloudexec.toml exists
					if _, err := os.Stat(LaunchConfigFilePath); os.IsNotExist(err) {
						// Check if the path to a launch config is provided
						if c.Args().Len() < 1 {
							return fmt.Errorf("please provide a path to a cloudexec.toml file or create one in the current directory")
						}
						LaunchConfigFilePath = c.Args().Get(0)
					}
					// Load the launch configuration
					lc, err := LoadLaunchConfig(LaunchConfigFilePath)
					if err != nil {
						return err
					}
					// Get the optional droplet size and region
					dropletSize := c.String("size")
					dropletRegion := c.String("region")
					// Initialize the s3 state
					err = Init(config, slug)
					if err != nil {
						return err
					}
					fmt.Printf("Launching a %s droplet in the %s region\n", dropletSize, dropletRegion)
					err = Launch(config, dropletSize, dropletRegion, lc)
					if err != nil {
						log.Fatal(err)
					}
					return nil
				},
			},

			{
				Name:    "status",
				Usage:   "Get status of running jobs",
				Aliases: []string{"s"},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "all",
						Aliases: []string{"a"},
						Usage:   "show all jobs, including failed, cancelled, and completed",
					},
				},
				Action: func(c *cli.Context) error {
					// Abort on configuration error
					if configErr != nil {
						return configErr
					}
					slug := fmt.Sprintf("cloudexec-%s", config.Username)
					// Initialize the s3 state
					err := Init(config, slug)
					if err != nil {
						return err
					}
					showAll := c.Bool("all")
					err = PrintStatus(config, slug, showAll)
					if err != nil {
						return err
					}
					return nil
				},
			},

			{
				Name:  "pull",
				Usage: "Pulls down the results of the latest successful job",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "job",
						Value: 0,
						Usage: "Optional job ID to pull results from",
					},
				},
				Action: func(c *cli.Context) error {
					// Abort on configuration error
					if configErr != nil {
						return configErr
					}
					slug := fmt.Sprintf("cloudexec-%s", config.Username)
					// Check if the path is provided
					if c.Args().Len() < 1 {
						return fmt.Errorf("please provide a path to download job outputs to")
					}
					path := c.Args().Get(0)
					// Initialize the s3 state
					err := Init(config, slug)
					if err != nil {
						return err
					}
					existingState, err := state.GetState(config, slug)
					if err != nil {
						return err
					}
					if c.Int("job") != 0 {
						err = DownloadJobOutput(config, c.Int("job"), path, slug)
						if err != nil {
							return err
						}
						return nil
					} else {
						latestCompletedJob, err := state.GetLatestCompletedJob(slug, existingState)
						if err != nil {
							return err
						}
						err = DownloadJobOutput(config, int(latestCompletedJob.ID), path, slug)
						if err != nil {
							return err
						}
						return nil
					}
				},
			},

			{
				Name:  "logs",
				Usage: "Stream logs from a running job",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "job",
						Value: 0,
						Usage: "Optional job ID to get logs from",
					},
				},
				Action: func(c *cli.Context) error {
					// Abort on configuration error
					if configErr != nil {
						return configErr
					}
					slug := fmt.Sprintf("cloudexec-%s", config.Username)
					// Initialize the s3 state
					err := Init(config, slug)
					if err != nil {
						return err
					}
					existingState, err := state.GetState(config, slug)
					if err != nil {
						return err
					}
					latestJob := existingState.GetLatestJob()
					jobID := int(latestJob.ID)
					jobStatus := latestJob.Status
					// If there's a running job, stream the logs directly from the droplet
					if jobStatus == state.Provisioning || jobStatus == state.Running {
						err = ssh.StreamLogs()
						if err != nil {
							return err
						}
						return nil
					} else if c.Int("job") != 0 {
						jobID := c.Int("job")
						err := GetLogsFromBucket(config, jobID, slug)
						return err
					} else {
						err := GetLogsFromBucket(config, jobID, slug)
						return err
					}
				},
			},

			{
				Name:    "attach",
				Aliases: []string{"a"},
				Usage:   "Attach to a running job",
				Action: func(*cli.Context) error {
					// Abort on configuration error
					if configErr != nil {
						return configErr
					}
					slug := fmt.Sprintf("cloudexec-%s", config.Username)
					// Initialize the s3 state
					err := Init(config, slug)
					if err != nil {
						return err
					}
					// First check if there's a running job
					existingState, err := state.GetState(config, slug)
					if err != nil {
						return err
					}
					targetJob := existingState.GetLatestJob()
					jobStatus := targetJob.Status
					// Attach to the running job with tmux
					if jobStatus == state.Running {
						err = ssh.AttachToTmuxSession()
						if err != nil {
							return err
						}
						return nil
					} else {
						fmt.Println("error: Can't attach, no running job found")
						fmt.Println("Check the status of the job with cloudexec status")
						return nil
					}
				},
			},

			{
				Name:  "cancel",
				Usage: "Cancels any running cloudexec jobs",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "job",
						Value: 0,
						Usage: "Optional job ID to get logs from",
					},
				},
				Action: func(c *cli.Context) error {
					if configErr != nil {
						return configErr // Abort on configuration error
					}
					slug := fmt.Sprintf("cloudexec-%s", config.Username)
					err := Init(config, slug) // Initialize the s3 state
					if err != nil {
						return err
					}
					existingState, err := state.GetState(config, slug)
					if err != nil {
						return err
					}
					jobID := c.Int64("job")
					var targetJob *state.Job
					if jobID == 0 {
						targetJob = existingState.GetLatestJob()
					} else {
						targetJob = existingState.GetJob(jobID)
					}
					err = CancelJob(targetJob, existingState, config)
					if err != nil {
						return err
					}
					return nil
				},
			},

			{
				Name:  "clean",
				Usage: "Cleans up any running cloudexec droplets and clears the spaces bucket",
				Action: func(*cli.Context) error {
					if configErr != nil {
						return configErr // Abort on configuration error
					}
					slug := fmt.Sprintf("cloudexec-%s", config.Username)
					err := Init(config, slug) // Initialize the s3 state
					if err != nil {
						return err
					}
					existingState, err := state.GetState(config, slug)
					if err != nil {
						return err
					}
					err = ConfirmCancelAll(config, existingState)
					if err != nil {
						return err
					}
					// clean existing files from the bucket
					err = ResetBucket(config, slug)
					if err != nil {
						return err
					}
					return nil
				},
			},

			{
				Name:  "state",
				Usage: "Manage state file",
				Subcommands: []*cli.Command{

					{
						Name:  "list",
						Usage: "List jobs in the state file",
						Action: func(c *cli.Context) error {
							// Abort on configuration error
							if configErr != nil {
								return configErr
							}
							slug := fmt.Sprintf("cloudexec-%s", config.Username)
							// Initialize the s3 state
							err := Init(config, slug)
							if err != nil {
								return err
							}
							// Retrieve existing state
							existingState, err := state.GetState(config, slug)
							if err != nil {
								return err
							}
							// Print the jobs from the state
							for _, job := range existingState.Jobs {
								fmt.Printf("Job ID: %d, Status: %s\n", job.ID, job.Status)
							}
							return nil
						},
					},

					{
						Name:  "rm",
						Usage: "Remove a job from the state file",
						Action: func(c *cli.Context) error {
							// Abort on configuration error
							if configErr != nil {
								return configErr
							}
							slug := fmt.Sprintf("cloudexec-%s", config.Username)
							// Initialize the s3 state
							err := Init(config, slug)
							if err != nil {
								return err
							}
							jobID := c.Args().First() // Get the job ID from the arguments
							if jobID == "" {
								fmt.Println("Please provide a job ID to remove")
								return nil
							}
							// Convert jobID string to int64
							id, err := strconv.ParseInt(jobID, 10, 64)
							if err != nil {
								fmt.Printf("Invalid job ID: %s\n", jobID)
								return nil
							}
							newState := &state.State{}
							deleteJob := state.Job{
								ID:     id,
								Delete: true,
							}
							newState.CreateJob(deleteJob)
							err = state.UpdateState(config, slug, newState)
							if err != nil {
								return err
							}
							return nil
						},
					},

					{
						Name:  "json",
						Usage: "Output the raw state file as JSON",
						Action: func(c *cli.Context) error {
							// Abort on configuration error
							if configErr != nil {
								return configErr
							}
							slug := fmt.Sprintf("cloudexec-%s", config.Username)
							// Initialize the s3 state
							err := Init(config, slug)
							if err != nil {
								return err
							}
							// Retrieve existing state
							existingState, err := state.GetState(config, slug)
							if err != nil {
								return err
							}
							// output the raw json
							json, err := json.MarshalIndent(existingState, "", "  ")
							if err != nil {
								return err
							}
							fmt.Println(string(json))
							return nil
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

}
