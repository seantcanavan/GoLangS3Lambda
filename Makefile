build:
	env GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" main.go

test:
	go test

format:
	go fmt
