FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /netmap .

FROM alpine:3.20
RUN apk add --no-cache iputils bind-tools curl
COPY --from=builder /netmap /usr/local/bin/netmap
ENTRYPOINT ["netmap"]
