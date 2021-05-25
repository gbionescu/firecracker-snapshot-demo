package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	// This fork contains the LoadSnapshot logic
	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	ops "github.com/firecracker-microvm/firecracker-go-sdk/client/operations"
)

const (
	// How Firecracker is launched
	firecrackerPath = "./firecracker"
	kernelPath      = "vmlinux.bin"
	rootfsPath      = "rootfs.ext4"

	// Firecracker settings
	noCpus                 = 2
	memorySize             = 4096
	kernelArgs             = "console=ttyS0 reboot=k panic=1 pci=off quiet"
	firecrackerInitTimeout = 3
)

func launchVM(socketPath string) {
	// Remove the socket path if it exists
	if _, err := os.Stat(socketPath); err == nil {
		os.Remove(socketPath)
	}

	// Create a config structure that specifies how we launch
	// the microVM.
	cfg := firecracker.Config{
		SocketPath:      socketPath,
		KernelImagePath: kernelPath,
		KernelArgs:      kernelArgs,
		Drives:          firecracker.NewDrivesBuilder(rootfsPath).Build(),
		MachineCfg: models.MachineConfiguration{
			VcpuCount:       firecracker.Int64(noCpus),
			MemSizeMib:      firecracker.Int64(memorySize),
			HtEnabled:       firecracker.Bool(false),
			TrackDirtyPages: true,
		},
	}

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build the command
	cmd := firecracker.VMCommandBuilder{}.
		WithSocketPath(socketPath).
		WithBin(firecrackerPath).
		WithStdin(os.Stdin).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		Build(ctx)

	// Create a logger to have a nice output
	logger := log.New()

	// Create the machine instance
	machine, err := firecracker.NewMachine(
		ctx,
		cfg,
		firecracker.WithProcessRunner(cmd),
		firecracker.WithLogger(log.NewEntry(logger)))

	if err != nil {
		panic(fmt.Errorf("failed to create new machine: %v", err))
	}

	// Start the microVM
	if err := machine.Start(ctx); err != nil {
		panic(fmt.Errorf("Failed to start machine: %v", err))
	}
	defer machine.StopVMM()

	// wait for the VMM to exit
	if err := machine.Wait(ctx); err != nil {
		panic(fmt.Errorf("Wait returned an error %s", err))
	}
	os.Remove(socketPath)
}

// Create a snapshot to a given path.
// Handles an existing VM socket path and a snapshot path.
func createSnapshot(socketPath string, snapshotPath string) {
	cfg := firecracker.Config{SocketPath: socketPath}
	ctx := context.Background()

	// Create a logger to have a nice output
	logger := log.New()

	machine, err := firecracker.NewMachine(ctx, cfg, firecracker.WithLogger(log.NewEntry(logger)))
	if err != nil {
		panic(fmt.Errorf("failed to create new machine: %v", err))
	}

	machine.PauseVM(ctx)

	start := time.Now()
	machine.CreateSnapshot(ctx, snapshotPath+".mem", snapshotPath+".file",
		func(data *ops.CreateSnapshotParams) {
			data.Body.SnapshotType = "Diff"
		})
	fmt.Println("Created snapshot duration:", time.Since(start))

	machine.ResumeVM(ctx)
}

// Load a snapshot from a given path.
// Handles VM socket path and a snapshot path.
func loadSnapshot(socketPath string, snapshotPath string) {
	// Remove the socket path if it exists
	if _, err := os.Stat(socketPath); err == nil {
		os.Remove(socketPath)
	}

	cfg := firecracker.Config{
		SocketPath:        socketPath,
		DisableValidation: true,
	}

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build the command
	cmd := firecracker.VMCommandBuilder{}.
		WithSocketPath(socketPath).
		WithBin(firecrackerPath).
		WithStdin(os.Stdin).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		Build(ctx)

	logger := log.New()

	// Start Firecracker
	err := cmd.Start()
	if err != nil {
		logger.Error("Failed to start Firecracker")
	}

	machine, err := firecracker.NewMachine(ctx, cfg, firecracker.WithLogger(log.NewEntry(logger)))
	if err != nil {
		panic(fmt.Errorf("failed to create new machine: %v", err))
	}

	// TODO: WaitForSocket interface could look better
	errCh := make(chan error)
	machine.WaitForSocket(time.Duration(firecrackerInitTimeout)*time.Second, errCh)

	start := time.Now()
	machine.LoadSnapshot(ctx, snapshotPath+".mem", snapshotPath+".file")
	fmt.Println("Load snapshot duration:", time.Since(start))

	machine.ResumeVM(ctx)

	// wait for the VMM to exit
	if err := machine.Wait(ctx); err != nil {
		panic(fmt.Errorf("Wait returned an error %s", err))
	}
}

func main() {
	socketPath := flag.String("socket", "", "UDS socket path for Firecracker to use.")
	toSnapshot := flag.String("toSnapshot", "", "Save snapshot to file.")
	fromSnapshot := flag.String("fromSnapshot", "", "Load snapshot from a file.")
	flag.Parse()

	if *socketPath == "" {
		panic(fmt.Errorf("UDS socket path needed."))
	}

	if *toSnapshot != "" {
		createSnapshot(*socketPath, *toSnapshot)
		os.Exit(0)
	}

	if *fromSnapshot != "" {
		loadSnapshot(*socketPath, *fromSnapshot)
		os.Exit(0)
	}

	launchVM(*socketPath)
}
