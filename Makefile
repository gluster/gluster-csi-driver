.PHONY: all glusterfs-csi-driver clean

all: glusterfs-csi-driver


glusterfs-csi-driver:
	if [ ! -d ./vendor ]; then dep ensure -vendor-only; fi
	go build -o build/glusterfs-csi-driver  cmd/glusterfs/main.go
clean:
	go clean -r -x
	rm -rf build
