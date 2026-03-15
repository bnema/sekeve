FROM golang:1.26 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOEXPERIMENT=runtimesecret CGO_ENABLED=0 go build -o /bin/sekeve ./cmd/sekeve

FROM gcr.io/distroless/static-debian12
COPY --from=builder /bin/sekeve /bin/sekeve
ENTRYPOINT ["/bin/sekeve"]
