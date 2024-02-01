# CloudExec

CloudExec is a management tool for running computation jobs on DigitalOcean via the command line. It is general purpose; Cloudexec can set up the server with arbitrary dependencies and then run an arbitrary workload process, but it is designed to run a single long-running code analysis job such as a fuzz testing or mutation testing campaign. Output data and runtime logs are uploaded to DigitalOcean's S3-style object storage for later analysis and, when the job is complete, the server is automatically destroyed. The client-side management engine of CloudExec is written in Golang and the server itself is managed by a Bash script.

Features:

- 1Password CLI support for secure DigitalOcean API key management. CloudExec will help you configure these credentials and verify that they are valid.
- Launch config file allows specification of:
  - An input folder which is uploaded to the runtime server and also to DigitalOcean's S3-style object storage for later reference. This folder is zipped for speedy uploads.
  - A job name, providing human-readable tags for each job.
  - A timeout, after which the workload process will be terminated if it hasn't finished already, output will be uploaded to persistent storage, and the server will be destroyed so you will stop being charged for it.
  - A setup command which uses bash to install dependencies and prepare the server to run the workload process.
  - A run command which will kick off the workload process
- An `init` subcommand for creating a new, default launch config file
- Output is periodically uploaded to DigitalOcean's S3-style object storage so you can pull results incrementally from a running job
- Jobs can be cancelled early if the workload process hasn't completed or the timeout hasn't been reached yet
- Monitoring the runtime logs of a specific job or the status of all jobs
- Automatic `ssh_config` additions allowing you to access a running server by executing `ssh cloudexec`
- Tracks the cumulative costs incurred for running processes and the total cost of completed processes

Run `cloudexec help` to list available subcommands or `cloudexec <subcommand> --help` for information regarding a specific subcommand:

```bash
$ cloudexec help
NAME:
   cloudexec - easily run cloud based jobs

USAGE:
   cloudexec [global options] command [command options] [arguments...]

COMMANDS:
   check, c    Verifies cloud authentication
   configure   Configure credentials
   init        Create a new cloudexec.toml launch configuration in the current directory
   launch, l   Launch a droplet and start a job
   logs        Stream logs from a running job
   cancel      Cancels any running cloudexec jobs
   clean       Cleans up any running cloudexec droplets and clears the spaces bucket
   pull        Pulls down the results of the latest successful job
   status, s   Get status of running jobs
   state       Manage state file
   attach, a   Attach to a running job
   version, v  Gets the version of the app
   help, h     Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h  show help
```

Example job status output:

```bash
$ cloudexec status --all
+--------+----------------+-----------+----------------+---------+------+-------+---------------------+---------------------+--------------+-------------+------------+
| JOB ID |    JOB NAME    |  STATUS   |   DROPLET IP   | MEMORY  | CPUS | DISK  |     STARTED AT      |     UPDATED AT      | TIME ELAPSED | HOURLY COST | TOTAL COST |
+--------+----------------+-----------+----------------+---------+------+-------+---------------------+---------------------+--------------+-------------+------------+
| 1      | medusa fuzz    | completed | 12.34.56.78    | 4096 MB | 4    | 50 GB | 2024-01-01 13:55:53 | 2024-01-02 14:05:29 | 2 days       | $0.125      | $6.0100    |
+--------+----------------+-----------+----------------+---------+------+-------+---------------------+---------------------+--------------+-------------+------------+
| 2      | experiment     | failed    | 12.34.56.79    | 2048 MB | 2    | 25 GB | 2024-01-01 14:04:50 | 2024-01-01 14:08:03 | 3 minutes    | $0.0625     | $0.0034    |
+--------+----------------+-----------+----------------+---------+------+-------+---------------------+---------------------+--------------+-------------+------------+
| 3      | slither-mutate | running   | 12.34.56.80    | 2048 MB | 2    | 25 GB | 2024-01-02 10:04:50 | 2024-01-02 12:08:03 | 2 hours      | $0.0625     | $0.125     |
+--------+----------------+-----------+----------------+---------+------+-------+---------------------+---------------------+--------------+-------------+------------+
```

## Getting Started

### Installation

#### Install with Brew

```bash
brew tap trailofbits/tools
brew install cloudexec
```

#### Upgrade with Brew

```bash
brew update && brew upgrade cloudexec
```

alternatively, you can install from a GitHub release:

### Install from a GitHub release

