date := $(shell date "+%Y-%m-%d-%H-%M")
watchman_version := $(shell git describe --tags --match 'watchman-v*'|sed -e 's/.*\-v//')
git_hash := $(shell git rev-parse --short HEAD)

.PHONY: test
test:
	go test -cover ./...

.PHONY: test_race
test_race:
	go test -race -gcflags=all=-d=checkptr=0 ./...

prepare_test:
	go get golang.org/x/tools/cmd/cover
	go get github.com/mattn/goveralls
	go run . db_migrate_up
	go run ./apps/collector db_migrate_up

.PHONY: test_circleci
test_circleci:
	scripts/wait_for_wallet.sh
	go get golang.org/x/tools/cmd/cover
	go get github.com/mattn/goveralls
	go run . db_migrate_up
	go test -covermode=count -coverprofile=coverage.out ./...
	goveralls -coverprofile=coverage.out -service=circle-ci -ignore=models/ -repotoken $(COVERALLS_TOKEN)

release:
	GO111MODULE=on goreleaser --rm-dist

snapshot:
	GO111MODULE=on goreleaser --snapshot --rm-dist

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

app_path := ./apps/collector
.PHONY: collector_models
collector_models: get_sqlboiler
	sqlboiler --no-tests --add-global-variants --wipe psql --no-context -o $(app_path)/models -c $(app_path)/sqlboiler.toml
	# So sqlboiler can discover their sqlboiler.toml config files instead of reaching for the one in the root
	# find . -name boil_main_test.go|xargs sed -i '' -e 's/outputDirDepth = 3/outputDirDepth = 1/g'

GORELEASER_CURRENT_TAG := $(shell git describe --tags --match 'collector-v*'|sed -e 's/.*\-v//')
collector:
	goreleaser build -f apps/collector/.goreleaser.yml --snapshot --rm-dist
	find . -name pkged.go -delete

watchman:
	GOARCH=amd64 GOOS=linux go build \
		-o apps/watchman/dist/linux_amd64/watchman \
		-ldflags "-X github.com/lbryio/lbrytv/version.version=$(watchman_version) -X github.com/lbryio/lbrytv/version.commit=$(git_hash) -X github.com/lbryio/lbrytv/apps/version.buildDate=$(date)" \
		./apps/watchman/cmd/watchman/

watchman_image:
	docker build -t lbry/odysee-watchman:$(watchman_version) ./apps/watchman

watchman_models:
	sqlc -f apps/watchman/sqlc.yaml generate

watchman_api:
	goa gen github.com/lbryio/lbrytv/apps/watchman/design -o apps/watchman

watchman_example:
	goa example github.com/lbryio/lbrytv/apps/watchman/design -o apps/watchman
