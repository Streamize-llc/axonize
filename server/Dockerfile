FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY go.sum* ./
COPY . .
RUN go mod tidy && go mod download
RUN CGO_ENABLED=0 go build -o /axonize-server ./cmd/axonize-server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /axonize-server /usr/local/bin/
EXPOSE 4317 8080
ENTRYPOINT ["axonize-server"]
