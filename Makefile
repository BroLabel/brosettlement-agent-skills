GO_DIR := brosettlement-api/scripts/go

.PHONY: check validate lifecycle-test test vet build

check: validate lifecycle-test test vet build

validate:
	@test -f brosettlement-onboarding/SKILL.md
	@test -f brosettlement-api/SKILL.md
	@head -n 5 brosettlement-onboarding/SKILL.md | grep -q '^name: brosettlement-onboarding$$'
	@head -n 5 brosettlement-api/SKILL.md | grep -q '^name: brosettlement-api$$'
	@grep -q 'Public key (PEM)' brosettlement-onboarding/SKILL.md
	@grep -q 'fenced `pem` block' brosettlement-onboarding/SKILL.md
	@grep -q 'integration-api-public.pem' brosettlement-onboarding/SKILL.md
	@grep -q 'https://nileex.io/join/getJoinPage' brosettlement-onboarding/SKILL.md
	@grep -q 'https://developers.tron.network/docs/getting-testnet-tokens-on-tron' brosettlement-onboarding/SKILL.md
	@grep -q '0.0.0.0/0' brosettlement-onboarding/SKILL.md
	@grep -q 'must never be used' brosettlement-onboarding/SKILL.md
	@grep -q 'brosettlement update --auto' brosettlement-api/SKILL.md
	@grep -q 'never update `SKILL.md`' brosettlement-api/SKILL.md
	@test -x brosettlement-api/scripts/build-cli.sh
	@test -f .github/workflows/release-cli.yml

lifecycle-test:
	./tests/install-uninstall.sh

test:
	cd $(GO_DIR) && go test ./...

vet:
	cd $(GO_DIR) && go vet ./...

build:
	mkdir -p dist
	cd $(GO_DIR) && go build -trimpath \
		-ldflags "-X github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/brocli.Version=0.0.0-dev -X github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/brocli.Commit=local" \
		-o ../../../dist/brosettlement ./cmd/brosettlement
