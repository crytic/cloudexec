package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/s3"
)

func GetLogsFromBucket(config config.Config, jobID int64) error {
	itemKey := fmt.Sprintf("job-%d/cloudexec.log", jobID)

	log, err := s3.GetObject(config, itemKey)
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			return fmt.Errorf("The specified job logs do not exist. Please check the job ID and try again.")
		}
		return fmt.Errorf("Failed to read log data: %w", err)
	}

	// Convert log to a string
	logString := string(log)

	// Print the log with `less`, starting at the end of the file
	less := exec.Command("less", "+G")
	less.Stdin = strings.NewReader(logString)
	less.Stdout = os.Stdout
	less.Stderr = os.Stderr
	err = less.Run()
	if err != nil {
		return fmt.Errorf("Failed to run less: %w", err)
	}
	return nil
}
