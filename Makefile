VER?=0.0.1
MODULES=$(shell find . -mindepth 2 -maxdepth 2 -type f -name 'go.mod' | cut -c 3- | sed 's|/[^/]*$$||' | sort -u)
targets = $(addprefix test-, $(MODULES))

all: $(targets)

tidy-%:
	cd $*; go mod tidy

fmt-%:
	cd $*; go fmt ./...

vet-%:
	cd $*; go vet ./...

test-%: tidy-% fmt-% vet-%
	cd $*; go test ./... -coverprofile cover.out

release-%:
	@if ! test -f $*/go.mod; then echo "Missing ./$*/go.mod, terminating release process"; exit 1; fi
	git checkout master
	git pull
	git tag "$*/v$(VER)"
	git push origin "$*/v$(VER)"
