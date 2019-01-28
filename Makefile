UNIX_BINARY=onionbox

run: # Rebuild the docker container
	docker build -t onionbox_image . && \
	docker run --name onionbox -p 80 onionbox_image
stop:
	docker stop onionbox && \
	docker rm onionbox && \
	docker container rmi onionbox_image
restart: stop run
exec:
	docker exec -it onionbox bash
lint: # Will lint the project
	golint
	go vet ./...
	go fmt ./...
test: lint # Will run tests on the project as well as lint
	go test -v -race -bench -cpu=1,2,4 ./...

.PHONY: run stop restart exec lint test