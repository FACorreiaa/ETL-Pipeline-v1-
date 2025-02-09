build:
	docker build -t scoring-app .

run:
	docker run --rm scoring-app

up:
	docker compose up --build

test:
	go test -v ./...
