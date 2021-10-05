#!/usr/bin/env bash
if [ "$(uname)" == "Darwin" ]; then
	launchctl unload -w ~/Library/LaunchAgents/com.github.konradreiche.kong.plist || true
else
	systemctl --user stop kong.service
fi
