package digitalocean

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/util"

	"github.com/crytic/cloudexec/pkg/config"
	"github.com/crytic/cloudexec/pkg/s3"
)

type Droplet struct {
	Name    string
	ID      int64
	IP      string
	Created string
}

type Snapshot struct {
	Name string
	ID   string
}

/*
 * the vps hub, everything related to digital ocean server management
 * exports the following functions:
 * - CheckAuth(config config.Config) (string, error)
 * - CreateDroplet(config config.Config, username string, region string, size string, userData string, jobId int64, publicKey string) (Droplet, error)
 * - GetDropletsByName(config config.Config, dropletName string) ([]Droplet, error)
 * - DeleteDroplet(config config.Config, dropletID int64) error
 * - GetLatestSnapshot(config config.Config) (Snapshot, error)
 */

var doClient *godo.Client
var ctx context.Context

const timeLayout = time.RFC3339

////////////////////////////////////////
// Internal Helper Functions

func initializeDOClient(accessToken string) (*godo.Client, error) {
	// Immediately return our cached client if available
	if doClient != nil {
		return doClient, nil
	}
	doClient = godo.NewFromToken(accessToken)
	ctx = context.TODO()
	return doClient, nil
}

func createSSHKeyOnDigitalOcean(keyName string, publicKey string) (string, error) {
	createKeyRequest := &godo.KeyCreateRequest{
		Name:      keyName,
		PublicKey: publicKey,
	}

	key, _, err := doClient.Keys.Create(ctx, createKeyRequest)
	if err != nil {
		return "", fmt.Errorf("Failed to create SSH key on DigitalOcean: %w", err)
	}

	return key.Fingerprint, nil
}

func findSSHKeyOnDigitalOcean(dropletName string) (string, error) {
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 200, // Maximum allowed by DigitalOcean
	}

	keys, _, err := doClient.Keys.List(context.Background(), opt)
	if err != nil {
		return "", fmt.Errorf("Failed to list DigitalOcean SSH keys: %w", err)
	}

	for _, key := range keys {
		if key.Name == dropletName {
			return key.Fingerprint, nil
		}
	}

	return "", fmt.Errorf("SSH key with name '%s' not found", dropletName)
}

////////////////////////////////////////
// Exported Functions

func CheckAuth(config config.Config) (string, error) {
	// create a client
	doClient, err := initializeDOClient(config.DigitalOcean.ApiKey)
	if err != nil {
		return "", err
	}

	greenCheck := "\u2705"
	noEntry := "\U0001F6AB"

	_, _, err = doClient.Account.Get(context.Background())
	if err != nil {
		return "", fmt.Errorf("%s Failed to authenticate with DigitalOcean API: %w", noEntry, err)
	}
	doResp := fmt.Sprintf("%s Successfully authenticated with DigitalOcean API", greenCheck)

	// Check Spaces authentication
	_, err = s3.ListBuckets(config)
	if err != nil {
		return "", fmt.Errorf("%s Failed to authenticate with DigitalOcean Spaces API: %w", noEntry, err)
	}
	bucketResp := fmt.Sprintf("%s Successfully authenticated with DigitalOcean Spaces API", greenCheck)

	return fmt.Sprintf("%s\n%s", doResp, bucketResp), nil
}

