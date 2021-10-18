date := $(shell date "+%Y-%m-%d-%H-%M")
api_version := $(shell git describe --tags --match 'api-v*'|sed -e 's/.*\-v//')
watchman_version := $(shell git describe --tags --match 'watchman-v*'|sed -e 's/.*\-v//')
git_hash := $(shell git rev-parse --short HEAD)

.PHONY: test
test:
	go test -cover ./...

.PHONY: test_race
test_race:
	go test -race -gcflags=all=-d=checkptr=0 ./...

prepare_test:
	cd tools && \
		go install $(go list -tags tools -f '{{range $_, $p := .Imports}}{{$p}} {{end}}')
	go run . db_migrate_up

.PHONY: test_circleci
test_circleci:
	scripts/wait_for_wallet.sh
	cd tools &&\
		go install $(go list -tags tools -f '{{range $_, $p := .Imports}}{{$p}} {{end}}')
	go run . db_migrate_up
	go test -covermode=count -coverprofile=coverage.out ./...
	goveralls -coverprofile=coverage.out -service=circle-ci -ignore=models/ -repotoken $(COVERALLS_TOKEN)

.PHONY: clean
clean:
	find . -name rice-box.go | xargs rm
	rm -rf ./dist

.PHONY: server
server:
	LW_DEBUG=1 go run .

tag := $(shell git describe --abbrev=0 --tags)
.PHONY: retag
retag:
	@echo "Re-setting tag $(tag) to the current commit"
	git push origin :$(tag)
	git tag -d $(tag)
	git tag $(tag)

get_sqlboiler:
	go get -u -t github.com/volatiletech/sqlboiler@v3.4.0
	go get -u -t github.com/volatiletech/sqlboiler/drivers/sqlboiler-psql@v3.4.0

.PHONY: models
models: get_sqlboiler
	sqlboiler --add-global-variants --wipe psql --no-context

.PHONY: api
api:
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build \
		-o dist/linux_amd64/lbrytv \
		-ldflags "-s -w -X github.com/lbryio/lbrytv/version.version=$(api_version) -X github.com/lbryio/lbrytv/version.commit=$(git_hash) -X github.com/lbryio/lbrytv/apps/version.buildDate=$(date)" \
		.

watchman:
	GOARCH=amd64 GOOS=linux go build \
		-o apps/watchman/dist/linux_amd64/watchman \
		-ldflags "-s -w -X github.com/lbryio/lbrytv/version.version=$(watchman_version) -X github.com/lbryio/lbrytv/version.commit=$(git_hash) -X github.com/lbryio/lbrytv/apps/version.buildDate=$(date)" \
		./apps/watchman/cmd/watchman/

watchman_image:
	docker build -t lbry/odysee-watchman:$(watchman_version) ./apps/watchman

watchman_design:
	goa gen github.com/lbryio/lbrytv/apps/watchman/design -o apps/watchman

watchman_example:
	goa example github.com/lbryio/lbrytv/apps/watchman/design -o apps/watchman
