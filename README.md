# Sekeve

A CLI secret manager. Secrets are encrypted locally with your GPG key and stored on a remote gRPC server. The server sees only encrypted blobs. Your private key never leaves your machine.

Decrypted values are protected in memory using Go 1.26's `runtime/secret.Do`, which zeroes registers, stack, and heap after use.

## Install

```bash
go install -tags runtimesecret github.com/bnema/sekeve/cmd/sekeve@latest
```

Or build from source:

```bash
make build
make install
```

## Server setup

> [!WARNING]
> The gRPC server uses **plaintext HTTP/2** by default and does not terminate TLS. Do not expose it directly to the internet. Place it behind a reverse proxy that handles TLS, such as Caddy, nginx with a certificate, or Cloudflare proxy. All data on the wire is GPG-encrypted, but connection metadata and the auth handshake are not protected without TLS.

### Docker Compose (recommended)

```bash
docker compose up -d
```

On first run, initialize the server with your GPG public key:

```bash
docker compose run -it sekeve-server server init --data /data/sekeve.db
```

This opens an interactive prompt where you paste your armored GPG public key. The key is stored in the database; no file remains on disk.

You can also provide the key non-interactively:

```bash
# Via file
docker compose run sekeve-server server init --data /data/sekeve.db --pubkey-file /path/to/key.asc

# Via environment variable
SEKEVE_PUBKEY="$(gpg --export --armor you@example.com)" docker compose run sekeve-server server init --data /data/sekeve.db

# Via stdin
gpg --export --armor you@example.com | docker compose run -T sekeve-server server init --data /data/sekeve.db
```

### Local

```bash
sekeve server init --pubkey-file ./key.asc --data ./sekeve.db
sekeve server start --data ./sekeve.db --addr :50051
```

### Port configuration

Create a `.env` file (gitignored) to map the host port:

```env
SEKEVE_SERVER_PORT=50053
```

The container always listens on port 50051 internally.

## Client setup

Run the interactive setup:

```bash
sekeve init
```

This prompts for the server address and GPG key ID, writes `$XDG_CONFIG_HOME/sekeve/config.toml` (defaults to `~/.config/sekeve/config.toml`), and verifies the server connection.

Both fields can be overridden with `--server`, `--gpg-key` flags or `SEKEVE_SERVER_ADDR`, `SEKEVE_GPG_KEY_ID` environment variables.

## Environment variables

| Variable             | Description                                          | Default   |
|----------------------|------------------------------------------------------|-----------|
| `SEKEVE_LOG_LEVEL`   | Log level: `trace`, `debug`, `info`, `warn`, `error` | `info`    |
| `SEKEVE_LOG_FORMAT`  | Log output format: `console`, `json`                 | `console` |
| `SEKEVE_SERVER_ADDR` | gRPC server address                                  | `localhost:50051` |
| `SEKEVE_GPG_KEY_ID`  | GPG key ID for encryption/auth                       | (none)    |
| `SEKEVE_PUBKEY`      | Armored GPG public key for non-interactive server init | (none)  |

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
sekeve dmenu --copy "$(sekeve dmenu --list | fuzzel --dmenu --with-nth=1 --accept-nth=2)"
```

Logins copy the password, secrets copy the value, notes copy the full content.

## Import from Bitwarden

Export your vault from Bitwarden as unencrypted JSON (`bw export --format json`), then import:

```bash
sekeve import bitwarden ~/bw-export.json
```

Logins are imported with the username appended to the name (e.g., `GitHub (alice@work.com)`). URIs are normalized to strip paths while preserving subdomains. Secure notes are imported as-is. Cards, identities, and SSH keys are skipped.

Delete the export file after import - it contains plaintext credentials.

## Auth

Authentication uses GPG challenge-response. The server encrypts a nonce with your public key; the client decrypts it to prove identity. No passwords. Session tokens are cached locally for one hour.

## Development

```bash
make proto    # regenerate protobuf
make test     # run all tests
make lint     # golangci-lint
make mock     # regenerate mocks
make build    # build binary
make wipe     # delete all vault entries (requires jq)
make install  # install to $GOBIN
```

Requires Go 1.26+, `buf`, `mockery`, `golangci-lint`, and `gpg`.
