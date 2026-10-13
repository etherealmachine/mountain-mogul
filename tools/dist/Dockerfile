# The demo's Windows and Linux build environment (make demo-cross): Go,
# the X11 and GL headers, and MinGW, in an x86-64 Debian 11 container (run
# under emulation on an Apple silicon Mac) so the Linux build links
# against an old glibc (2.31, the Steam runtime's) and runs on most
# distros.
FROM --platform=linux/amd64 debian:bullseye

ARG GO_VERSION=1.26.2
# Bullseye has moved to the Debian archive.
RUN printf '%s\n' \
        'deb http://archive.debian.org/debian bullseye main' \
        'deb http://archive.debian.org/debian bullseye-updates main' \
        'deb http://archive.debian.org/debian-security bullseye-security main' > /etc/apt/sources.list \
    && apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl make zip binutils gcc libc6-dev pkg-config \
        gcc-mingw-w64-x86-64 \
        libgl1-mesa-dev libx11-dev libxcursor-dev libxrandr-dev \
        libxinerama-dev libxi-dev libxxf86vm-dev \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
ENV PATH=/usr/local/go/bin:$PATH GOFLAGS=-buildvcs=false
WORKDIR /src
