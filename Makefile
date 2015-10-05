name=ballot
container_name=wifast/$(name)
project_name=go-$(name)

changed=$(shell git diff --shortstat 2> /dev/null | tail -n1)
version=$(shell git tag --points-at HEAD | tail -n1)
branch=$(shell git rev-parse --abbrev-ref HEAD | tr -c "[[:alnum:]]\\n._-" "_")

export GOBIN=$(shell pwd)/bin
export GOPATH=$(shell pwd)/.go
org_url=github.com/WiFast
org_path=$(GOPATH)/src/$(org_url)
project_url=$(org_url)/$(project_name)
project_path=$(org_path)/$(project_name)

.PHONY: clean test static
all: $(GOBIN)/ballotd
static: $(GOBIN)/ballotd.static

clean:
	rm -rf $(GOBIN) $(GOPATH)

$(project_path):
	mkdir -p $(org_path)
	ln -sf $(shell pwd) $(project_path)

$(GOBIN)/ballotd: $(project_path)
	go get -d $(project_url)/cmd/ballotd
	go install -a $(project_url)/cmd/ballotd
$(GOBIN)/ballotd.static: $(project_path)
	go get -d $(project_url)/cmd/ballotd
	GOBIN=$(GOPATH)/bin go install -a -ldflags "-linkmode external -extldflags -static" $(project_url)/cmd/ballotd
	mkdir -p $(GOBIN)
	mv $(GOPATH)/bin/ballotd $(GOBIN)/ballotd.static

test: $(project_path)
	test -z "$(shell gofmt -s -l ./election ./heartbeat ./lock)"
	go vet ./cmd/ballotd ./election ./heartbeat ./lock
	go get -d -t ./cmd/ballotd ./election ./heartbeat ./lock
	go test -v -race ./cmd/ballotd ./election ./heartbeat ./lock
	#go test -v ./election

container:
	docker build -t $(container_name):latest .

push-tag:
ifeq "$(version)" ""
	$(info Commit is untagged, nothing to do)
else
ifneq "$(changed)" ""
	$(error Repo has changes)
endif
	git verify-tag $(version)
	docker tag -f $(container_name):latest $(container_name):$(version)
	docker push $(container_name):$(version)
endif

push-branch:
ifeq "$(branch)" "master"
	docker push $(container_name):latest
else
	docker tag -f $(container_name):latest $(container_name):$(branch)
	docker push $(container_name):$(branch)
endif

push: push-tag push-branch
