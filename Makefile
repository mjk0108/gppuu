.PHONY: doctor frontend-install frontend-build go-test dev build

doctor:
	./scripts/doctor.sh

frontend-install:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build

go-test:
	go test ./...

dev: doctor frontend-install
	wails dev

build: doctor frontend-install frontend-build
	wails build -m -trimpath -tags webkit2_41,with_quic
