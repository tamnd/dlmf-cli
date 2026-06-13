---
title: "Installation"
description: "Install dlmf from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/dlmf-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `dlmf` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/dlmf-cli/cmd/dlmf@latest
```

That puts `dlmf` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/dlmf-cli
cd dlmf-cli
make build        # produces ./bin/dlmf
./bin/dlmf version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/dlmf:latest --help
```

## Checking the install

```bash
dlmf version
```

prints the version and exits.
