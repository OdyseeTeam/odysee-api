VERSION := $(shell git describe --tags)

.PHONY: prepare_test
prepare_test:
	docker-compose up --no-start test_lbrynet
	docker cp conf/daemon_settings.yml $(shell docker-compose ps -q test_lbrynet):/storage/data/daemon_settings.yml
	docker-compose start test_daemon

.PHONY: test
test:
	go test -cover ./...

.PHONY: test_race
test_race:
	go test -race -gcflags=all=-d=checkptr=0 ./...

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

.PHONY: image
image:
	docker build -t lbry/lbrytv:$(VERSION) -t lbry/lbrytv:latest .

.PHONY: dev_image
dev_image:
	docker build -t lbry/lbrytv:$(VERSION) -t lbry/lbrytv:latest-dev .

.PHONY: publish_image
publish_image:
	docker push lbry/lbrytv:$(VERSION)

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
