run: # Run the docker container
	docker build -t onionbox_image . && \
	docker run --name onionbox -p 80 onionbox_image
stop: # Stop the docker container
	docker stop onionbox && \
	docker rmi -f onionbox_image && \
	docker container rm -f onionbox
restart: stop run # Rebuilds the docker container
exec: # Open a bash shell into the docker container
	docker exec -it onionbox bash
compose:
	export EXTERNAL_IP="dig +short myip.opendns.com @resolver1.opendns.com"
	docker-compose up -d
compose-stop: # Stops the project
	docker-compose down -v --remove-orphans && \
	docker rmi -f onionbox_onionbox:latest
compose-restart: compose-stop compose
compose-logs:
	docker-compose logs -f --tail 100 onionbox
lint: # Will lint the project
	golint
	go vet ./...
	go fmt ./...
test: lint # Will run tests on the project as well as lint
	go test -v -race -bench -cpu=1,2,4 ./...

.PHONY: run stop restart exec compose compose-stop compose-restart compose-logs lint test