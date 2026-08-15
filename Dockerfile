# Stage 1: Build Go binary
FROM golang:1.26-alpine AS go-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
ARG VERSION=dev
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=1 go build -ldflags="-s -w -X main.Version=${VERSION}" -o /trmnl-mealie .

# Stage 2: Minimal runtime
FROM alpine:3.24
RUN apk add --no-cache ca-certificates curl
COPY --from=go-builder /trmnl-mealie /app/trmnl-mealie
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/app/trmnl-mealie"]
