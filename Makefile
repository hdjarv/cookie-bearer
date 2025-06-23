APP_NAME = cookie-bearer

TAG = $(shell git describe --tags --exact-match 2>/dev/null)
GIT_COMMIT = $(shell git rev-parse --short HEAD)
BUILD_DATE = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MODIFIED = $(shell test -n "$$(git status --porcelain)" && echo "-modified")

ifeq ($(strip $(TAG)),)
	VERSION = $(GIT_COMMIT)
else
	VERSION = $(TAG)
endif

ifneq ($(strip $(MODIFIED)),)
	VERSION := $(VERSION)$(MODIFIED)
endif

LDFLAGS = -X main.version="$(VERSION)" \
          -X main.buildDate="$(BUILD_DATE)" \
          -X main.gitCommit="$(GIT_COMMIT)"

.PHONY: all build clean docker docker-clean

all: build

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(APP_NAME)

clean:
	rm -f $(APP_NAME)

docker:
	docker build \
		--build-arg VERSION="$(VERSION)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		--build-arg GIT_COMMIT="$(GIT_COMMIT)" \
		-t $(APP_NAME):$(VERSION) .

docker-clean:
	docker rmi $(APP_NAME):$(VERSION)
