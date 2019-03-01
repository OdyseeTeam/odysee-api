# LBRY on the Web

The LBRY experience, in the comfort of your own browser.

Active development is in progress, expect failing tests and breaking changes.

## Running with Docker (easy)

Make sure you have Docker and docker-compose installed.

Then run:

```
docker-compose up app
```

_Warning: this will pull and launch lbrynet image, which lbryweb requires to operate_

After everything is done launching, open `http://localhost:8080` in your browser.

## Development

This allows modifying `bundle.js` locally and seeing changes immediately without having to restart or re-build the `app` image.
Set `LBRY_DESKTOP_BUILD` to wherever your bundle.js is (ex. `~/Repos/lbry-desktop/dist/web`), then run:

```
docker-compose up dev-app
```

#### If you would like to run off the source

You're still going to need lbrynet, so prefix your `go` commangs with docker stuff.

```
docker-compose up lbrynet --no-start
docker-compose start lbrynet && export LW_LBRYNET=http://localhost:5579/
go mod download
go generate ./...
make embed
LW_DEBUG=1 go run *.go serve
```

## Testing

Make sure you got `lbrynet` container running and run `make test`.

## Modifying and building a Docker image

First, make sure you have Go 1.11+

- Ubuntu: https://launchpad.net/~longsleep/+archive/ubuntu/golang-backports or https://github.com/golang/go/wiki/Ubuntu
- OSX: `brew install go`

Then build the binary, create a docker image locally and run off it:

```
make image && docker-compose up app
```

## Contributing

Contributions to this project are welcome, encouraged, and compensated. For more details, see [lbry.io/faq/contributing](https://lbry.io/faq/contributing).

Please ensure that your code builds (using `./build.sh`) before pushing your branch. You must `go fmt` your code before you commit it, or the build will fail.


## License

This project is MIT licensed. For the full license, see [LICENSE](LICENSE).


## Security

We take security seriously. Please contact security@lbry.io regarding any issues you may encounter.
Our PGP key is [here](https://keybase.io/lbry/key.asc) if you need it.


## Contact

The primary contact for this project is [@lyoshenka](https://github.com/lyoshenka) (grin@lbry.io).

