# Firecracker snapshot demo using go-sdk

This repository contains the code for the demo presented during `Hands-on Introduction to Firecracker | Rawkode Live`.

## Building

```
go build launcher.go
```

## Prerequisites

The application assumes that the working directory contains:

1. `firecracker`: the firecracker binary.
2. `rootfs.ext4`: a root filesystem to boot the microvm from.
3. `vmlinux.bin`: the kernel that the microvm will use.

## Running

1. Launch a microvm:

```
./launcher --socket 1.sock
```

2. Create a snapshot:

```
./launcher --socket 1.sock --toSnapshot state1
```

3. Load a snapshot:

```
./launcher --socket 2.sock --fromSnapshot state1
```
