build:	test
	go build -o ${GOPATH}/bin/git-phabricator-mirror git-phabricator-mirror/git-phabricator-mirror.go

test:	fmt
	go build ./...
	go test ./...

fmt:
	gofmt -w `find ./ -name '*.go'`