func CreateDroplet(config config.Config, username string, region string, size string, userData string, jobId int64, publicKey string) (Droplet, error) {
	var droplet Droplet
	// create a client
	doClient, err := initializeDOClient(config.DigitalOcean.ApiKey)
	if err != nil {
		return droplet, err
	}

	dropletName := fmt.Sprintf("cloudexec-%v", username)

	sshKeyFingerprint, err := findSSHKeyOnDigitalOcean(dropletName)
	if err == nil {
		fmt.Printf("SSH key %v found on DigitalOcean with fingerprint: %v\n", dropletName, sshKeyFingerprint)
	} else {
		// Create the SSH key on DigitalOcean
		fmt.Println("Creating SSH key on DigitalOcean...")
		keyName := fmt.Sprintf("cloudexec-%v", username)
		sshKeyFingerprint, err = createSSHKeyOnDigitalOcean(keyName, publicKey)
		if err != nil {
			return droplet, fmt.Errorf("Failed to create SSH key on DigitalOcean: %w", err)
		}
		fmt.Printf("SSH key created on DigitalOcean with fingerprint: %v\n", sshKeyFingerprint)
	}

	snap, err := GetLatestSnapshot(config)
	if err != nil {
		return droplet, fmt.Errorf("Failed to get snapshot ID: %w", err)
	}

	// Create a new droplet
	fmt.Println("Creating droplet...")
	createRequest := &godo.DropletCreateRequest{
		Name:   dropletName,
		Region: region,
		Size:   size,
		Image: godo.DropletCreateImage{
			Slug: snap.ID,
		},
		UserData: userData,
		SSHKeys: []godo.DropletCreateSSHKey{
			{
				Fingerprint: sshKeyFingerprint,
			},
		},
		Tags: []string{
			"Purpose:cloudexec",
			"Owner:" + username,
			"Job:" + fmt.Sprintf("%v", jobId),
		},
		// Don't install the droplet agent
		WithDropletAgent: new(bool),
	}

	newDroplet, resp, err := doClient.Droplets.Create(ctx, createRequest)
	if err != nil {
		return droplet, fmt.Errorf("Failed to create droplet: %w", err)
	}
	var action *godo.LinkAction
	for _, a := range resp.Links.Actions {
		if a.Rel == "create" {
			action = &a
			break
		}
	}

	if action != nil {
		_ = util.WaitForActive(ctx, doClient, action.HREF)
		doDroplet, _, err := doClient.Droplets.Get(context.TODO(), newDroplet.ID)
		if err != nil {
			return droplet, fmt.Errorf("Failed to get droplet by id: %w", err)
		}
		newDroplet = doDroplet
	}

	droplet.Created = newDroplet.Created
	droplet.Name = newDroplet.Name
	droplet.ID = int64(newDroplet.ID)
	droplet.IP, err = newDroplet.PublicIPv4()
	if err != nil {
		return droplet, fmt.Errorf("Failed to get droplet IP: %w", err)
	}

	return droplet, nil
}

// GetDropletsByName returns a list of droplets with the given tag using a godo client
func GetDropletsByName(config config.Config, dropletName string) ([]Droplet, error) {
	var droplets []Droplet
	// create a client
	doClient, err := initializeDOClient(config.DigitalOcean.ApiKey)
	if err != nil {
		return droplets, err
	}

	opts := &godo.ListOptions{}

	for {
		dropletList, resp, err := doClient.Droplets.ListByName(ctx, dropletName, opts)
		if err != nil {
			return droplets, fmt.Errorf("Failed to fetch droplets by name: %w", err)
		}

		for _, droplet := range dropletList {
			pubIp, err := droplet.PublicIPv4()
			if err != nil {
				return droplets, fmt.Errorf("Failed to fetch droplet IP: %w", err)
			}
			droplets = append(droplets, Droplet{
				Name:    droplet.Name,
				ID:      int64(droplet.ID),
				IP:      pubIp,
				Created: droplet.Created,
			})
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return droplets, fmt.Errorf("Failed to fetch page of droplets list: %w", err)
		}

		opts.Page = page + 1
	}

	return droplets, nil
}

func DeleteDroplet(config config.Config, dropletID int64) error {
	// create a client
	doClient, err := initializeDOClient(config.DigitalOcean.ApiKey)
	if err != nil {
		return err
	}
	_, err = doClient.Droplets.Delete(context.Background(), int(dropletID))
	if err != nil {
		return fmt.Errorf("Failed to delete droplet: %w", err)
	}
	return nil
}

func GetLatestSnapshot(config config.Config) (Snapshot, error) {
	empty := Snapshot{
		ID:   "",
		Name: "",
	}
	// create a client
	doClient, err := initializeDOClient(config.DigitalOcean.ApiKey)
	if err != nil {
		return empty, err
	}

	var latestSnapshot *godo.Snapshot
	var latestCreatedAt time.Time

	options := &godo.ListOptions{
		Page:    1,
		PerPage: 50,
	}

	for {
		snapshots, resp, err := doClient.Snapshots.ListDroplet(context.Background(), options)
		if err != nil {
			return empty, fmt.Errorf("Failed to list snapshots: %w", err)
		}
		for _, snapshot := range snapshots {
			snapshotCreatedAt, err := time.Parse(timeLayout, snapshot.Created)
			if err != nil {
				return empty, fmt.Errorf("Failed to parse snapshot creation timestamp: %w", err)
			}
			if (latestSnapshot == nil || snapshotCreatedAt.After(latestCreatedAt)) && strings.HasPrefix(snapshot.Name, "cloudexec-") {
				latestSnapshot = &snapshot
				latestCreatedAt = snapshotCreatedAt
			}
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		options.Page++
	}

  if latestSnapshot == nil {
    return Snapshot{
      ID:   "ubuntu-22-04-x64",
      Name: "default",
    }, nil
	}

	return Snapshot{
		ID:   latestSnapshot.ID,
		Name: latestSnapshot.Name,
	}, nil
}
