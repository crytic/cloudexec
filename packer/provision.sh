#!/bin/bash
# shellcheck source=/dev/null
set -e

########################################
## Helper Functions

function github_api {
  project="$1"
  github_api="https://api.github.com/repos/${project}/releases/latest"
  curl -s "$github_api"
}

function get_latest_version {
  github_api "$1" | jq '.tag_name' | tr -d 'v' | tr -d '"'
}

function get_latest_artifact {
  github_api "$1" | jq '.assets[] | select(.name | test("linux")) | .browser_download_url' | tr -d '"'
}

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

echo "Installing doctl v$(get_latest_version "digitalocean/doctl")..."
curl -fsSL -o /tmp/doctl.tar.gz "$(get_latest_artifact "digitalocean/doctl" | grep "amd64")"
tar -xzf /tmp/doctl.tar.gz -C /tmp
mv /tmp/doctl /usr/local/bin
rm /tmp/doctl.tar.gz
echo "Done installing: $(doctl version)"

########################################
## Common fuzz testing and analysis tools

echo "Installing slither..."
python3 -m venv ~/venv
source ~/venv/bin/activate
pip3 install solc-select slither-analyzer crytic-compile
solc-select use latest --always-install

echo "Installing echidna v$(get_latest_version "crytic/echidna")..."
curl -fsSL -o /tmp/echidna.tar.gz "$(get_latest_artifact "crytic/echidna" | grep -v "sigstore")"
tar -xzf /tmp/echidna.tar.gz -C /tmp
mv /tmp/echidna /usr/local/bin
chmod +x /usr/local/bin/echidna
rm /tmp/echidna.tar.gz
echo "Done installing: $(echidna --version)"

echo "Installing medusa v$(get_latest_version "crytic/medusa")..."
curl -fsSL -o /tmp/medusa.tar.gz "$(get_latest_artifact "crytic/medusa" | grep -v "sigstore")"
tar -xzf /tmp/medusa.tar.gz -C /tmp
sudo mv /tmp/medusa /usr/local/bin
chmod +x /usr/local/bin/medusa
rm /tmp/medusa.tar.gz
echo "Done installing: $(medusa --version)"

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
echo "Done installing docker:"
docker version

echo "Done provisioning"
