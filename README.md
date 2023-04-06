# CloudFuzz

CloudFuzz is a command-line tool for easily running cloud-based fuzzing jobs on DigitalOcean. Start, manage and pull the results of fuzzing jobs from your terminal.

## Getting Started

### Installation

#### Install with Brew

```bash
brew tap trailofbits/tools
brew install cloudfuzz
```

#### Upgrade with Brew

```bash
brew update && brew upgrade cloudfuzz
```

alternatively, you can install from a GitHub release:

### Install from a GitHub release

Download the latest release for your platform from the [releases page](https://github.com/trailofbits/cloudfuzz/releases).

#### Release verification

Releases are signed with sigstore. You can verify using [`cosign`](https://github.com/sigstore/cosign) with the following example command:

```bash
cosign verify-blob \
    --certificate-identity-regexp "https://github.com/trailofbits/cloudfuzz.*" \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com  \
    --bundle cloudfuzz-<version>-<os>-<arch>.tar.gz.bundle \
    cloudfuzz-<version>-<os>-<arch>.tar.gz
```

#### Install from a tarball

```bash
tar -xzf cloudfuzz-<version>-<os>-<arch>.tar.gz
mv cloudfuzz /usr/local/bin
```

#### Install from source

Running the command below will build the CLI tool from source with a binary named `cloudfuzz` in a `dist` folder:

```bash
make build
```

Then, move the resulting binary from `./dist/cloufuzz` into your `PATH`.

Nix users can run `nix build` and then `nix profile install ./result` to install `cloudfuzz`. A helper command `make nix-install` is available which performs these steps for you and also upgrades an existing version of `cloudfuzz` that might already be installed.

### Configure credentials

CloudFuzz requires DigitalOcean API credentials to manage droplets, and Spaces credentials to store state and job data. The recommended method for storing and providing your credentials securely is by using the 1Password CLI.

CloudFuzz supports natively integrating with 1Password, allowing you to reference your credentials stored in your 1Password vault. However, you can also choose to provide plaintext credentials using the `cloudfuzz configure` command. Additionally, you can override individual values or the entire configuration by setting the corresponding environment variables.

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

Note what your [1Password secret references](https://developer.1password.com/docs/cli/secret-references/) are and use them in place of your actual secret values during the `cloudfuzz configure` or env var setup steps described in the next section.

These references generally follow the format: `op://<vault-name>/<item-name>/<field-name>`. For example, if you saved your keys to a vault called `Private`, in an item called `DigitalOcean` and the api key field is called `ApiKey`, then the secret reference to use is `op://Private/DigitalOcean/ApiKey`.

#### Configure CloudFuzz

```bash
cloudfuzz configure
```

or set environment variables:

```bash
DIGITALOCEAN_API_KEY
DIGITALOCEAN_SPACES_ACCESS_KEY
DIGITALOCEAN_SPACES_SECRET_ACCESS_KEY
DIGITALOCEAN_SPACES_REGION
```

Remember, if you save secret values to a `.env` file, never commit it to any version control system. Add such `.env` files to your project's `.gitignore` file to help prevent making such mistakes.

### Check CloudFuzz access

Confirm `cloudfuzz` has access to DigitalOcean.

```bash
cloudfuzz check
```

### Initialize your CloudFuzz environment

```bash
cloudfuzz init
```

### Launch a new remote fuzzing job

Generate a cloudfuzz.toml configuration file in the current directory.

```bash
cloudfuzz launch init
```

Update the `cloudfuzz.toml` as needed.

```bash
# default nyc3 region and c-2 size droplet, using a cloudfuzz.toml file in the current directory
cloudfuzz launch
# custom region and droplet size
cloudfuzz launch --size c-4 --region sfo2
```

### Stream logs from the provisioning script

```bash
cloudfuzz logs
```

Note that the `logs` subcommand will continue to stream logs until you stop with ctrl-c, even after the job is finished and stops producing new logs. This is a read-only command and it is safe to kill it at any point.

### Get logs from a previous run

```bash
cloudfuzz logs --job 1
```

### Attach to the running job

```bash
cloudfuzz attach

# or
ssh -t cloudfuzz tmux attach -s cloudfuzz
```

### SSH to your droplet

```bash
ssh cloudfuzz
```

### Check on the status of your jobs

```bash
# show only runnning jobs, and the last completed job
cloudfuzz status
# show all jobs
cloudfuzz status --all
```

### Sync files from a completed job to a local path

```bash
# pull from the latest successful job
cloudfuzz pull example/output
# pull from any job ID
cloudfuzz pull --job 1 example/output

```

### Cancel any in progress jobs

```bash
cloudfuzz cancel
```

### Cleanup all bucket contents and reset state (destructive)

```bash
cloudfuzz clean
```

Note that there is often a delay while deleting files from Digital Ocean Spaces buckets.
