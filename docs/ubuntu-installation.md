# Ubuntu Installation Guide for Lash

This guide covers multiple ways to install Lash on Ubuntu and other Debian-based Linux distributions.

## Quick Install (Recommended)

### Option 1: Download .deb Package

```bash
# Download the latest .deb package
wget https://github.com/lacymorrow/lash/releases/latest/download/lash_Linux_x86_64.deb

# Install the package
sudo dpkg -i lash_Linux_x86_64.deb

# Fix any dependency issues (if needed)
sudo apt-get install -f
```

### Option 2: Direct Binary Installation

```bash
# Download and extract the binary
wget https://github.com/lacymorrow/lash/releases/latest/download/lash_Linux_x86_64.tar.gz
tar -xzf lash_Linux_x86_64.tar.gz

# Install to system path
sudo mv lash/lash /usr/local/bin/

# Make executable (should already be)
sudo chmod +x /usr/local/bin/lash
```

## Architecture-Specific Downloads

Lash provides builds for multiple architectures:

| Architecture | Download Link |
|-------------|---------------|
| Intel/AMD 64-bit | `lash_Linux_x86_64.{deb,tar.gz}` |
| ARM 64-bit | `lash_Linux_arm64.{deb,tar.gz}` |
| Intel 32-bit | `lash_Linux_i386.{deb,tar.gz}` |
| ARM 32-bit | `lash_Linux_armv7.{deb,tar.gz}` |

Replace the download URL with your specific architecture.

## Alternative Installation Methods

### Go Install (if you have Go installed)

```bash
go install github.com/lacymorrow/lash@latest
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/lacymorrow/lash.git
cd lash

# Build the binary
go build -o lash .

# Install to system path
sudo mv lash /usr/local/bin/
```

## Verification

After installation, verify Lash is working:

```bash
# Check version
lash --version

# Check help
lash --help
```

## Shell Completions

The .deb package automatically installs shell completions for:
- Bash: `/etc/bash_completion.d/lash`
- Zsh: `/usr/share/zsh/site-functions/_lash`  
- Fish: `/usr/share/fish/vendor_completions.d/lash.fish`

If installed via binary, you can manually install completions:

```bash
# Generate and install bash completion
lash completion bash | sudo tee /etc/bash_completion.d/lash

# Generate and install zsh completion
lash completion zsh | sudo tee /usr/share/zsh/site-functions/_lash

# Generate and install fish completion
lash completion fish | sudo tee /usr/share/fish/vendor_completions.d/lash.fish
```

## Man Page

The .deb package includes a man page at `/usr/share/man/man1/lash.1.gz`.

For binary installations, you can view the manual:

```bash
lash man
```

## Uninstallation

### If installed via .deb package:
```bash
sudo apt remove lash
```

### If installed via binary:
```bash
sudo rm /usr/local/bin/lash
sudo rm /etc/bash_completion.d/lash
sudo rm /usr/share/zsh/site-functions/_lash
sudo rm /usr/share/fish/vendor_completions.d/lash.fish
```

## Troubleshooting

### Permission Issues
If you get permission errors, ensure the binary is executable:
```bash
chmod +x /usr/local/bin/lash
```

### Command Not Found
If `lash` is not found after installation:

1. Check if `/usr/local/bin` is in your PATH:
   ```bash
   echo $PATH | grep -o '/usr/local/bin'
   ```

2. If not, add it to your shell profile:
   ```bash
   echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
   source ~/.bashrc
   ```

### Dependency Issues (for .deb package)
If you encounter dependency issues:
```bash
sudo apt-get update
sudo apt-get install -f
```

## Getting Started

Once installed, you can start using Lash:

```bash
# Initialize lash in your project
cd your-project
lash

# Follow the interactive setup to configure your API keys
```

For more information, see the [main README](../README.md) and [configuration documentation](../README.md#configuration).
