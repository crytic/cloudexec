package main

import (
	"os"
	"strconv"
	"time"

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
    table.SetHeader([]string{"Job ID", "Status", "Droplet ID", "Droplet IP", "Started At", "Updated At", "Completed At"})

    formatDate := func(timestamp int64) string {
      if timestamp == 0 {
        return ""
      }
      return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
    }

    formatInt := func(i int64) string {
      if i == 0 {
        return ""
      }
      return strconv.Itoa(int(i))
    }

    // Find the latest completed job
    latestCompletedJob, err := state.GetLatestCompletedJob(bucketName, existingState)
    if err != nil {
      return err
    }

    for _, job := range existingState.Jobs {
      if showAll || (job.Status == state.Running || job.Status == state.Provisioning) || (latestCompletedJob != nil && job.ID == latestCompletedJob.ID) {
        table.Append([]string{
          strconv.Itoa(int(job.ID)),
          string(job.Status),
          formatInt(job.InstanceID),
          job.InstanceIP,
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
