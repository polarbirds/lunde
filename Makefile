build-dir:
	mkdir -p bin

build: build-dir
	go build -o bin/lunde cmd/main/main.go

run: # make ARGS="-arg1 val1 -arg2 -arg3" run
	./bin/lunde ${ARGS}

clean:
	rm -r bin

lint:
	revive -config revive.toml -formatter friendly -exclude vendor/... ./...

install:
	go get github.com/mgechev/revive
	dep ensure
