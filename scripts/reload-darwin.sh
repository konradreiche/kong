#!/usr/bin/env bash
cp kong.plist ~/Library/LaunchAgents/com.github.konradreiche.kong.plist
launchctl unload -w ~/Library/LaunchAgents/com.github.konradreiche.kong.plist || true
launchctl load -w ~/Library/LaunchAgents/com.github.konradreiche.kong.plist
