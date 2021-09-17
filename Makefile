#!make
include envfile

# This version-strategy uses git tags to set the version string
VERSION := $(shell git describe --tags --always --dirty)

ifeq (${PROD}, true)
	# do nothing
else ifeq (${DEV}, true)
	VERSION := ${VERSION}-dev
else ifeq (${HOTFIX}, true)
	VERSION := ${VERSION}-hotfix
else
	VERSION := ${VERSION}-local
endif

ALL_ARCH := amd64 arm arm64 ppc64le

# Set default base image dynamically for each arch
ifeq ($(ARCH),amd64)
    BASEIMAGE?=alpine
endif
ifeq ($(ARCH),arm)
    BASEIMAGE?=armel/busybox
endif
ifeq ($(ARCH),arm64)
    BASEIMAGE?=aarch64/busybox
endif
ifeq ($(ARCH),ppc64le)
    BASEIMAGE?=ppc64le/busybox
endif

IMAGE := $(REGISTRY)/$(BIN)

BUILD_IMAGE ?= golang:1.11-alpine

# If you want to build all binaries, see the 'all-build' rule.
# If you want to build all containers, see the 'all-container' rule.
# If you want to build AND push all containers, see the 'all-push' rule.
all: build

build-%:
	@$(MAKE) --no-print-directory ARCH=$* build

container-%:
	@$(MAKE) --no-print-directory ARCH=$* container

push-%:
	@$(MAKE) --no-print-directory ARCH=$* push

all-build: $(addprefix build-, $(ALL_ARCH))

all-container: $(addprefix container-, $(ALL_ARCH))

all-push: $(addprefix push-, $(ALL_ARCH))

build: bin/$(ARCH)/$(BIN)

bin/$(ARCH)/$(BIN): build-dirs
	@echo "building: $@"
	 ARCH=$(ARCH)       \
	 VERSION=$(VERSION) \
	 PKG=$(PKG)         \
	 ./build/build.sh   \

DOTFILE_IMAGE = $(subst :,_,$(subst /,_,$(IMAGE))-$(VERSION))

container: .container-$(DOTFILE_IMAGE) container-name
.container-$(DOTFILE_IMAGE): build Dockerfile.in
	@sed \
	    -e 's|ARG_BIN|$(BIN)|g' \
	    -e 's|ARG_ARCH|$(ARCH)|g' \
	    -e 's|ARG_FROM|$(BASEIMAGE)|g' \
	    Dockerfile.in > .dockerfile-$(ARCH)
	@docker build -t $(IMAGE):$(VERSION) -f .dockerfile-$(ARCH) .
	@docker images -q $(IMAGE):$(VERSION) > $@

container-name:
	@echo "container: $(IMAGE):$(VERSION)"

push: .push-$(DOTFILE_IMAGE) push-name
.push-$(DOTFILE_IMAGE): .container-$(DOTFILE_IMAGE)
	@docker push $(IMAGE):$(VERSION)
	@docker images -q $(IMAGE):$(VERSION) > $@

push-name:
	@echo "pushed: $(IMAGE):$(VERSION)"

run: build # make ARGS="hello these are my args" run
	CONFIG="file::./dev_cfg/local-cfg.yml" \
	./bin/$(ARCH)/$(BIN) ${ARGS}

run-container: container
	docker run -v "$(PWD)/dev_cfg:/cfg" -e CONFIG=file::/cfg/cfg.yml ${IMAGE}:${VERSION}

version:
	@echo $(VERSION)

install-tools:
	./build/install-tools.sh

env:
	env

test: build-dirs
	AP_CFG="file::../../../testdata/cfg.yml" \
	./build/test.sh $(SRC_DIRS)

lint: lint-all

lint-all:
	revive -config revive.toml -formatter friendly -exclude vendor/... ./...

lint-changed:
	revive -config revive.toml -formatter friendly -exclude vendor/... $(CHANGED_GO_FILES)

lint-staged:
	revive -config revive.toml -formatter friendly -exclude vendor/... $(STAGED_GO_FILES)

mods:
	GO111MODULE=on go mod verify

build-dirs:
	@mkdir -p bin/$(ARCH)
	@mkdir -p .go/src/$(PKG) .go/pkg .go/bin .go/std/$(ARCH)

clean: container-clean bin-clean

container-clean:
	rm -rf .container-* .dockerfile-* .push-*

bin-clean:
	rm -rf .go bin

azure-deploy: push
	az container create --resource-group lunde-rsc --name lunde --image ${IMAGE}:${VERSION}
