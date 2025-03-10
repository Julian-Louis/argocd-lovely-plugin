.PHONY:	all clean code-vet code-fmt test get docker variations generate code-check

DEPS := $(shell find . -type f -name "*.go" -printf "%p ")
IMAGE_REPO := ghcr.io/crumbhole
BASE_LOVELY_IMAGE := argocd-lovely-plugin-cmp
all: code-vet code-fmt test build/argocd-lovely-plugin

docker: plugin_versioned.yaml
	docker build . -t ${IMAGE_REPO}/${BASE_LOVELY_IMAGE}:${LOVELY_VERSION}

variations: docker
	BASE_LOVELY_IMAGE=${BASE_LOVELY_IMAGE} IMAGE_REPO=${IMAGE_REPO} LOVELY_VERSION=${LOVELY_VERSION} variations/variations.sh

clean:
	$(RM) -rf build

get: $(DEPS)
	go get ./...

test: get
	go test ./...

test_verbose: get
	go test -v ./...

generate: config.md .github/actions/variations/action.yaml

code-check: generate code-fmt
	git diff --exit-code

.github/actions/variations/action.yaml: variations/variationActions.sh variations/variations.txt
	BASE_LOVELY_IMAGE=${BASE_LOVELY_IMAGE} $<

config.md plugin.yaml &: build/generator
	$<

plugin_versioned.yaml: plugin.yaml
	yq e '.spec.version |= "${LOVELY_VERSION}"' < $< > $@

update:
	go get -u ./...
	go mod tidy

build/argocd-lovely-plugin: $(DEPS) get
	mkdir -p build
	CGO_ENABLED=0 go build -buildvcs=false -o build ./cmd/argocd-lovely-plugin/.

build/generator: $(DEPS) get
	mkdir -p build
	CGO_ENABLED=0 go build -buildvcs=false -o build ./cmd/generator/.

code-vet: $(DEPS) get
## Run go vet for this project. More info: https://golang.org/cmd/vet/
	@echo go vet
	go vet $$(go list ./... )

code-fmt: $(DEPS) get
## Run go fmt for this project
	@echo go fmt
	go fmt $$(go list ./... )

lint: $(DEPS) get
	golangci-lint run

coverage: $(DEPS) get
	go test -v ./... -coverpkg=./... -coverprofile=coverage.out
	go tool cover -html=coverage.out
