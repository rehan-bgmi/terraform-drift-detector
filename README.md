# Driftctl — Terraform Drift Detection

Driftctl continuously compares Terraform state files against live cloud infrastructure to detect configuration drift — without running `terraform plan` or `terraform apply`.

## Features

- **Expected state** from Terraform state (local file or S3)
- **Actual state** from cloud provider APIs (AWS: EC2, VPC, Subnet, Security Group, S3)
- **Normalized resource model** with drift detection for missing resources, extras, attribute changes, and tag changes
- **On-demand scans** via CLI or REST API
- **Scheduled scans** via cron
- **Outputs**: JSON, table (CLI), web dashboard
- **SQLite** persistence (workspaces, scans, schedules)

## Architecture

```
Terraform State → State Reader → Extractor → Expected Model ─┐
                                                              ├→ Drift Engine → Report
Cloud APIs      → Cloud Fetcher → Extractor → Actual Model  ─┘
```

## Quick Start

### Build

```bash
make build
```

### Run a scan (state-only test mode)

```bash
./bin/driftctl scan --state testdata/state/sample.tfstate --provider aws --skip-cloud
```

### Run a live AWS scan

```bash
export AWS_REGION=us-east-1
./bin/driftctl scan --state /path/to/terraform.tfstate --provider aws --region us-east-1
```

Exit codes: `0` = no drift, `1` = drift detected, `2` = error.

### Start API server + dashboard

```bash
make run-server
# Open http://localhost:8080
```

## CLI

```bash
# Scan with workspace from config
driftctl scan --config configs/driftctl.yaml --workspace prod

# Scan ad-hoc state file
driftctl scan --state terraform.tfstate --provider aws --output json

# View saved report
driftctl report <scan-id> --output table

# List workspaces
driftctl workspace list

# Create schedule
driftctl schedule create --workspace prod --cron "0 6 * * *"
```

## Configuration

See `configs/driftctl.yaml`:

```yaml
database: driftctl.db
api:
  addr: ":8080"
workspaces:
  - name: prod
    provider: aws
    state:
      backend: s3
      bucket: my-tf-state
      key: prod/terraform.tfstate
      region: us-east-1
    regions: [us-east-1]
    schedule:
      cron: "0 */6 * * *"
```

## REST API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/workspaces` | List workspaces |
| POST | `/api/v1/workspaces` | Create workspace |
| POST | `/api/v1/workspaces/{id}/scans` | Trigger scan |
| GET | `/api/v1/scans` | List recent scans |
| GET | `/api/v1/scans/{id}/report` | Get drift report (`?format=json\|table`) |
| PUT | `/api/v1/workspaces/{id}/schedules` | Set cron schedule |

Optional API key via `X-API-Key` header when `api.api_key` is set in config.

## Extending

Add a cloud provider by implementing `providers.CloudProvider` in `internal/providers/` and registering it in `providers.DefaultRegistry()`.

Add resource types by extending the state extractor's compare keys and the provider fetch logic.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
