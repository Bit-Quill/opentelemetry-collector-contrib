include ../../Makefile.Common

local-run-example:
	cd ../../ && make docker-otelcontribcol
	cd demo && docker-compose up -d
recreate-otel-collector:
	cd ../../ && GOOS=linux go build -o ./local/otelcontribcol ./cmd/otelcontribcol
	cd demo && docker-compose up --build otelcollector
