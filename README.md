# 🦍 Kong
![golangci-lint](https://github.com/konradreiche/kong/actions/workflows/lint-and-test.yml/badge.svg)

Kong is a Jira CLI for low-latency interactions with Jira's API which is known to take multiple seconds to respond. Through background caching Kong responds in 10ms or less from the comfort of your terminal speeding up your daily agile chores.

## Usage

```
kong
```

## Features

- List issues, epics and sprints
- Create issues in batch
- Create sprints

## Installation

### Requirements

- Go 1.17+
- systemctl (Linux)
- launchctl (macOS)

This will checkout the repository, compile the Go code, create a user service, prompt you to configure Kong for your Jira API and reload the service.

```
git clone git@github.com:konradreiche/kong.git && cd kong
make install
kong configure
make reload
```