Download the latest release for your platform from the [releases page](https://github.com/crytic/cloudexec/releases).

#### Release verification

Releases are signed with sigstore. You can verify using [`cosign`](https://github.com/sigstore/cosign) with the following example command:

```bash
cosign verify-blob \
    --certificate-identity-regexp "https://github.com/crytic/cloudexec.*" \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com  \
    --bundle cloudexec-<version>-<os>-<arch>.tar.gz.bundle \
    cloudexec-<version>-<os>-<arch>.tar.gz
```

#### Install from a tarball

```bash
tar -xzf cloudexec-<version>-<os>-<arch>.tar.gz
mv cloudexec /usr/local/bin
```

#### Install from source

Running the command below will build the CLI tool from source with a binary named `cloudexec` in a `dist` folder:

```bash
make build
```

Then, move the resulting binary from `./dist/clouexec` into your `PATH`.

Nix users can run `nix build` and then `nix profile install ./result` to install `cloudexec`. A helper command `make nix-install` is available which performs these steps for you and also upgrades an existing version of `cloudexec` that might already be installed.

### Configure credentials

CloudExec requires DigitalOcean API credentials to manage droplets, and Spaces credentials to store state and job data. The recommended method for storing and providing your credentials securely is by using the 1Password CLI.

CloudExec supports natively integrating with 1Password, allowing you to reference your credentials stored in your 1Password vault. However, you can also choose to provide plaintext credentials using the `cloudexec configure` command. Additionally, you can override individual values or the entire configuration by setting the corresponding environment variables.

#### Get credentials from DigitalOcean

[API Token](https://cloud.digitalocean.com/account/api/tokens)

[Spaces Token](https://cloud.digitalocean.com/account/api/spaces)

#### Configure 1password CLI (optional)

Save the above tokens in your 1Password vault and [install the 1password CLI](https://developer.1password.com/docs/cli/get-started/#step-1-install-1password-cli).

```bash
brew install --cask 1password/tap/1password-cli # see the link above for installation instructions on other platforms
```

[Sign in to your 1Password account](https://developer.1password.com/docs/cli/sign-in-manually/).

```bash
eval $(op signin)
```

Note what your [1Password secret references](https://developer.1password.com/docs/cli/secret-references/) are and use them in place of your actual secret values during the `cloudexec configure` or env var setup steps described in the next section.

These references generally follow the format: `op://<vault-name>/<item-name>/<field-name>`. For example, if you saved your keys to a vault called `Private`, in an item called `DigitalOcean` and the api key field is called `ApiKey`, then the secret reference to use is `op://Private/DigitalOcean/ApiKey`.

#### Configure CloudExec

```bash
cloudexec configure
```

or set environment variables:

```bash
DIGITALOCEAN_API_KEY
DIGITALOCEAN_SPACES_ACCESS_KEY
DIGITALOCEAN_SPACES_SECRET_ACCESS_KEY
DIGITALOCEAN_SPACES_REGION
```

Remember, if you save secret values to a `.env` file, never commit it to any version control system. Add such `.env` files to your project's `.gitignore` file to help prevent making such mistakes.

### Check CloudExec access

Confirm `cloudexec` has access to DigitalOcean.

```bash
cloudexec check
```

### Launch a new remote job

Generate a cloudexec.toml configuration file in the current directory.

```bash
cloudexec init
```

Update the `cloudexec.toml` as needed.

```bash
# default nyc3 region and c-2 size droplet, using a cloudexec.toml file in the current directory
cloudexec launch
# custom region and droplet size
cloudexec launch --size c-4 --region sfo2
```

### Stream logs from the provisioning script

```bash
cloudexec logs
```

Note that the `logs` subcommand will continue to stream logs until you stop with ctrl-c, even after the job is finished and stops producing new logs. This is a read-only command and it is safe to kill it at any point.

### Get logs from a previous run

```bash
cloudexec logs --job 1
```

### Attach to the running job

```bash
cloudexec attach

# or
ssh -t cloudexec tmux attach -s cloudexec
```

### SSH to your droplet

```bash
ssh cloudexec
```

### Check on the status of your jobs

```bash
# show only runnning jobs, and the last completed job
cloudexec status
# show all jobs
cloudexec status --all
```

### Sync files from a completed job to a local path

```bash
# pull from the latest successful job
cloudexec pull example/output
# pull from any job ID
cloudexec pull --job 1 example/output

```

### Cancel any in progress jobs

```bash
cloudexec cancel
```

### Cleanup all bucket contents and reset state (destructive)

```bash
cloudexec clean
```

Note that there is often a delay while deleting files from Digital Ocean Spaces buckets.

## Optional: Create a CloudExec DigitalOcean image

Building and uploading a dedicated DigitalOcean image for `cloudexec` will simplify your launch configuration and improve startup times.

To do so, install `packer` with `brew install packer`. If you're using `nix` and `direnv`, it's added to your PATH via the flake's dev shell.

To build and upload a docker image, run the following command. Make sure your DigitalOcean API key is either in your env vars or replace it with the actual token.

`packer build -var do_api_token=$DIGITALOCEAN_API_KEY cloudexec.pkr.hcl`

This will take care of everything and if you visit the [DigitalOcean snapshots page](https://cloud.digitalocean.com/images/snapshots/droplets), you'll see a snapshot called `cloudexec-20230920164605` or similar. `cloudexec` will search for snapshots starts with a `cloudexec-` prefix and it will use the one with the most recent timestamp string.

Now, you can remove everything from the setup command in the example launch config or replace it to install additional tools.
