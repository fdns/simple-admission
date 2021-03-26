build:
	docker build . --tag fdns/simple-admission:latest

certificates:
	./generate_certs.sh simple-admission default

kind: build certificates
	kind load docker-image fdns/simple-admission:latest --name kind
