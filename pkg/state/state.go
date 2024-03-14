package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/crytic/cloudexec/pkg/config"
	do "github.com/crytic/cloudexec/pkg/digitalocean"
	"github.com/crytic/cloudexec/pkg/s3"
)

type JobStatus string

const maxRetries = 3

const (
	Provisioning JobStatus = "provisioning"
	Running      JobStatus = "running"
	Completed    JobStatus = "completed"
	Failed       JobStatus = "failed"
	Cancelled    JobStatus = "cancelled"
	Timedout     JobStatus = "timedout"
)

type Job struct {
	Name        string    `json:"name"`
	ID          int64     `json:"id"`
	StartedAt   int64     `json:"started_at"` // Unix timestamp
	CompletedAt int64     `json:"completed_at"`
	UpdatedAt   int64     `json:"updated_at"`
	Status      JobStatus `json:"status"`
	Delete      bool
	Droplet     do.Droplet `json:"droplet"`
}

type State struct {
	Jobs []Job `json:"jobs"`
}

func GetState(config config.Config) (*State, error) {
	stateKey := "state/state.json"
	var state State

	// Read the state.json object data
	stateData, err := s3.GetObject(config, stateKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to read state data, make sure you've run 'cloudexec init': %w", err)
	}

	// Unmarshal the state JSON data into a State struct
	err = json.Unmarshal(stateData, &state)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal state JSON: %w", err)
	}

	// Replace empty names with a placeholder
	for i, job := range state.Jobs {
		if job.Name == "" {
			state.Jobs[i].Name = "no name"
		}
	}

	return &state, nil
}

// GetJob returns a job with the specified ID or nil if not found.
func (s *State) GetJob(jobID int64) *Job {
	for _, job := range s.Jobs {
		if job.ID == jobID {
			return &job
		}
	}
	return nil
}

func UpdateState(config config.Config, newState *State) error {
	// TODO: Handle locking to prevent concurrent updates
	stateKey := "state/state.json"

	existingState, err := GetState(config)
	if err != nil {
		return err
	}

	// Merge the existing state and the new state
	MergeStates(existingState, newState)

	// Marshal the merged state struct to JSON
	mergedStateJSON, err := json.Marshal(existingState)
	if err != nil {
		return fmt.Errorf("Failed to marshal merged state to JSON: %w", err)
	}

	for i := 1; i <= maxRetries; i++ {

		err = s3.PutObject(config, stateKey, mergedStateJSON)

		if err == nil {
			break
		}

		if i < maxRetries {
			time.Sleep(time.Duration(i) * time.Second)
		} else {
			return fmt.Errorf("Failed to update state after %d retries: %w", maxRetries, err)
		}
	}

	return nil
}

func MergeStates(existingState, newState *State) {
	// Create a map to keep track of deleted jobs
	deletedJobs := make(map[int64]bool)

	// Iterate through the jobs in the new state
	for _, newJob := range newState.Jobs {
		jobFound := false

		// Iterate through the existing jobs and update if the job ID matches
		for i, existingJob := range existingState.Jobs {
			if existingJob.ID == newJob.ID {
				// If the delete flag is set, remove the job from the existing state
				if newJob.Delete {
					existingState.Jobs = append(existingState.Jobs[:i], existingState.Jobs[i+1:]...)
					deletedJobs[newJob.ID] = true
				} else {
					existingState.Jobs[i] = newJob
				}
				jobFound = true
				break
			}
		}

		// If the job is not found in the existing state and should not be deleted, add it
		if !jobFound && !newJob.Delete {
			existingState.Jobs = append(existingState.Jobs, newJob)
		}
	}

	// Remove deleted jobs from the new state
	newState.Jobs = removeDeletedJobs(newState.Jobs, deletedJobs)
}

// Helper function to remove deleted jobs from the new state
func removeDeletedJobs(jobs []Job, deletedJobs map[int64]bool) []Job {
	filteredJobs := jobs[:0]
	for _, job := range jobs {
		if !deletedJobs[job.ID] {
			filteredJobs = append(filteredJobs, job)
		}
	}
	return filteredJobs
}

// CreateJob adds a new job to the state.
func (s *State) CreateJob(job Job) {
	s.Jobs = append(s.Jobs, job)
}

// GetLatestJob returns the latest job in the state.
func (s *State) GetLatestJob() *Job {
	if len(s.Jobs) > 0 {
		return &s.Jobs[len(s.Jobs)-1]
	}
	return nil
}

// DeleteJob removes a job with the specified ID from the state.
func (s *State) DeleteJob(jobID int64) {
	for i, job := range s.Jobs {
		if job.ID == jobID {
			s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			break
		}
	}
}

func (s *State) CancelRunningJob(config config.Config, jobID int64) error {
	// Mark any running jobs as cancelled
	for i, job := range s.Jobs {
		if job.ID == jobID {
			if job.Status == Running || job.Status == Provisioning {
				fmt.Printf("Setting status of job %d to 'Cancelled'\n", job.ID)
				s.Jobs[i].Status = Cancelled
				break
			} else {
				return fmt.Errorf("Job %v is not running", jobID)
			}
		}
	}
	err := UpdateState(config, s)
	if err != nil {
		return err
	}

	return nil
}

func GetLatestCompletedJob(state *State) (*Job, error) {
	var latestCompletedJob *Job

	// Find the latest completed job
	for i := len(state.Jobs) - 1; i >= 0; i-- {
		job := state.Jobs[i]
		if job.Status == Completed || job.Status == Failed {
			latestCompletedJob = &job
			break
		}
	}

	if latestCompletedJob == nil {
		return nil, errors.New("no completed jobs available")
	}

	return latestCompletedJob, nil
}

func GetJobIdsByInstance(config config.Config) (map[int64][]int64, error) {
	existingState, err := GetState(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to get state: %w", err)
	}
	instanceToJobIds := make(map[int64][]int64)
	if existingState.Jobs == nil {
		return instanceToJobIds, nil
	}
	for _, job := range existingState.Jobs {
		if job.Droplet.ID == 0 {
			fmt.Printf("Warning: Uninitialized droplet id for job %d\n", job.ID)
		}
		instanceToJobIds[job.Droplet.ID] = append(instanceToJobIds[job.Droplet.ID], job.ID)
	}
	return instanceToJobIds, nil
}
