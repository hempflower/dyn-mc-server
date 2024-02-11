# Makefile for building a Go project

# Variables
GO := go
BUILD_DIR := ./build
BIN_NAME := dmcs

# Build target
build:
	@mkdir -p $(BUILD_DIR)
	@cp -n ./conf/config.json $(BUILD_DIR)/config.json
	$(GO) build -o $(BUILD_DIR)/$(BIN_NAME) -v .

# Clean target
clean:
	@rm -rf $(BUILD_DIR)
	$(GO) clean

run: build
	@cd $(BUILD_DIR) && ./$(BIN_NAME)

# Default target
.DEFAULT_GOAL := build

# Phony targets
.PHONY: build clean

