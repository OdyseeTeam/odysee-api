# LBRY on the Web

The LBRY experience, in the comfort of your own browser.

Active development is in progress, expect failing tests and breaking changes.

[![CircleCI](https://img.shields.io/circleci/project/github/lbryio/lbrytv/master.svg)](https://circleci.com/gh/lbryio/lbrytv/tree/master) [![Coverage](https://img.shields.io/coveralls/github/lbryio/lbrytv.svg)](https://coveralls.io/github/lbryio/lbrytv)

## Running with Docker

This is the recommended method for frontend development.

Make sure you have recent enough Docker and `docker-compose` installed.

**1. Initialize and launch the containers**

This will pull and launch SDK and postgres images, which lbrytv requires to operate.

`docker-compose up app`

*Note: if you're running a LBRY desktop app or lbrynet instance, you will have to either shut it down or change ports*

**2. Setup up the database schema if this is your first launch**

`docker-compose run app ./lbrytv db_migrate_up`

**3. Clone [lbry-desktop](https://github.com/lbryio/lbry-desktop/) repo, if you don't have it**

```
cd ..
git clone git@github.com:lbryio/lbry-desktop.git
```

**4. Launch UI in lbry-desktop repo folder**

```
SDK_API_URL=http://localhost:8080/api/v1/proxy yarn dev:web
```

**5. Open http://localhost:8081/ in Chrome**

## Running off the source (if you want to modify things)

You still might want to use `docker` and `docker-compose` for running SDK and DB containers.

**1. Initialize and launch the containers**

If this is your first launch, initialize:

`docker-compose up --no-start postgres lbrynet`

After that, launch the containers:

`docker-compose start postgres lbrynet`

*Note: if you're running a LBRY desktop app or lbrynet instance, you will have to either shut it down or change ports*

**2. Setup up the database schema if this is your first launch**

`go run . db_migrate_up`

**3. Start lbrytv API server**

`go run . serve`

**4. Clone [lbry-desktop](https://github.com/lbryio/lbry-desktop/) repo, if you don't have it**

```
cd ..
git clone git@github.com:lbryio/lbry-desktop.git
```

**5. Launch UI in lbry-desktop repo folder**

```
SDK_API_URL=http://localhost:8080/api/v1/proxy yarn dev:web
```

**6. Open http://localhost:8081/ in Chrome**

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

