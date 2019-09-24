# Author  <dorzheho@cisco.com>

include apphc.mk

APP_NAME        := controller
DOCKER_REGISTRY ?= dockerhub.cisco.com/intucell-son-dev-docker
DOCKER_REPO     ?= apphoster/$(APP_NAME)
TARGETS         := darwin/amd64 linux/amd64 windows/amd64
DIST_DIRS       := find * -type d -exec
VERSION         ?= 1.0.0

# go option
GO        ?= go
TAGS      := kqueue
TESTS     := .
TESTFLAGS :=
LDFLAGS   :=
GOFLAGS   :=
GOXFLAGS  :=
BINDIR    := $(CURDIR)/bin
BINARIES  := apphcd

GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_SHA := $(shell git rev-parse --short HEAD)
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null)
GIT_DIRTY := $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")
SHELL := /bin/bash
SED := /usr/local/bin/gsed

ifdef GIT_TAG
  VERSION = $(GIT_TAG)
else
  GIT_TAG = $(VERSION)
endif

LDFLAGS += -X cisco.com/son/apphcd/internal/app/grpc/apphcmanager/version.Release=$(GIT_TAG)
LDFLAGS += -X cisco.com/son/apphcd/internal/app/grpc/apphcmanager/version.GitCommit=$(GIT_COMMIT)
LDFLAGS += -X cisco.com/son/apphcd/internal/app/grpc/apphcmanager/version.GitTreeState=$(GIT_DIRTY)


# usage: make clean compile dist  VERSION=v1.0.0
.PHONY: compile
compile: LDFLAGS += -extldflags "-static"
compile:
	CGO_ENABLED=0 gox -output="_dist/{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' $(GOXFLAGS) cisco.com/son/apphcd/

.PHONY: build
build: clean compile docker-build lcm-build

.PHONY: deploy
deploy: clean compile docker-build docker-push helm-build lcm-build apphc-deploy

.PHONY: upgrade
upgrade: clean compile docker-build docker-push helm-build lcm-build apphc-upgrade

.PHONY: docker-build
docker-build:
	rm -rf _dist/docker/staging
	mkdir -p _dist/docker/staging
	cp Dockerfile _dist/docker
	cp _dist/linux-amd64/apphcd _dist/docker/staging/apphcd
	docker build _dist/docker -t $(DOCKER_REGISTRY)/$(DOCKER_REPO):$(VERSION)

.PHONY: docker-push
docker-push:
	docker push $(DOCKER_REGISTRY)/$(APP_NAME):$(VERSION)

.PHONY: lcm-build
lcm-build:
	rm -rf _dist/controller 
	mkdir -p _dist/controller 
	cp -a deployment/controller/* _dist/controller

.PHONY: helm-deploy
apphc-deploy:
	_dist/controller/scripts/install.sh $(DOCKER_REGISTRY) $(APPHC_ADAPTERS_RANCHER_SERVER_ENDPOINT) $(SVCS_URL_EXTERNAL)


.PHONY: helm-upgrade
apphc-upgrade:
	helm-build
	rm -f _dist/controller/helm/values.yaml
	helm upgrade _dist/controller/helm --name $(DEPLOYMENT_NAME) --set image.tag=$(VERSION) --debug

#.PHONY: info
info:
	@echo "Registry:     $(DOCKER_REGISTRY)"
	@echo "Image:        $(IMAGE_NAME)"
	@echo "Build tag:    $(VERSION)"
	@echo "GIT_COMMIT:   $(GIT_COMMIT)"
	@echo "GIT_SHA:      $(GIT_SHA)" 
	@echo "GIT_TAG:      $(GIT_DIRTY)" 

.PHONY: dist
dist:
	( \
		cd _dist && \
		$(DIST_DIRS) cp ../LICENSE {} \; && \
		$(DIST_DIRS) cp ../README.md {} \; && \
		$(DIST_DIRS) tar -zcf apphcd-${VERSION}-{}.tar.gz {} \; && \
		$(DIST_DIRS) zip -r apphcd-${VERSION}-{}.zip {} \; \
	)

.PHONY: upack
upack: lcm-build \
       	rm -rf _dist/components
	mkdir _dist/components
	cp -a _dist/controller _dist/components
	rm -f _dist/upgrade.tgz
	tar -C  _dist/components -cfvz _dist/upgrade.tgz  

.PHONY: checksum
checksum:
	for f in _dist/*.{gz,zip} ; do \
		shasum -a 256 "$${f}"  | awk '{print $$1}' > "$${f}.sha256" ; \
	done

.PHONY: clean
clean:
	rm -rf _dist/

.PHONY: test

HAS_GOMETALINTER := $(shell command -v gometalinter;)
HAS_DEP := $(shell command -v dep;)
HAS_GOX := $(shell command -v gox;)
HAS_GIT := $(shell command -v git;)
HAS_BINDATA := $(shell command -v go-bindata;)

.PHONY: bootstrap
bootstrap:
ifndef HAS_GOMETALINTER
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install
endif
ifndef HAS_DEP
	go get -u github.com/golang/dep/cmd/dep
endif
ifndef HAS_GOX
	go get -u github.com/mitchellh/gox
endif
ifndef HAS_GIT
	$(error You must install git)
endif
ifndef HAS_BINDATA
	go get github.com/jteeuwen/go-bindata/...
endif
	dep ensure -v

