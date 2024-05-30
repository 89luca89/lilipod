.PHONY: all lilipod pty coverage

all: clean pty lilipod

clean:
	@rm -f lilipod
	@rm -f pty
	@rm -f pty.tar.gz

lilipod:
	@rm -f lilipod
	CGO_ENABLED=0 go build -mod vendor -ldflags="-s -w -X 'github.com/89luca89/lilipod/pkg/constants.Version=$${RELEASE_VERSION:-0.0.0}'" -o lilipod main.go

coverage:
	@rm -rf coverage/*
	@mkdir -p coverage
	CGO_ENABLED=0 go build -mod vendor -cover -o coverage/pty ptyagent/main.go ptyagent/pty.go
	@rm -f pty
	@rm -f pty.tar.gz
	CGO_ENABLED=0 go build -mod vendor -gcflags=all="-l -B -C" -ldflags="-s -w" -o pty ptyagent/main.go ptyagent/pty.go
	tar czfv pty.tar.gz pty
	CGO_ENABLED=0 go build -mod vendor -cover -o coverage/lilipod main.go

pty:
	@rm -f pty
	@rm -f pty.tar.gz
	CGO_ENABLED=0 go build -mod vendor -gcflags=all="-l -B -C" -ldflags="-s -w -X 'main.version=$${RELEASE_VERSION:-0.0.0}'" -o pty ptyagent/main.go ptyagent/pty.go
	tar czfv pty.tar.gz pty

trivy:
	@trivy fs --scanners vuln .
