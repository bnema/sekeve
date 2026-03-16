FROM golang:1.26 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOEXPERIMENT=runtimesecret CGO_ENABLED=0 go build -o /bin/sekeve ./cmd/sekeve

FROM alpine:3.23
RUN apk add --no-cache gnupg
COPY --from=builder /bin/sekeve /bin/sekeve
ENTRYPOINT ["/bin/sekeve"]
