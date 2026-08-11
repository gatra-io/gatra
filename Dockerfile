# Step 1: Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /gatra ./cmd/gatra

# Step 2: Minimal runtime container
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /gatra /app/gatra

EXPOSE 8080

ENTRYPOINT ["/app/gatra"]
CMD ["start", "-c", "policy.json"]