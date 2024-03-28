#!/bin/bash
# shellcheck source=/dev/null
set -e

########################################
## Helper Functions

function get_latest_version {
  project="$1"
  github_api="https://api.github.com/repos/${project}/releases/latest"
  curl -s "$github_api" | jq '.tag_name' | tr -d 'v' | tr -d '"'
}

function get_latest_artifact {
  project="$1"
  github_api="https://api.github.com/repos/${project}/releases/latest"
  curl -s "$github_api" | jq '.assets[] | select(.name | test("linux")) | .browser_download_url' | grep -v "sigstore" | tr -d '"'
}

########################################
## Versions

doctl_version="1.92.0"
solc_version="0.8.6"
echidna_version="$(get_latest_version "crytic/echidna")"
medusa_version="$(get_latest_version "crytic/medusa")"

########################################
## Required Configuration and Dependencies

# set hostname
echo "Setting hostname..."
echo "cloudexec" >/etc/hostname
hostname -F /etc/hostname

echo "Installing prereqs..."
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y jq s3cmd tmux python3-pip python3-venv

echo "Downloading doctl..."
curl -fsSL -o /tmp/doctl-1.92.0-linux-amd64.tar.gz https://github.com/digitalocean/doctl/releases/download/v1.92.0/doctl-1.92.0-linux-amd64.tar.gz
echo "Extracting doctl..."
tar -xzf /tmp/doctl-1.92.0-linux-amd64.tar.gz -C /tmp
echo "Installing doctl..."
mv /tmp/doctl /usr/local/bin
echo "Cleaning up..."
rm /tmp/doctl-1.92.0-linux-amd64.tar.gz

########################################
## Common fuzz testing and analysis tools

echo "Installing slither and solc v${solc_version}..."
python3 -m venv ~/venv
source ~/venv/bin/activate
pip3 install solc-select slither-analyzer crytic-compile
solc-select use latest --always-install

echo "Installing echidna v${echidna_version}..."
curl -fsSL -o /tmp/echidna.tar.gz "$(get_latest_artifact "crytic/echidna")"
tar -xzf /tmp/echidna.tar.gz -C /tmp
mv /tmp/echidna /usr/local/bin
chmod +x /usr/local/bin/echidna
rm /tmp/echidna.tar.gz

echo "Installing medusa v${medusa_version}..."
curl -fsSL -o /tmp/medusa.zip "$(get_latest_artifact "crytic/medusa")"
unzip /tmp/medusa.zip -d /tmp
sudo mv /tmp/medusa /usr/local/bin
chmod +x /usr/local/bin/medusa
rm /tmp/medusa.zip

echo "Installing docker and its dependencies..."
apt-get install -y apt-transport-https ca-certificates curl gnupg-agent software-properties-common
docker_key="$(curl -fsSL https://download.docker.com/linux/ubuntu/gpg)"
echo "${docker_key}" | apt-key add -
release="$(lsb_release -cs)"
add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu ${release} stable"
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io
user="$(whoami)"
usermod -aG docker "${user}"
systemctl enable docker
