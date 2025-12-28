# Admin Backend

A custom PocketBase backend with Go migrations, IP whitelisting, and Fly.io deployment.

## Prerequisites

- Go 1.24+
- Docker (for testing and local development)

## Project Structure

```
.
├── main.go                 # Application entry point with IP whitelist middleware
├── main_test.go            # Integration tests using testcontainers
├── migrations/             # Database migrations
│   ├── migrations.go
│   └── ...
├── pb_data/                # PocketBase data directory (gitignored)
├── Dockerfile
├── docker-compose.yml
└── fly.toml                # Fly.io deployment configuration
```

## Local Development

### Run the server

```bash
go run main.go serve
```

The admin UI will be available at http://localhost:8090/\_/

### Run with Docker

```bash
docker compose up --build
```

## Migrations

### Apply migrations

```bash
go run main.go migrate up
```

### Rollback migrations

```bash
go run main.go migrate down 1
```

### Create a new migration

```bash
go run main.go migrate create migration_name
```

### Sync migration history

If migrations get out of sync, clean up orphaned entries:

```bash
go run main.go migrate history-sync
```

## Testing

Tests use [testcontainers-go](https://golang.testcontainers.org/) to spin up a real PocketBase instance in Docker.

```bash
go test -v -timeout 120s ./...
```

## Deployment

The application is deployed to [Fly.io](https://fly.io).

### Deploy

```bash
fly deploy
```

### View logs

```bash
fly logs
```

## Security

The application implements IP whitelisting:

- Fly.io 6PN (private network) traffic is always allowed
- Standard private network ranges are allowed
- External access requires the `ALLOWED_HOME_IP` environment variable to be set

Set your home IP in Fly.io secrets:

```bash
fly secrets set ALLOWED_HOME_IP="your.ip.address"
```

CIDR notation is supported (e.g., `192.168.1.0/24`).

## Environment Variables

| Variable               | Description                                      |
| ---------------------- | ------------------------------------------------ |
| `ALLOWED_HOME_IP`      | IP address or CIDR allowed external access       |
| `POCKETBASE_TEST_MODE` | Set to `true` to bypass IP checks (testing only) |
