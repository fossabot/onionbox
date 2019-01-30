run:
	export EXTERNAL_IP="dig +short myip.opendns.com @resolver1.opendns.com"
	docker-compose up -d
stop: # Stops the project
	docker-compose down -v --remove-orphans && \
	docker rmi -f onionbox_onionbox:latest
restart: stop run
logs:
	docker-compose logs -f --tail 100 onionbox
exec: # Open a bash shell into the docker container
	docker exec -it onionbox bash
lint: # Will lint the project
	golint
	go vet ./...
	go fmt ./...
test: lint # Will run tests on the project as well as lint
	go test -v -race -bench -cpu=1,2,4 ./...

.PHONY: exec compose compose-stop compose-restart compose-logs lint test