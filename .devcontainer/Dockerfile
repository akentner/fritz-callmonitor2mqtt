# Golang Dev Container
FROM golang:1.23-bullseye

# Install additional tools and dependencies including sudo
RUN apt-get update && apt-get install -y \
    git \
    curl \
    wget \
    build-essential \
    sudo \
    vim \
    nano \
    htop \
    net-tools \
    telnet \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Install Go tools (with compatible versions)
RUN go install golang.org/x/tools/gopls@v0.18.0 \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install honnef.co/go/tools/cmd/staticcheck@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Set up vscode user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=1000

# Create user and add to sudo group
RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME -s /bin/bash \
    && echo "$USERNAME ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Set up Go module cache with proper permissions
RUN mkdir -p /go/pkg/mod \
    && chown -R $USERNAME:$USERNAME /go

# Set working directory
WORKDIR /workspaces

# Change ownership of workspace to vscode user
RUN chown -R $USERNAME:$USERNAME /workspaces

# Switch to vscode user
USER $USERNAME

# Set environment variables for Go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH
