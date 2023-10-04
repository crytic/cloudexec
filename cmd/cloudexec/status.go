package main

import (
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
	table.SetHeader([]string{"Job ID", "Status", "Droplet IP", "Memory", "CPUs", "Disk", "Hourly Cost", "Started At", "Updated At", "Completed At", "Total Cost"})

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
			totalCost := float64(latestUpdate-job.StartedAt) / float64(3600) * job.Droplet.Size.HourlyCost

			table.Append([]string{
				strconv.Itoa(int(job.ID)),
				string(job.Status),
				job.Droplet.IP,
				formatInt(job.Droplet.Size.Memory) + " MB",
				formatInt(job.Droplet.Size.CPUs),
				formatInt(job.Droplet.Size.Disk) + " GB",
				"$" + formatFloat(job.Droplet.Size.HourlyCost),
				formatDate(job.StartedAt),
				formatDate(job.UpdatedAt),
				"$" + formatFloat(totalCost),
			})

		}
	}

	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetRowLine(true)
	table.Render()
	return nil
}
