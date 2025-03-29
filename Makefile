.PHONY: test fmt cov tidy lint run

COVFILE = coverage.out
COVHTML = cover.html

test:
	go test ./... -json | go tool tparse -all

fmt:
	go tool gofumpt -l -w *.go

cov:
	go test -cover ./... -coverprofile=$(COVFILE)
	go tool cover -html=$(COVFILE) -o $(COVHTML)
	rm $(COVFILE)
	open $(COVHTML)

tidy:
	go mod tidy -v

lint:
	go tool golangci-lint run -v

run:
	go build
	./gh-myprs
