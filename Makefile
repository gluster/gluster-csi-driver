.PHONY: all glusterfs-csi-driver clean vendor-update vendor-install check-go check-reqs test

all: check-go check-reqs vendor-install glusterfs-csi-driver

glusterfs-csi-driver:
	@echo Building glusterfs csi driver
	go build -o build/glusterfs-csi-driver  cmd/glusterfs/main.go

clean:
	go clean -r -x
	rm -rf build

check-go:
	@./scripts/check-go.sh
	@echo

vendor-update:
	@echo Updating vendored packages
	@$(DEPENV) dep ensure -update
	@echo

vendor-install:
	@echo Installing vendored packages
	@$(DEPENV) dep ensure -vendor-only
	@echo

check-reqs:
	@./scripts/check-reqs.sh
	@echo

test: check-reqs
	@./test.sh $(TESTOPTIONS)
	@echo
