FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o restapi ./main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/restapi /usr/local/bin/restapi
EXPOSE 8080
CMD ["restapi"]