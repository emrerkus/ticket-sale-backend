# Sik kullanilan komutlar icin kisayollar. Calistir: `make <hedef>`
# Windows'ta `make` yoksa: `choco install make` veya WSL/Git-Bash icindeki make.
# Alternatif olarak her hedefin altindaki komutu elle de calistirabilirsin.

.PHONY: help up down logs run worker web build test test-e2e tidy fmt vet migrate-up migrate-down migrate-version seed psql full

help: ## Bu listeyi goster
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

up: ## Postgres + Redis'i baslat (arka planda)
	docker compose up -d

down: ## Altyapiyi durdur
	docker compose down

logs: ## Altyapi loglarini izle
	docker compose logs -f

run: ## API'yi calistir (go run)
	go run ./cmd/api

worker: ## Arka plan worker'ini calistir (suresi gecen hold/siparisleri temizler)
	go run ./cmd/worker

web: ## Frontend gelistirme sunucusu (http://localhost:5173)
	cd web && npm run dev

full: ## Tum yigini konteynerde ayaga kaldir (api+worker+web+pg+redis)
	docker compose --profile full up --build -d

migrate-up: ## Bekleyen migration'lari uygula
	go run ./cmd/migrate up

migrate-down: ## Son migration'i geri al (1 adim)
	go run ./cmd/migrate down

migrate-version: ## Su anki sema surumu
	go run ./cmd/migrate version

seed: ## Ornek veri yukle (katalogu temizleyip bastan yazar)
	go run ./cmd/seed

psql: ## Postgres'e psql kabugu ac
	docker compose exec postgres psql -U ticketsale -d ticketsale

build: ## Binary uret -> bin/api
	go build -o bin/api ./cmd/api

test: ## Go testlerini kosur (race detector ile)
	go test ./... -race -count=1

test-e2e: ## Ucdan uca akis testi (API calisiyor olmali)
	python scripts/e2e.py

tidy: ## go.mod / go.sum'i duzenle
	go mod tidy

fmt: ## Kodu formatla
	go fmt ./...

vet: ## Statik analiz (yaygin hatalar)
	go vet ./...
