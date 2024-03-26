date := $(shell date "+%Y-%m-%d-%H-%M")
git_hash := $(shell git rev-parse --short HEAD)
oapi_version ?= $(shell git describe --tags --match 'api-v*'|sed 's/api-v\([0-9.]*\)/\1/')
watchman_version := $(shell git describe --tags --match 'watchman-v*'|sed 's/api-v\([0-9.]*\)/\1/')
forklift_version ?= $(shell git describe --tags --match 'forklift-v*'|sed 's/forklift-v\([0-9.]*\).*/\1/')
uploads_version ?= $(shell git describe --tags --match 'uploads-v*'|sed 's/uploads-v\([0-9.]*\).*/\1/')
go ?= go

.PHONY: test
test:
	$(go) test -cover ./...

.PHONY: test_race
test_race:
	$(go) test -race -gcflags=all=-d=checkptr=0 ./...

prepare_test:
	cd tools && \
		$(go) install $($(go) list -tags tools -f '{{range $_, $p := .Imports}}{{$p}} {{end}}')
	$(go) run . db_migrate_up

.PHONY: test_circleci
test_circleci:
	scripts/wait_for_wallet.sh
	cd tools &&\
		$(go) install $($(go) list -tags tools -f '{{range $_, $p := .Imports}}{{$p}} {{end}}')
	$(go) run . db_migrate_up
	$(go) test -covermode=count -coverprofile=coverage.out ./...
	goveralls -coverprofile=coverage.out -service=circle-ci -ignore=models/ -repotoken $(COVERALLS_TOKEN)

.PHONY: clean
clean:
	rm -rf ./dist

.PHONY: server
server:
	LW_DEBUG=1 $(go) run .

tag := $(shell git describe --abbrev=0 --tags)
.PHONY: retag
retag:
	@echo "Re-setting tag $(tag) to the current commit"
	git push origin :$(tag)
	git tag -d $(tag)
	git tag $(tag)

get_sqlboiler:
	$(go) install github.com/volatiletech/sqlboiler
	$(go) install github.com/volatiletech/sqlboiler/drivers/sqlboiler-psql

.PHONY: models
models: get_sqlboiler
	sqlboiler --add-global-variants --wipe psql --no-context --no-tests

.PHONY: oapi
oapi:
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux $(go) build \
		-o dist/linux_amd64/oapi \
		-ldflags "-s -w -X github.com/OdyseeTeam/odysee-api/version.version=$(oapi_version) -X github.com/OdyseeTeam/odysee-api/version.commit=$(git_hash) -X github.com/OdyseeTeam/odysee-api/apps/version.buildDate=$(date)" \
		.

watchman:
	GOARCH=amd64 GOOS=linux $(go) build \
		-o apps/watchman/dist/linux_amd64/watchman \
		-ldflags "-s -w -X github.com/OdyseeTeam/odysee-api/version.version=$(watchman_version) -X github.com/OdyseeTeam/odysee-api/version.commit=$(git_hash) -X github.com/OdyseeTeam/odysee-api/apps/version.buildDate=$(date)" \
		./apps/watchman/cmd/watchman/

forklift:
	GOARCH=amd64 GOOS=linux $(go) build \
		-o dist/linux_amd64/forklift \
		-ldflags "-s -w -X github.com/OdyseeTeam/odysee-api/version.version=$(forklift_version) \
		-X github.com/OdyseeTeam/odysee-api/version.commit=$(git_hash) \
		-X github.com/OdyseeTeam/odysee-api/apps/version.buildDate=$(date)" \
		./apps/forklift/cmd/

uploads:
	GOARCH=amd64 GOOS=linux $(go) build \
		-o dist/linux_amd64/uploads \
		-ldflags "-s -w -X github.com/OdyseeTeam/odysee-api/version.version=$(uploads_version) \
		-X github.com/OdyseeTeam/odysee-api/version.commit=$(git_hash) \
		-X github.com/OdyseeTeam/odysee-api/apps/version.buildDate=$(date)" \
		./apps/uploads/cmd/

watchman_design:
	goa gen github.com/OdyseeTeam/odysee-api/apps/watchman/design -o apps/watchman

watchman_example:
	goa example github.com/OdyseeTeam/odysee-api/apps/watchman/design -o apps/watchman

cur_branch := $(shell git rev-parse --abbrev-ref HEAD)
.PHONY: oapi_image
oapi_image:
	docker buildx build -t odyseeteam/odysee-api:$(oapi_version) -t odyseeteam/odysee-api:latest -t odyseeteam/odysee-api:$(cur_branch) --platform linux/amd64 .

.PHONY: watchman_image
watchman_image:
	docker buildx build -t odyseeteam/watchman:$(watchman_version) --platform linux/amd64 ./apps/watchman

.PHONY: uploads_image
uploads_image:
	docker buildx build -t odyseeteam/uploads:$(uploads_version) --platform linux/amd64 -f ./build/uploads/Dockerfile .

.PHONY: forklift_image
forklift_image:
	docker buildx build -t odyseeteam/forklift:$(forklift_version) --platform linux/amd64 -f ./build/forklift/Dockerfile .
