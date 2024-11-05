# API Server for Odysee

⚠️ If you are looking for the UI code, that lives here: https://github.com/OdyseeTeam/odysee-frontend. ⚠️

This is the API server used by [Odysee](https://odysee.com), mostly acting as a bridge between web and the blockchain.

![Tests](https://github.com/OdyseeTeam/odysee-api/actions/workflows/test-release.yml/badge.svg) [![Coverage Status](https://coveralls.io/repos/github/OdyseeTeam/odysee-api/badge.svg?branch=master)](https://coveralls.io/github/OdyseeTeam/odysee-api?branch=master)

## Required tools

```
go install github.com/volatiletech/sqlboiler
go install github.com/volatiletech/sqlboiler/drivers/sqlboiler-psql

```

## Running off source (the preferred way for development)

**1. Launch the core environment containers**

`docker compose up -d`

*Note: if you're running a LBRY desktop app or a lbrynet instance, you will have to either shut it down or change its ports*

**2. Set up database schema if this is your first launch**

`go run . db_migrate_up`

**3. Generate RSA key file**

`ssh-keygen -t rsa -f token_privkey.rsa -m pem`

**4. Start Odysee API server**

`go run .`

**5. Clone [odysee-frontend](https://github.com/OdyseeTeam/odysee-frontend) repo, if you don't have it**

**6. Launch UI in lbry-desktop repo folder**

```
SDK_API_URL=http://localhost:8080 yarn dev:web
```

## Running with Docker

Make sure you have recent enough Docker and `docker compose` installed.

**1. Initialize and launch the containers**

This will pull and launch SDK and postgres images, which Odysee API requires to operate.

`docker compose -f docker-compose.yml -f docker compose.app.yml up -d`

*Note: if you're running a LBRY desktop app or lbrynet instance, you will have to either shut it down or change ports*

**2. Clone [lbry-desktop](https://github.com/lbryio/lbry-desktop/) repo, if you don't have it**

```
cd ..
git clone git@github.com:lbryio/lbry-desktop.git
```

**3. Launch UI in lbry-desktop repo folder**

```
LBRY_WEB_API=http://localhost:8080 yarn dev:web
```

**4. Open http://localhost:9090/ in Chrome or Firefox for best experience**

## Development tips

### Change DB schema

1. Create a schema migration
2. Apply schema migration with `go run . db_migrate_up`
3. Run `make models` for sqlboiler to pick up the changes

## Testing

Make sure you have `lbrynet`, `postgres` and `postgres-collector` containers running and run `make prepare_test test`.

## Building Docker images

Make sure you have Go 1.23 installed.

Then build the binary, create a docker image locally:

```
make oapi && make oapi_image
```

## Tools used in development

* [golangci-lint](https://golangci-lint.run/welcome/install/#local-installation)

* govulncheck

```
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## Versioning

This project is using [CalVer](https://calver.org) YY.MM.MINOR since February 2021 (SemVer prior to that)

## Contributing

Contributions to this project are welcome.

Please ensure that your code builds and automated tests run successfully before pushing your branch. You must `go fmt` your code before you commit it, or the build will fail.


## License

This project is MIT licensed. For the full license, see [LICENSE](LICENSE).


## Security

We take security seriously. Please contact security@lbry.io regarding any issues you may encounter.
Our PGP key is [here](https://lbry.com/faq/pgp-key) if you need it.


## Contact

The primary contact for this project is [@anbsky](https://github.com/anbsky) (andrey.beletsky@odysee.com).

