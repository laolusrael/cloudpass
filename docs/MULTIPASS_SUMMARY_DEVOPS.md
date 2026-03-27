# Multipass Summary for DevOps Engineers

## Overview

Multipass is a lightweight VM manager by Canonical that provides cloud-style Ubuntu VMs on Linux, macOS, and Windows. It's designed for developers who need quick access to Ubuntu environments for testing, development, and prototyping cloud deployments.

## Why DevOps Engineers Should Use Multipass

### Key Benefits

1. **Rapid Environment Provisioning**
   - Create Ubuntu VMs in seconds with a single command
   - No manual hypervisor setup required
   - Automated cloud-init customization

2. **Cloud-Native Workflow**
   - Familiar cloud-init interface for VM customization
   - Infrastructure-as-Code ready
   - Supports custom images and URLs

3. **Resource Efficiency**
   - Minimal overhead compared to traditional VMs
   - On-demand instances (start/stop/suspend)
   - Configurable CPU, RAM, and disk allocation

4. **Development & Testing**
   - Spin up isolated test environments
   - Prototype cloud deployments locally
   - Test infrastructure scripts before production

## Common DevOps Use Cases

### 1. Local Kubernetes Development
```bash
# Launch a VM with sufficient resources for K8s
multipass launch --cpus 4 --memory 8G --disk 20G --name k8s-dev
multipass shell k8s-dev
```

### 2. CI/CD Pipeline Testing
```bash
# Launch ephemeral test instances
multipass launch --name ci-runner --cloud-init ci-config.yaml
multipass exec ci-runner -- ./run-tests.sh
multipass delete ci-runner --purge
```

### 3. Infrastructure Script Testing
```bash
# Test Ansible/Terraform scripts
multipass launch --name test-vm
multipass exec test-vm -- sudo apt-get update
multipass delete test-vm --purge
```

### 4. Docker/Container Development
```bash
# Run Docker inside Multipass
multipass launch --name docker-host
multipass exec docker-host -- sudo snap install docker
```

## Architecture Components

| Component | Description |
|-----------|-------------|
| **multipassd** | Background service/daemon managing VMs |
| **multipass** | CLI client for interacting with the service |
| **Driver** | Hypervisor backend (QEMU on Linux/macOS, Hyper-V on Windows) |
| **Image** | Ubuntu VM images from Ubuntu releases or custom URLs |

## Platform Support

| Platform | Default Driver | Notes |
|----------|---------------|-------|
| Linux | QEMU | KVM acceleration available |
| macOS | QEMU | Previously HyperKit (deprecated) |
| Windows | Hyper-V | Requires PowerShell setup |

## Best Practices

1. **Use the Primary Instance**
   - Automatically mounts host home directory
   - Quick access with `multipass shell` (no name needed)

2. **Leverage Cloud-Init**
   - Automate VM provisioning
   - Include users, packages, and configurations

3. **Use Snapshots for Testing**
   - Create baseline snapshots
   - Restore after destructive tests

4. **Configure Resource Limits**
   - Match VM resources to your workload
   - Use `--cpus`, `--memory`, `--disk` flags

5. **Use Mounts for Development**
   - Share code between host and VM
   - Use `--mount` option on launch or `multipass mount`

## Troubleshooting Tips

- **View logs**: `multipass info <name>` for instance details
- **Access service logs**: Check system logs for multipassd
- **Network issues**: Use `multipass networks` to list available interfaces
- **Performance**: Ensure KVM/Hyper-V acceleration is enabled

## Integration with Existing Tools

- **Terraform**: Use external provisioner with multipass CLI
- **Ansible**: Use `multipass exec` as inventory target
- **Docker**: Run Docker inside instances for container workloads
- **Packer**: Build custom Multipass images

## Quick Reference Commands

```bash
# Create and access VM
multipass launch                    # Create with defaults
multipass launch --name my-vm       # Named instance
multipass launch --cpus 4 --memory 8G  # Custom resources

# Manage instances
multipass list                     # List all instances
multipass start <name>             # Start instance
multipass stop <name>              # Stop instance
multipass delete <name>            # Delete instance
multipass purge                    # Remove deleted instances

# Access and execute
multipass shell <name>             # Interactive shell
multipass exec <name> -- <command> # Run command
multipass mount <src> <target>     # Mount directory

# Information
multipass info <name>              # Instance details
multipass networks                 # Available networks
multipass find                     # List available images
```

## Security Considerations

- VM isolation from host network
- ID mapping for user permissions
- Support for encrypted mounts
- Local passphrase for sensitive data

## Resources

- Documentation: https://documentation.ubuntu.com/multipass/
- GitHub: https://github.com/canonical/multipass
- Community: https://discourse.ubuntu.com/c/project/multipass/
