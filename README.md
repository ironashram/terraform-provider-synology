# Terraform Provider Synology (ironashram fork)

An OpenTofu provider for managing Synology DSM through its web API. Built with the
[Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework) and
the [ironashram/go-synology](https://github.com/ironashram/go-synology) API client fork.

This is a hard fork of
[synology-community/terraform-provider-synology](https://github.com/synology-community/terraform-provider-synology).
It has diverged deliberately and no longer tracks upstream. Reasons:

- Validated against real DSM hardware (DSM 7.3.x), not a virtualized DSM. Several API
  payloads differ on real DSM 7 builds (e.g. reverse proxy entries have no `name` field)
  and this fork targets what DSM actually accepts.
- The container project resource received substantial fixes upstream never had:
  content-driven updates, working import, verbatim compose content, uploaded-file drift
  detection, and correct no-op update handling.
- The go-synology client fork handles Container Manager compose stream responses and DSM's
  inconsistent error envelopes (array vs object `errors`, bogus OTP detection).
- Selected features from the
  [florianehmke fork](https://github.com/florianehmke/terraform-provider-synology-dsm)
  are imported and adapted for DSM 7.3: reverse proxy rules, certificate import and
  service binding, LDAP/OIDC SSO client config, NFS share privileges, reworked task
  scheduler plus one-shot task runs, and a File Station path data source.
- Acceptance tests are gated behind `TF_ACC=1` and run against a real NAS. CI runs unit
  tests only.

Published on the OpenTofu registry as `ironashram/synology`. Versions are tagged
`vX.Y.Z-ironashram`.

## Usage

```hcl
terraform {
  required_providers {
    synology = {
      source  = "ironashram/synology"
      version = "0.8.0-ironashram"
    }
  }
}

provider "synology" {
  host     = "https://your-synology:5001"
  user     = "admin"
  password = var.synology_password
}

resource "synology_container_project" "app" {
  name       = "my-app"
  share_path = "/docker/my-app"
  run        = true
  content    = file("${path.module}/compose.yaml")

  configs = {
    env = {
      name    = "env"
      file    = ".env"
      content = "FOO=bar\n"
    }
  }
}
```

Provider credentials can also come from `SYNOLOGY_HOST`, `SYNOLOGY_USER`,
`SYNOLOGY_PASSWORD`, `SYNOLOGY_OTP_SECRET` and `SYNOLOGY_SKIP_CERT_CHECK` environment
variables. Session caching (`session_cache`) and TOTP handling are built in.

Full resource documentation lives in [docs/](./docs). Examples are under
[examples/](./examples).

## Feature overview

- Container Manager: compose projects (verbatim `content` or structured schema), networks,
  container operation actions. Uploaded config files (e.g. `.env`) are re-read on refresh,
  so manual edits on the NAS surface as drift and heal on apply.
- Core: packages and package feeds, task scheduler and one-shot task runs, events,
  reverse proxy rules, certificate import / service binding / data source, Directory
  Server OIDC SSO client, NFS share privileges.
- File Station: files, folders, ISO and cloud-init image generation, path data source.
- Virtual Machine Manager: guests, images, guest data sources.
- Generic `synology_api` resource for any other DSM endpoint.

## Development

```
go build ./...
go test ./synology/...
```

Acceptance tests need a real DSM reachable via `SYNOLOGY_HOST`/`SYNOLOGY_USER`/
`SYNOLOGY_PASSWORD` and are opt-in:

```
TF_ACC=1 go test ./synology/provider/... -v -timeout 30m
```

Docs are generated with `go generate ./...` (requires a terraform binary on PATH).

## Release

Tag `vX.Y.Z-ironashram` on `main` and push. goreleaser builds and signs the release, the
registry picks it up from GitHub releases. Publish the release draft immediately: the
OpenTofu registry scanner records a version error if it sees the tag while the release is
still a draft.

## License

Mozilla Public License 2.0, see [LICENSE](LICENSE).
