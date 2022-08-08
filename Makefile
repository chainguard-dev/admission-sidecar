GIT_TAG ?= $(shell git describe --tags --always --dirty)
GIT_HASH ?= $(shell git rev-parse HEAD)
LDFLAGS=""

KO_DOCKER_REPO ?= ghcr.io/chainguard-dev/admission-sidecar

.PHONY: ko-resolve
ko-resolve:
	LDFLAGS="$(LDFLAGS)" \
	ko resolve --tags $(GIT_TAG),latest --base-import-paths --recursive \
	--filename ./config --platform=all \
	--image-refs imagerefs > release-$(GIT_TAG).yaml

### Release

.PHONY: goreleaser
goreleaser:
	GIT_TAG="$(GIT_TAG)" GIT_HASH="$(GIT_HASH)" LDFLAGS="$(LDFLAGS)" \
	goreleaser release --rm-dist

.PHONY: release-images
release-images: ko-resolve sign-images

.PHONY: sign-images
sign-images:
	./scripts/sign-images.sh

### Testing

.PHONY: ko-apply
ko-apply:
	LDFLAGS="$(LDFLAGS)" \
	ko apply -BRf ./config/
