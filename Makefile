.PHONY: run serve build test api web sample

## run: build the UI and start everything on http://localhost:8080
run: build serve

## serve: start the Go server and serve the already-built UI
serve:
	cd backend && go run ./cmd/server -static ../frontend/dist

## build: install frontend deps and build the UI
build:
	cd frontend && npm ci && npm run build

## test: run backend tests (with the race detector) and type-check the UI
test:
	cd backend && go vet ./... && go test -race ./...
	cd frontend && npm run typecheck

## api: development API on :8080 (pair with `make web`)
api:
	cd backend && go run ./cmd/server

## web: development UI with hot reload on :5173, proxying /api to :8080
web:
	cd frontend && npm run dev

## sample: write the demo statement to sample-statement.csv
sample:
	cd backend && go run ./cmd/gensample > ../sample-statement.csv
