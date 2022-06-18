ARG LDFLAGS=-s -w
FROM golang:alpine as builder
ENV LDFLAGS=$LDFLAGS
ADD . /work
RUN cd /work && \
    CGO_ENABLED=0 && \
    go build -ldflags="$LDFLAGS" -o edgevpn

# TODO: move to distroless
FROM alpine
COPY --from=builder /work/edgevpn /usr/bin/edgevpn

ENTRYPOINT ["/usr/bin/edgevpn"]
