#!/bin/bash

#run this script in wsl or linux to setup docker and make the agent run on startup
set -e

echo "🚀 Updating packages..."
sudo apt update

echo "📦 Installing Docker if not present..."
if ! command -v docker &> /dev/null
then
    sudo apt install -y docker.io
else
    echo "Docker already installed"
fi

echo "🔧 Enabling Docker service..."
sudo systemctl enable docker || true

echo "▶️ Starting Docker service..."
sudo service docker start

echo "👤 Adding user to docker group..."
sudo usermod -aG docker $USER

echo "🔄 Applying group changes..."
newgrp docker <<EOF
echo "Group updated"
EOF

echo "🧪 Testing Docker..."
docker run hello-world || echo "⚠️ You may need to restart your shell"

echo "✅ Docker setup complete!"

echo ""
echo "⚡ Optional: auto-start Docker on WSL launch"

BASHRC_LINE="sudo service docker start > /dev/null 2>&1"

if ! grep -Fxq "$BASHRC_LINE" ~/.bashrc
then
    echo "$BASHRC_LINE" >> ~/.bashrc
    echo "Added auto-start to ~/.bashrc"
else
    echo "Auto-start already configured"
fi

echo ""
echo "🎉 Done. Restart your terminal or run: exec bash"