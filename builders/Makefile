BUILDERS_PREFIX := line/wasmvm-builder

.PHONY: docker-image-centos7
docker-image-centos7:
	docker build . -t $(BUILDERS_PREFIX):centos7 -f ./Dockerfile.centos7

.PHONY: docker-image-cross
docker-image-cross:
	docker build . -t $(BUILDERS_PREFIX):cross -f ./Dockerfile.cross

.PHONY: docker-image-alpine
docker-image-alpine:
	docker build . -t $(BUILDERS_PREFIX):alpine -f ./Dockerfile.alpine

.PHONY: docker-image-static
docker-image-static:
	docker build . -t $(BUILDERS_PREFIX):static -f ./Dockerfile.static

.PHONY: docker-images
docker-images: docker-image-centos7 docker-image-cross docker-image-alpine docker-image-static

# TODO publish images to registry
# .PHONY: docker-publish
# docker-publish: docker-images
# 	docker push $(BUILDERS_PREFIX):cross
# 	docker push $(BUILDERS_PREFIX):centos7
# 	docker push $(BUILDERS_PREFIX):alpine
# 	docker push $(BUILDERS_PREFIX):static
