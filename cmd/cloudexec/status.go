package main

import (
	"os"
	"strconv"
	"time"

	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/olekukonko/tablewriter"
	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/state"
)

func PrintStatus(config config.Config, bucketName string, showAll bool) error {

    existingState, err := state.GetState(config, bucketName)
    if err != nil {
      return err
    }

    // Print the status of each job using tablewriter
    table := tablewriter.NewWriter(os.Stdout)
    table.SetHeader([]string{"Job ID", "Status", "Droplet IP", "Memory", "CPUs", "Disk", "Monthly Cost", "Hourly Cost", "Started At", "Updated At", "Completed At"})

    formatDate := func(timestamp int64) string {
      if timestamp == 0 {
        return ""
      }
      return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
    }

    formatInt := func(i int64) string {
      return strconv.Itoa(int(i))
    }

    formatFloat := func(f float64) string {
      return strconv.FormatFloat(f, 'f', -1, 64)
    }

    // Find the latest completed job
    latestCompletedJob, err := state.GetLatestCompletedJob(bucketName, existingState)
    if err != nil {
      return err
    }

    for _, job := range existingState.Jobs {
      if showAll || (job.Status == state.Running || job.Status == state.Provisioning) || (latestCompletedJob != nil && job.ID == latestCompletedJob.ID) {
        droplet, err := do.GetDropletById(config, job.InstanceID)
        if err != nil {
          return err
        }

        table.Append([]string{
          strconv.Itoa(int(job.ID)),
          string(job.Status),
          droplet.IP,
          formatInt(droplet.Size.Memory),
          formatInt(droplet.Size.CPUs),
          formatInt(droplet.Size.Disk),
          formatFloat(droplet.Cost.Monthly),
          formatFloat(droplet.Cost.Hourly),
          formatDate(job.StartedAt),
          formatDate(job.UpdatedAt),
          formatDate(job.CompletedAt),
        })
      }
    }

    table.SetAlignment(tablewriter.ALIGN_LEFT)
    table.SetRowLine(true)
    table.Render()
    return nil
}
