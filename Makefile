GO_DIR := brosettlement-api/scripts/go

.PHONY: check validate test vet build

check: validate test vet build

validate:
	@test -f brosettlement-onboarding/SKILL.md
	@test -f brosettlement-api/SKILL.md
	@head -n 5 brosettlement-onboarding/SKILL.md | grep -q '^name: brosettlement-onboarding$$'
	@head -n 5 brosettlement-api/SKILL.md | grep -q '^name: brosettlement-api$$'

test:
	cd $(GO_DIR) && go test ./...

vet:
	cd $(GO_DIR) && go vet ./...

build:
	mkdir -p dist
	cd $(GO_DIR) && go build -o ../../../dist/brosettlement ./cmd/brosettlement
