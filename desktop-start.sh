#!/bin/bash

#startup the desktop agent on windows :)
cd "$(dirname "$0")/desktop-agent"
go run ./cmd/queue-up-agent