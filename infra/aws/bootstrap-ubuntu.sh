#!/bin/bash
set -euo pipefail

# install basic dependencies
sudo apt update
sudo apt install -y ca-certificates curl gnupg lsb-release git

# install Docker if missing
if ! command -v docker >/dev/null 2>&1; then
  sudo install -m 0755 -d /etc/apt/keyrings || true
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  sudo chmod a+r /etc/apt/keyrings/docker.gpg

  source /etc/os-release
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu $VERSION_CODENAME stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list >/dev/null

  sudo apt update
  sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
else
  echo "Docker already installed: $(docker --version)"
fi

# enable Docker daemon and add current user to docker group
sudo systemctl enable --now docker
sudo usermod -aG docker "$USER" || true

echo "Bootstrap complete. Log out and back in for docker group changes to take effect."