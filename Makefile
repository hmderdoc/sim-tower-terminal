# Sim Tower (terminal version)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -X main.version=$(VERSION)

.PHONY: build test trace vet run demo reference clean screenshots

build:            ## build ./simtower for this machine
	go build -buildvcs=false -ldflags "$(LDFLAGS)" -o simtower ./cmd/simtower

test:             ## unit tests (sim trace replays included when reference/ is present)
	go test ./...

trace:            ## replay every binary reference trace, verbose
	go test ./internal/sim/ -run 'TestTrace' -v

vet:
	go vet ./...

run: build        ## play on this terminal
	./simtower

demo: build       ## the scripted demo tower
	./simtower -demo

reference:        ## fetch the upstream sim + trace fixtures (needed for the trace tests)
	@test -d reference/tower-together/sim || ( \
	  rm -rf /tmp/tower-together && git clone --depth 1 https://github.com/phulin/tower-together /tmp/tower-together && \
	  mkdir -p reference/tower-together/sim && \
	  cp -r /tmp/tower-together/specs /tmp/tower-together/LICENSE.md /tmp/tower-together/README.md reference/tower-together/ && \
	  cp -r /tmp/tower-together/apps/worker/src/sim/. reference/tower-together/sim/ && \
	  (cd /tmp/tower-together && git rev-parse HEAD) > reference/tower-together/COMMIT )

screenshots:      ## re-render docs/screenshots from the UI goldens
	python3 tools/render_goldens.py 80x24_start 132x50_start 80x24_main 132x50_main 80x24_build 80x24_elevator 80x24_night 80x24_finance 80x24_map

clean:
	rm -f simtower simtower.exe simtower.log
