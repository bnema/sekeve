# sekeve

A CLI secret manager. Secrets are encrypted locally with your GPG key and stored on a remote gRPC server. The server only sees encrypted blobs. Your private key never leaves your machine.

Decrypted values are protected in memory using Go 1.26's `runtime/secret.Do`, which zeroes registers, stack, and heap after use.

## Install

```bash
go install -tags runtimesecret github.com/bnema/sekeve/cmd/sekeve@latest
```

Or build from source:

```bash
GOEXPERIMENT=runtimesecret go build -o bin/sekeve ./cmd/sekeve
```

## Server setup

Initialize the database with your GPG public key, then start:

```bash
sekeve server init --gpg-key user@example.com --data ./sekeve.db
sekeve server start --data ./sekeve.db --addr :50051
```

Or use Docker Compose:

```bash
docker compose up -d
```

## Client config

Create `~/.config/sekeve/config.yaml`:

```yaml
server_addr: "localhost:50051"
gpg_key_id: "user@example.com"
```

Both fields can be overridden with `--server`, `--gpg-key` flags or `SEKEVE_SERVER_ADDR`, `SEKEVE_GPG_KEY_ID` env vars.

## Usage

```bash
# Add entries
sekeve add login github --site github.com --username user
sekeve add secret stripe-key sk_live_abc123
echo "some notes" | sekeve add note meeting-notes

# Retrieve and decrypt
sekeve get github
sekeve get stripe-key --json

# List and search
sekeve list
sekeve list --type login --json
sekeve search git

# Edit and delete
sekeve edit github
sekeve rm stripe-key
```

## Fuzzel / dmenu integration

List all entries in a picker-friendly format, then copy the selected value to clipboard:

```bash
sekeve dmenu --copy "$(sekeve dmenu --list | fuzzel --dmenu)"
```

Logins copy the password, secrets copy the value, notes copy the full content.

## Auth

Authentication uses GPG challenge-response. The server encrypts a nonce with your public key; the client decrypts it to prove identity. No passwords. Session tokens are cached locally for one hour.

## Development

```bash
make proto    # regenerate protobuf
make test     # run all tests
make lint     # golangci-lint
make mock     # regenerate mocks
make build    # build binary
```

Requires Go 1.26+, `buf`, `mockery`, `golangci-lint`, and `gpg`.
