build:
	@GOOS=linux go build -ldflags="-s -w" -o ./bin/nim main.go

lint:
	@golangci-lint run
	@golangci-lint run --enable-only gocyclo

changelog-lint:
	@changelog-lint

version-lint:
	@./scripts/lint-version.sh

fix:
	@golangci-lint run --fix

install-hooks:
	@pre-commit install

install-changelog-lint:
	@go install github.com/chavacava/changelog-lint@master

dev-version:
	@./scripts/dev-version.sh

test:
	@go test -v ./tests

bench:
	@go test -run=^$$ -bench=. -benchmem ./tests

git-tag:
	@./scripts/release.sh $(v)
	@./scripts/lint-version.sh
	@git tag --sign v$(v) -m v$(v)
	@git push origin v$(v)

git-tag-remove:
	@git tag -d v$(v)
	@git push --delete origin v$(v)
