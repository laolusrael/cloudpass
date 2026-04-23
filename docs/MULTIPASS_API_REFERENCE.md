# Multipass API Reference Documentation

## Table of Contents

1. [Overview](#overview)
2. [Command-Line Interface](#command-line-interface)
3. [Instance Management Commands](#instance-management-commands)
4. [File Operations](#file-operations)
5. [Configuration Commands](#configuration-commands)
6. [Utility Commands](#utility-commands)
7. [Settings Reference](#settings-reference)
8. [Instance States](#instance-states)
9. [Instance Name Format](#instance-name-format)

---

## Overview

Multipass is a tool to generate cloud-style Ubuntu VMs quickly on Linux, macOS, and Windows. It provides a simple but powerful CLI that enables you to quickly access an Ubuntu command line or create your own local mini-cloud.

### Architecture

- **multipassd**: The Multipass daemon/service that runs in the background
- **multipass**: CLI client that communicates with the daemon
- **Driver**: Hypervisor backend (QEMU, Hyper-V)
- **Instance**: A virtual machine running Ubuntu
- **Image**: Ubuntu VM image used to create instances

---

## Command-Line Interface

### General Syntax

```bash
multipass [options] <command> [arguments]
```

### Global Options

| Option | Description |
|--------|-------------|
| `-h, --help` | Displays help on commandline options |
| `-v, --verbose` | Increase logging verbosity. Repeat for more detail (max 4: -vvvv) |

---

## Instance Management Commands

### launch

Create and start a new instance.

```bash
multipass launch [options] [[<remote:>]<image> | <url>]
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `-c, --cpus <cpus>` | Number of CPUs to allocate | 1 |
| `-d, --disk <disk>` | Disk space to allocate (K, M, G suffix) | 5G |
| `-m, --memory <memory>` | Amount of memory to allocate (K, M, G suffix) | 1G |
| `-n, --name <name>` | Name for the instance | Auto-generated |
| `--cloud-init <file>` | Path/URL to cloud-init config (use `-` for stdin) | - |
| `--network <spec>` | Add network interface | - |
| `--bridged` | Adds one bridged network | - |
| `--mount <local-path:instance-path>` | Mount local directory | - |
| `--timeout <timeout>` | Max wait time in seconds | 300 |

**Network Spec Format:**
```
key=value,key=value
```
Keys: `name` (required), `mode` (auto|manual), `mac` (hardware address)

**Arguments:**

| Argument | Description |
|----------|-------------|
| `image` | Optional image (default: Ubuntu LTS). Use `release:` or `daily:` prefix |
| `url` | Custom image URL (https://, http://, file://) |

**Examples:**
```bash
multipass launch                           # Default Ubuntu LTS
multipass launch 22.04                      # Specific Ubuntu release
multipass launch daily:ubuntu/devel         # Daily build
multipass launch --name my-vm --cpus 4      # Custom name and CPU
multipass launch --cloud-init userdata.yaml # With cloud-init
multipass launch --mount ~/projects:/workspace # With mount
```

---

### start

Start a stopped instance.

```bash
multipass start [options] <name>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity
- `--timeout <timeout>` - Max wait time (seconds)

**Examples:**
```bash
multipass start my-vm
multipass start primary
```

---

### stop

Stop a running instance.

```bash
multipass stop [options] <name>
```

**Options:**

| Option | Description |
|--------|-------------|
| `-h, --help` | Displays help |
| `-v, --verbose` | Increase verbosity |
| `--timeout <timeout>` | Max wait time (seconds) |
| `-t, --time <minutes>` | Suspend after inactivity (0 to cancel) |

**Examples:**
```bash
multipass stop my-vm
multipass stop --timeout 60 my-vm
```

---

### restart

Restart an instance.

```bash
multipass restart [options] <name>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity
- `--timeout <timeout>` - Max wait time (seconds)

---

### suspend

Suspend an instance.

```bash
multipass suspend [options] <name>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

---

### delete

Delete an instance.

```bash
multipass delete [options] <name>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity
- `-p, --purge` - Immediately remove deleted instances

**Examples:**
```bash
multipass delete my-vm
multipass delete my-vm --purge
```

---

### purge

Remove all deleted instances permanently.

```bash
multipass purge [options]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

---

### recover

Recover a deleted instance.

```bash
multipass recover [options] <name>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

---

### list (ls)

List all instances.

```bash
multipass list [options]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Output Columns:** Name, State, IPv4, Image

---

### info

Show detailed information about an instance.

```bash
multipass info [options] <name>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity
- `--format <format>` - Output format (table, json, yaml)

---

### shell

Open an interactive shell prompt on an instance.

```bash
multipass shell [options] [<name>]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity
- `--timeout <timeout>` - Max wait time (seconds)

**Arguments:**
- `name` - Instance name (defaults to 'primary')

**Notes:**
- If instance is not running, it will be started automatically
- Without arguments, opens shell to primary instance (creates if not exists)

**Examples:**
```bash
multipass shell
multipass shell my-vm
```

---

### exec

Run a command inside an instance.

```bash
multipass exec [options] <name> [--] <command>
```

**Options:**

| Option | Description |
|--------|-------------|
| `-h, --help` | Displays help |
| `-v, --verbose` | Increase verbosity |
| `-d, --working-directory <dir>` | Change to directory before execution |
| `-n, --no-map-working-directory` | Do not map host path to mounted path |

**Arguments:**
- `name` - Name of instance to execute on
- `command` - Command to execute

**Notes:**
- Use `--` to separate multipass options from command options
- Supports stdin/stdout piping

**Examples:**
```bash
multipass exec my-vm -- uname -r
multipass exec my-vm -- ls -la
multipass exec my-vm --working-directory /home -- ls -a
```

---

## File Operations

### mount

Mount a local directory to an instance.

```bash
multipass mount [options] <source> <target>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity
- `-u, --uid-map <host>:<instance>` - Map host UID to instance UID
- `-g, --gid-map <host>:<instance>` - Map host GID to instance GID

**Examples:**
```bash
multipass mount ~/projects my-vm:/workspace
multipass mount ~/data primary:~/data
```

---

### umount

Unmount a directory from an instance.

```bash
multipass umount [options] <target>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Examples:**
```bash
multipass umount my-vm:/workspace
multipass umount primary:~/data
```

---

### transfer

Transfer files between host and instances.

```bash
multipass transfer [options] <source> <target>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Examples:**
```bash
multipass transfer local-file.txt my-vm:/home/ubuntu/
multipass transfer my-vm:/var/log/syslog ./local-copy.txt
```

---

## Configuration Commands

### get

Get a configuration setting value.

```bash
multipass get [options] <key>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Examples:**
```bash
multipass get local.driver
multipass get client.primary-name
```

---

### set

Set a configuration setting value.

```bash
multipass set [options] <key>=<value>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Examples:**
```bash
multipass set local.driver=qemu
multipass set client.primary-name=main-vm
```

---

### alias

Create a command alias.

```bash
multipass alias [options] <instance> <alias>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Arguments:**
- `instance` - Instance to create alias for
- `alias` - Name for the alias

---

### aliases

List all aliases.

```bash
multipass aliases [options]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

---

### unalias

Remove a command alias.

```bash
multipass unalias [options] <alias>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

---

### networks

List available networks.

```bash
multipass networks [options]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Output:** Name, Type, IPv4, Description

---

### authenticate

Authenticate with the Multipass service.

```bash
multipass authenticate [options] <passphrase>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

---

### prefer

Set preferences for Multipass behavior.

```bash
multipass prefer [options]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

---

## Utility Commands

### find

Find and list available images.

```bash
multipass find [options] [<remote:>] [<string>]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Examples:**
```bash
multipass find                    # List all images
multipass find 20.04             # Search for Ubuntu 20.04
multipass find daily:             # List daily builds
```

---

### version

Show version information.

```bash
multipass version [options]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

---

### help

Display help information.

```bash
multipass help [options] [command]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Arguments:**
- `command` - Specific command to get help for

---

### wait-ready

Wait for an instance to be ready.

```bash
multipass wait-ready [options] <name>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity
- `--timeout <timeout>` - Max wait time (seconds)

---

### snapshot

Manage instance snapshots.

```bash
multipass snapshot [options] <name> [<snapshot-name>]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

**Subcommands:**
- `snapshot <name>` - Create snapshot of instance
- `snapshot <name> <snapshot-name>` - Create named snapshot

---

### restore

Restore an instance from a snapshot.

```bash
multipass restore [options] <name> [<snapshot-name>]
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity
- `--timeout <timeout>` - Max wait time (seconds)

---

### clone

Clone an instance.

```bash
multipass clone [options] <source> <target>
```

**Options:**
- `-h, --help` - Displays help
- `-v, --verbose` - Increase verbosity

---

## Settings Reference

### Client Settings

| Setting | Description | Default |
|---------|-------------|---------|
| `client.apps.windows-terminal.profiles` | Windows Terminal profiles | - |
| `client.gui.autostart` | Start GUI on system startup | false |
| `client.gui.hotkey` | GUI hotkey | Super+M |
| `client.primary-name` | Name for primary instance | primary |

### Local Settings

| Setting | Description | Default |
|---------|-------------|---------|
| `local.bridged-network` | Default bridged network interface | - |
| `local.driver` | Hypervisor driver | platform default |
| `local.passphrase` | Passphrase for encryption | - |
| `local.privileged-mounts` | Allow privileged mounts | false |

### Instance-Specific Settings

| Setting | Description |
|---------|-------------|
| `local.<instance-name>.bridged` | Enable bridged networking |
| `local.<instance-name>.cpus` | CPU allocation |
| `local.<instance-name>.disk` | Disk allocation |
| `local.<instance-name>.memory` | Memory allocation |

### Snapshot Settings

| Setting | Description |
|---------|-------------|
| `local.<instance-name>.<snapshot-name>.comment` | Snapshot comment |
| `local.<instance-name>.<snapshot-name>.name` | Snapshot display name |

---

## Instance States

An instance can be in one of the following states:

| State | Description |
|-------|-------------|
| `Starting` | Instance is booting up |
| `Running` | Instance is running and ready |
| `Stopping` | Instance is shutting down |
| `Stopped` | Instance is stopped |
| `Suspended` | Instance is suspended to disk |
| `Deleting` | Instance is being deleted |
| `Deleted` | Instance is deleted (recoverable) |

---

## Instance Name Format

Instance names must:
- Consist of letters, numbers, or hyphens
- Start with a letter
- End with an alphanumeric character
- Be unique

**Valid Examples:**
- `my-vm`
- `dev-server`
- `test-instance-1`

**Invalid Examples:**
- `1instance` (starts with number)
- `my-vm-` (ends with hyphen)

---

## Cloud-Init Integration

Multipass supports cloud-init for instance customization. Use the `--cloud-init` option with `launch`.

### Example user-data.yaml:

```yaml
#cloud-config
users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: sudo
    shell: /bin/bash
    ssh_authorized_keys:
      - ssh-rsa AAAA...

packages:
  - nginx
  - docker.io

runcmd:
  - systemctl enable nginx
  - systemctl start nginx
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `MULTIPASS_INSTANCE_NAME` | Set in exec/shell context |
| `MULTIPASS_HOST_HOME` | Host home directory path |

---

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Unhandled exception |

---

## Additional Resources

- Official Documentation: https://documentation.ubuntu.com/multipass/
- GitHub Repository: https://github.com/canonical/multipass
- Community Forum: https://discourse.ubuntu.com/c/project/multipass/
- Cloud-Init Documentation: https://cloudinit.readthedocs.io/
