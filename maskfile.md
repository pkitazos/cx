# CX Development Tasks

This maskfile defines development tasks for the `cx` command-line tool.

## build

> Build the cx binary

```bash
go build -o cx cmd/cx/*.go
```

## test

> Run unit tests with verbose output

```bash
go test -v ./cmd/cx
```

## test-coverage

> Run tests with coverage information

```bash
go test -cover ./cmd/cx
```

## test-coverage-html

> Generate HTML coverage report

```bash
go test -coverprofile=coverage.out ./cmd/cx
go tool cover -html=coverage.out -o coverage.html
echo "Coverage report generated: coverage.html"
```

## fmt

> Format Go code using go fmt

```bash
go fmt ./...
```

## vet

> Run go vet to check for common mistakes

```bash
go vet ./cmd/cx
```

## check

> Run all quality checks (format, vet, and test)

```bash
go fmt ./...
go vet ./cmd/cx
go test -v ./cmd/cx
```

## install

> Install the binary to GOPATH/bin

```bash
go install ./cmd/cx
```

## clean

> Remove build artifacts and generated files

```bash
rm -f cx
rm -f coverage.out coverage.html
```

## deps

> Update and verify Go dependencies

```bash
go mod tidy
go mod verify
```

## run (args)

> Build and run cx with provided arguments

```bash
go build -o cx cmd/cx/*.go
./cx $args
```