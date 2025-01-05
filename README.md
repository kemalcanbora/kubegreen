# KubeGreen

KubeGreen is a terminal-based Kubernetes management tool that provides an interactive interface for common Kubernetes operations.

## Features

- **Context Management**: Switch between different Kubernetes contexts easily
- **Pod Management**: View and manage Kubernetes pods
- **Certificate Management**: View and renew TLS certificates
- **Volume Management**: Resize Persistent Volume Claims (PVCs) with data preservation
  - Support for both expanding and shrinking volumes
  - Automatic data backup during resize
  - Safe volume resizing with running pods

## Prerequisites

- Go 1.22 or higher
- Access to a Kubernetes cluster
- `kubectl` configured with proper contexts

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/kubegreen.git
cd kubegreen

# Build the project
go build -o kubegreen cmd/main.go

# Run the application
./kubegreen
```

## Usage

Navigate through the interface using:
- Arrow keys (↑/↓) or `j`/`k` to move up/down
- `Enter` to select
- `Backspace` to go back
- `q` to quit
- `Esc` to cancel current operation

### Volume Resize Operation

1. Select "volumes" from the main menu
2. Choose the PVC you want to resize
3. Enter the new size (e.g., "10Gi")
4. Wait for the operation to complete

The tool will:
- Check if the resize is safe
- Handle running pods
- Preserve data during resize
- Clean up temporary resources

### Context Switching

1. Select "contexts" from the main menu
2. Choose the context you want to switch to
3. Confirm the switch

### Pod Management

1. Select "pod" from the main menu
2. View pod details including:
   - Status
   - Resource usage
   - Age
   - Restart count

### Certificate Management

1. Select "certificates" from the main menu
2. View certificate details including:
   - Expiration date
   - Status
   - Renewal options

## Development

### Project Structure

```
kubegreen/
├── cmd/
│   └── main.go           # Application entry point
├── internal/
│   ├── controller/       # Kubernetes operations
│   └── model/           # UI models and state management
├── go.mod               # Go module file
└── README.md           # This file
```

### Building from Source

```bash
# Get dependencies
go mod download

# Build
go build -o kubegreen cmd/main.go
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- Uses the official [Kubernetes Go client](https://github.com/kubernetes/client-go) 