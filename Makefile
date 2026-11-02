.PHONY: install run

install:
	go install ./cmd/diffo

run:
	go run ./cmd/diffo
