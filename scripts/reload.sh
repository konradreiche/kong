#!/usr/bin/env bash
if [ "$(uname)" == "Darwin" ]; then
	cp kong.plist ~/Library/LaunchAgents/com.github.konradreiche.kong.plist
	launchctl unload -w ~/Library/LaunchAgents/com.github.konradreiche.kong.plist || true
	launchctl load -w ~/Library/LaunchAgents/com.github.konradreiche.kong.plist
else
	mkdir -p ~/.config/systemd/user
	cp kong.service ~/.config/systemd/user
	systemctl --user disable kong.service
	systemctl --user enable kong.service
	systemctl --user restart kong.service
fi
