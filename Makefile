# AURA — Makefile
# Сборка aura-ctl и aura-indicator в bin/aura-indicator/

GO_DIR  = ./aura-indicator
BIN_DIR = $(GO_DIR)/bin

# Имена бинарников
CTL_BIN    = aura-ctl
IND_BIN    = aura-indicator

.PHONY: all build clean rebuild

all: build

build: $(BIN_DIR)/$(CTL_BIN) $(BIN_DIR)/$(IND_BIN)

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(BIN_DIR)/$(CTL_BIN): $(BIN_DIR)
	cd $(GO_DIR) && go build -ldflags="-s -w" -o bin/$(CTL_BIN) ./cmd/aura-ctl/

$(BIN_DIR)/$(IND_BIN): $(BIN_DIR)
	cd $(GO_DIR) && go build -ldflags="-s -w" -o bin/$(IND_BIN) ./cmd/aura-indicator/

# Кросс-компиляция для релиза
.PHONY: cross

cross:
	cd $(GO_DIR) && \
	GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(CTL_BIN)_linux_amd64   ./cmd/aura-ctl/ && \
	GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o bin/$(CTL_BIN)_linux_arm64   ./cmd/aura-ctl/ && \
	GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(IND_BIN)_linux_amd64   ./cmd/aura-indicator/ && \
	GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o bin/$(IND_BIN)_linux_arm64   ./cmd/aura-indicator/ && \
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(CTL_BIN)_windows_amd64.exe ./cmd/aura-ctl/ && \
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(IND_BIN)_windows_amd64.exe ./cmd/aura-indicator/ && \
	GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(CTL_BIN)_darwin_amd64  ./cmd/aura-ctl/ && \
	GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o bin/$(CTL_BIN)_darwin_arm64  ./cmd/aura-ctl/ && \
	GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(IND_BIN)_darwin_amd64  ./cmd/aura-indicator/ && \
	GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o bin/$(IND_BIN)_darwin_arm64  ./cmd/aura-indicator/

clean:
	rm -rf $(BIN_DIR)

rebuild: clean build
