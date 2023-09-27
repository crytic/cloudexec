#!/bin/bash
# shellcheck source=/dev/null
set -e

########################################
## Required Configuration and Dependencies

# set hostname
echo "Setting hostname..."
echo "cloudexec" >/etc/hostname
hostname -F /etc/hostname

echo "Installing prereqs..."
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y jq s3cmd tmux python3-pip python3-venv unzip

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

echo "Installing solc and slither..."
python3 -m venv ~/venv
source ~/venv/bin/activate
pip3 install solc-select slither-analyzer crytic-compile
solc-select install 0.8.6
solc-select use 0.8.6

echo "Downloading echidna..."
curl -fsSL -o /tmp/echidna.zip https://github.com/crytic/echidna/releases/download/v2.2.1/echidna-2.2.1-Linux.zip
echo "Extracting echidna..."
unzip /tmp/echidna.zip -d /tmp
tar -xzf /tmp/echidna.tar.gz -C /tmp
echo "Installing echidna..."
mv /tmp/echidna /usr/local/bin
rm /tmp/echidna.tar.gz

echo "Downloading medusa..."
sudo apt-get update
sudo apt-get install -y unzip
curl -fsSL https://github.com/crytic/medusa/releases/download/v0.1.0/medusa-linux-x64.zip -o medusa.zip
unzip medusa.zip
chmod +x medusa
sudo mv medusa /usr/local/bin

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
