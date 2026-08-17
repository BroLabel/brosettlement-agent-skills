GO_DIR := brosettlement-api/scripts/go
RECOVERY_DIR := brosettlement-disaster-recovery/scripts

.PHONY: check validate lifecycle-test test vet build recovery-check

check: validate lifecycle-test test vet build recovery-check

validate:
	@test -f brosettlement-onboarding/SKILL.md
	@test -f brosettlement-api/SKILL.md
	@test -f brosettlement-disaster-recovery/SKILL.md
	@head -n 5 brosettlement-onboarding/SKILL.md | grep -q '^name: brosettlement-onboarding$$'
	@head -n 5 brosettlement-api/SKILL.md | grep -q '^name: brosettlement-api$$'
	@head -n 5 brosettlement-disaster-recovery/SKILL.md | grep -q '^name: brosettlement-disaster-recovery$$'
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
	@test -f brosettlement-disaster-recovery/scripts/recovery-tron-sign.go
	@test -f brosettlement-disaster-recovery/scripts/go.mod
	@test -f brosettlement-disaster-recovery/scripts/go.sum
	@grep -q 'BROADCAST_ACCEPTED' brosettlement-disaster-recovery/SKILL.md
	@grep -q 'Share B and Share C do not form one validated recovery quorum' brosettlement-disaster-recovery/SKILL.md
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

recovery-check:
	cd $(RECOVERY_DIR) && go test ./...
	cd $(RECOVERY_DIR) && go vet ./...
	@temp_dir=$$(mktemp -d "$${TMPDIR:-/tmp}/brosettlement-recovery-build.XXXXXX"); \
		trap 'rm -rf "$$temp_dir"' EXIT HUP INT TERM; \
		cd $(RECOVERY_DIR); \
		go build -trimpath -o "$$temp_dir/recovery-tron-sign" .; \
		"$$temp_dir/recovery-tron-sign" --help >/dev/null 2>&1
