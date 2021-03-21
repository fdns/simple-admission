build:
	docker build . --tag fdns/simple-admission:latest

kind: build
	kind load docker-image fdns/simple-admission:latest --name kind
