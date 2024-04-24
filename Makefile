MAIN := gill-dashboard

CMD_DIR := cmd

.PHONY: all build

all: build

run: build
	./${MAIN}

build:
	go build -o ${MAIN} cmd/main.go

deps:
	@echo "Installing dependencies..."
	@go mod tidy
	@echo "Dependencies installed."

$(MAIN): deps
