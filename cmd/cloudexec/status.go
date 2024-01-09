package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/state"
	"github.com/olekukonko/tablewriter"
)

func PrintStatus(config config.Config, bucketName string, showAll bool) error {

	existingState, err := state.GetState(config, bucketName)
	if err != nil {
		return err
	}

	// Print the status of each job using tablewriter
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Job Name", "Job ID", "Status", "Droplet IP", "Memory", "CPUs", "Disk", "Started At", "Updated At", "Time Elapsed", "Hourly Cost", "Total Cost"})

	formatDate := func(timestamp int64) string {
		if timestamp == 0 {
			return ""
		}
		return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
	}

	formatElapsedTime := func(seconds int64) string {
		const (
			minute = 60
			hour   = minute * 60
			day    = hour * 24
			week   = day * 7
		)
		switch {
		case seconds < minute*2:
			return fmt.Sprintf("%d seconds", seconds)
		case seconds < hour*2:
			return fmt.Sprintf("%d minutes", seconds/minute)
		case seconds < day*2:
			return fmt.Sprintf("%d hours", seconds/hour)
		case seconds < week*2:
			return fmt.Sprintf("%d days", seconds/day)
		default:
			return fmt.Sprintf("%d weeks", seconds/week)
		}
	}

	formatInt := func(i int64) string {
		return strconv.Itoa(int(i))
	}

	formatFloat := func(f float64) string {
		return strconv.FormatFloat(f, 'f', 4, 64)
	}

	// Find the latest completed job
	latestCompletedJob, err := state.GetLatestCompletedJob(bucketName, existingState)
	if err != nil {
		return err
	}

	for _, job := range existingState.Jobs {
		if showAll || (job.Status == state.Running || job.Status == state.Provisioning) || (latestCompletedJob != nil && job.ID == latestCompletedJob.ID) {

			latestUpdate := func() int64 {
				if job.CompletedAt == 0 {
					return job.UpdatedAt
				}
				return job.CompletedAt
			}()
			elapsedTime := int64(latestUpdate - job.StartedAt)
			totalCost := float64(elapsedTime) / float64(3600) * job.Droplet.Size.HourlyCost

			table.Append([]string{
				string(job.Name),
				strconv.Itoa(int(job.ID)),
				string(job.Status),
				job.Droplet.IP,
				formatInt(job.Droplet.Size.Memory) + " MB",
				formatInt(job.Droplet.Size.CPUs),
				formatInt(job.Droplet.Size.Disk) + " GB",
				formatDate(job.StartedAt),
				formatDate(job.UpdatedAt),
				formatElapsedTime(elapsedTime),
				"$" + formatFloat(job.Droplet.Size.HourlyCost),
				"$" + formatFloat(totalCost),
			})

		}
	}

	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetRowLine(true)
	table.Render()
	return nil
}
