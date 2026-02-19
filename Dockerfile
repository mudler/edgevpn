# Define argument for linker flags
ARG LDFLAGS=-s -w

# Use a temporary build image based on Golang 1.20-alpine
FROM golang:1.26-alpine as builder

# Set environment variables: linker flags and disable CGO
ENV LDFLAGS=$LDFLAGS CGO_ENABLED=0

# Add the current directory contents to the work directory in the container
ADD . /work

# Set the current work directory inside the container
WORKDIR /work

# Install git and build the edgevpn binary with the provided linker flags
# --no-cache flag ensures the package cache isn't stored in the layer, reducing image size
RUN apk add --no-cache git && \
    go build -ldflags="$LDFLAGS" -o edgevpn

# TODO: move to distroless

# Use a new, clean alpine image for the final stage
FROM alpine

# Copy the edgevpn binary from the builder stage to the final image
COPY --from=builder /work/edgevpn /usr/bin/edgevpn

# Define the command that will be run when the container is started
ENTRYPOINT ["/usr/bin/edgevpn"]
