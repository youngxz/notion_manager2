.PHONY: build run clean test web web-install build-linux build-go

# Binary names
BINARY=notion-manager
REGISTER_BINARY=notion-manager-register

# Determine the OS for shell commands
ifeq ($(OS),Windows_NT)
    RM_DIR=cmd /c "if exist internal\web\dist rmdir /s /q internal\web\dist"
    COPY_DIR=cmd /c "xcopy web\dist internal\web\dist\ /E /I /Y /Q"
    CLEAN_CMD=del /f $(BINARY).exe 2>nul || true && rmdir /s /q web\dist 2>nul && rmdir /s /q internal\web\dist 2>nul
    EXE=.exe
else
    RM_DIR=rm -rf internal/web/dist
    COPY_DIR=cp -r web/dist internal/web/dist
    CLEAN_CMD=rm -rf $(BINARY) $(REGISTER_BINARY) web/dist internal/web/dist bin/
    EXE=
endif

# Build frontend and backend
build: web build-go

# Build backend only (skip frontend)
build-go:
	mkdir -p bin
	go build -o bin/$(BINARY)$(EXE) ./cmd/notion-manager/
	go build -o bin/$(REGISTER_BINARY)$(EXE) ./cmd/register/

# Run the proxy
run: build
	./bin/$(BINARY)$(EXE)

# Install frontend dependencies
web-install:
	cd web && npm install

# Build frontend and copy to embed directory
web:
	cd web && npm install && npm run build
	$(RM_DIR)
	$(COPY_DIR)

# Clean build artifacts
clean:
	$(CLEAN_CMD)

# Run tests
test:
	go test ./...

# Check code
vet:
	go vet ./...
