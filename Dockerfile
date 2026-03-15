FROM golang:1.26 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOEXPERIMENT=runtimesecret CGO_ENABLED=0 go build -o /bin/sekeve ./cmd/sekeve
RUN CGO_ENABLED=0 go install github.com/grpc-ecosystem/grpc-health-probe@v0.4.46

FROM alpine:3.23
RUN apk add --no-cache gnupg
COPY --from=builder /bin/sekeve /bin/sekeve
COPY --from=builder /go/bin/grpc-health-probe /bin/grpc-health-probe
ENTRYPOINT ["/bin/sekeve"]
