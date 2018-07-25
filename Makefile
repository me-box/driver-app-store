IMAGE_NAME=driver-app-store
DEFAULT_REG=toshbrown
VERSION=latest

.PHONY: all
all: build-amd64 build-arm64v8 publish-images

.PHONY: build-amd64
build-amd64:
	docker build -t $(DEFAULT_REG)/$(IMAGE_NAME)-amd64:$(VERSION) . $(OPTS)

.PHONY: build-arm64v8
build-arm64v8:
	docker build -t $(DEFAULT_REG)/$(IMAGE_NAME)-arm64v8:$(VERSION) -f Dockerfile-arm64v8 .  $(OPTS)

.PHONY: publish-images
publish-images:
	docker push $(DEFAULT_REG)/$(IMAGE_NAME)-amd64:$(VERSION)
	docker push $(DEFAULT_REG)/$(IMAGE_NAME)-arm64v8:$(VERSION)

	docker manifest create --amend $(IMAGE_NAME):$(VERSION) $(IMAGE_NAME)-amd64:$(DATABOX_VERSION)
	docker manifest annotate $(IMAGE_NAME):$(VERSION) $(IMAGE_NAME)-amd64:$(DATABOX_VERSION) --os linux --arch arm64
	#TODO re-enable this when core-store, export-servive and core network build for arm64v8
	#docker manifest annotate $(IMAGE_NAME):$(VERSION) $(IMAGE_NAME)-arm64v8:$(DATABOX_VERSION)--os linux --arch arm64 --variant v8
	docker manifest push -p $(IMAGE_NAME)

.PHONY: test
test:
	./setupTest.sh
	go run *.go  --storeurl tcp://127.0.0.1:5555 --arbiterurl tcp://127.0.0.1:4444