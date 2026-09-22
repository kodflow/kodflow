# Local loop: edit profile.json, `make preview`, open dist/preview.html.
DENYLIST ?= $(HOME)/.config/kodflow-profile/denylist

.PHONY: test render preview readme commit

test:
	go vet ./...
	PROFILE_DENYLIST=$(DENYLIST) go test ./... -count=1

render:
	GH_TOKEN=$$(gh auth token) go run ./cmd/profile -out dist

preview:
	GH_TOKEN=$$(gh auth token) go run ./cmd/profile -out dist -preview
	@echo "open dist/preview.html"

readme:
	go run ./cmd/profile -offline -out dist -readme README.md

# Commit with UTC dates: git stores the local offset otherwise, and the
# .githooks/pre-push hook refuses it. Usage: make commit m="feat: ..."
commit:
	TZ=UTC git commit -m "$(m)"
