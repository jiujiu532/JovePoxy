# Root targets. Intentionally omits go test -race (Windows host limitation).
.PHONY: build test vet web web-test embed-web all smoke

web:
	cd web && pnpm install --frozen-lockfile && pnpm build

web-test:
	cd web && pnpm test

embed-web: web
	@mkdir -p internal/webui/dist
	cp -R web/dist/. internal/webui/dist/

# Windows-friendly embed (PowerShell host)
embed-web-win:
	cd web && pnpm install --frozen-lockfile && pnpm build
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path 'internal/webui/dist' | Out-Null; Copy-Item -Path 'web/dist/*' -Destination 'internal/webui/dist/' -Recurse -Force"

build:
	go build -o bin/jovepoxy.exe ./cmd/server

test:
	go test -shuffle=on -count=1 ./...

vet:
	go vet ./...

all: embed-web-win test build

smoke:
	powershell -NoProfile -File scripts/smoke.ps1
