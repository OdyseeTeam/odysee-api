VERSION := $(shell git describe --tags)

.PHONY: prepare_test
prepare_test:
	# docker cp conf/test_daemon_settings.yml $(shell docker-compose ps -q test_daemon):/storage/data/daemon_settings.yml
	docker-compose up --no-start test_lbrynet
	docker cp conf/daemon_settings.yml $(shell docker-compose ps -q test_lbrynet):/storage/data/daemon_settings.yml
	docker-compose start test_daemon

.PHONY: test
test:
	go test ./...

release:
	goreleaser --rm-dist

snapshot:
	goreleaser --snapshot --rm-dist

.PHONY: image
image: snapshot
	docker build -t sayplastic/lbryweb-go:$(VERSION) .

embed:
	rice embed-go -i ./routes
