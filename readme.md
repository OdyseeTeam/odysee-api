# LBRY on the Web

The LBRY experience, in the comfort of your own browser.

Active development is in progress, expect failing tests and breaking changes.

[![CircleCI](https://img.shields.io/circleci/project/github/lbryio/lbrytv/master.svg)](https://circleci.com/gh/lbryio/lbrytv/tree/master) [![Coverage](https://img.shields.io/coveralls/github/lbryio/lbrytv.svg)](https://coveralls.io/github/lbryio/lbrytv)

## Running with Docker (if you want to try things)

Make sure you have Docker and docker-compose installed.

1. Run `docker-compose up app` (this will pull and launch SDK and postgres images, which lbrytv requires to operate)
2. `git clone` [lbry-desktop](https://github.com/lbryio/lbry-desktop/) repo in a separate folder
3. Run `SDK_API_URL=http://localhost:8080/api/proxy/ yarn dev:web` in lbry-desktop repo
4. Open http://localhost:8081/ in Chrome

After everything is done launching, open `http://localhost:8080` in your browser.

## Running off the source (if you want to modify things)

You're still going to need docker / docker-compose for running SDK and DB containers.

1. Run `docker-compose up postgres lbrynet  --no-start` (if this is your first launch)
2. Run `docker-compose start postgres lbrynet` to launch containers
3. Run `go run . serve` to start lbrytv API server
4. `git clone` [lbry-desktop](https://github.com/lbryio/lbry-desktop/) repo in a separate folder
5. Run `SDK_API_URL=http://localhost:8080/api/proxy/ yarn dev:web` in lbry-desktop repo
6. Open http://localhost:8081/ in Chrome

## Testing

Make sure you got `lbrynet` and `postgres` containers running and run `make test`.

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

Please ensure that your code builds and automated tests run successfully before pushing your branch. You must `go fmt` your code before you commit it, or the build will fail.


## License

This project is MIT licensed. For the full license, see [LICENSE](LICENSE).


## Security

We take security seriously. Please contact security@lbry.io regarding any issues you may encounter.
Our PGP key is [here](https://keybase.io/lbry/key.asc) if you need it.


## Contact

The primary contact for this project is [@sayplastic](https://github.com/sayplastic) (andrey@lbry.com).

