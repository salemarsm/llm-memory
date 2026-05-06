FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ginko      ./cmd/ginko && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ginko-admin ./cmd/ginko-admin && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/memserver   ./cmd/memserver && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/memmcp      ./cmd/memmcp && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/memctl      ./cmd/memctl

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/ /usr/local/bin/

VOLUME ["/data"]
ENV GINKO_DB=/data/ginko.db
EXPOSE 8787

ENTRYPOINT ["/usr/local/bin/ginko"]
CMD ["serve", "--db", "/data/ginko.db"]
