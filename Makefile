VER?=0.0.1
MODULES=$(shell find . -mindepth 2 -maxdepth 4 -type f -name 'go.mod' | cut -c 3- | sed 's|/[^/]*$$||' | sort -u | tr / :)
targets=$(addprefix test-, $(MODULES))

all:
	$(MAKE) $(targets)

tidy-%:
	cd $(subst :,/,$*); go mod tidy

fmt-%:
	cd $(subst :,/,$*); go fmt ./...

vet-%:
	cd $(subst :,/,$*); go vet ./...

test-%: tidy-% fmt-% vet-%
	cd $(subst :,/,$*); go test ./... -coverprofile cover.out

release-%:
	$(eval REL_PATH=$(subst :,/,$*))
	@if ! test -f $(REL_PATH)/go.mod; then echo "Missing ./$(REL_PATH)/go.mod, terminating release process"; exit 1; fi
	git checkout main
	git pull
	git tag "$(REL_PATH)/v$(VER)"
	git push origin "$(REL_PATH)/v$(VER)"
