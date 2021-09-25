#!/usr/bin/env bash
mkdir -p ~/.config/systemd/user
cp kong.service ~/.config/systemd/user
systemctl --user disable kong.service
systemctl --user enable kong.service
systemctl --user restart kong.service
