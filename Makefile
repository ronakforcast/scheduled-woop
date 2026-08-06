.PHONY: all test build image helm-check helm-package

all: test build helm-check

test:
	go test -race ./...

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/scheduled-woop ./cmd/scheduled-woop

image:
	docker build -t scheduled-woop:dev .

helm-check:
	helm lint charts/scheduled-woop
	helm template test charts/scheduled-woop --namespace woop-scheduler-system >/dev/null

helm-package: helm-check
	mkdir -p bin
	helm package charts/scheduled-woop --destination bin
