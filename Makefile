build-dir:
	mkdir -p bin

build: build-dir
	go build -o bin/lunde cmd/main/main.go

run: build
	CONFIG="file::./dev_cfg/cfg.yml" \
	./bin/lunde

watch:
	reflex --start-service=true -r '\.go$$' make run

clean:
	rm -r bin

mods: mod
mod:
	GOSUMDB=off ./build/mod.sh

lint: lint-all

lint-all:
	revive -config revive.toml -formatter friendly -exclude vendor/... ./...

install-tools:
	./build/install_tools.sh
