all: lint go-sec unit-test build scan acceptance-test down

test-results:
	mkdir -p -m 0777 .cache test-results .gocache cypress/screenshots .trivy-cache

setup-directories: test-results

lint: setup-directories
	docker compose run go-lint

go-sec: setup-directories
	docker compose run go-sec

unit-test: setup-directories
	docker compose run test-runner

build:
	docker compose build --parallel s3-antivirus s3-antivirus-update

scan: setup-directories
	docker compose run --rm trivy image --format table --exit-code 0 311462405659.dkr.ecr.eu-west-1.amazonaws.com/s3-antivirus:latest
	docker compose run --rm trivy image --format sarif --output /test-results/trivy.sarif --exit-code 1 311462405659.dkr.ecr.eu-west-1.amazonaws.com/s3-antivirus:latest

check-clam:
	docker compose up s3-antivirus -d
	docker compose exec -T s3-antivirus bash -c 'clamdscan --version'
	docker compose exec -T s3-antivirus bash -c 'clamd --config-file "/opt/etc/clamd.conf"'

acceptance-test:
	docker compose up --wait localstack
	docker compose exec -T localstack bash -c '. /scripts/wait/wait-until-s3-ready.sh'
	docker compose exec -T localstack awslocal lambda invoke --cli-read-timeout 120 --function-name s3-antivirus-update --payload '{}' /dev/stdout
	docker compose exec -T localstack awslocal s3api list-objects --bucket virus-definitions
	docker compose restart s3-antivirus
	sleep 10
	docker compose exec -T localstack bash -c 'echo "Test file" | awslocal s3 cp - s3://uploads-bucket/valid.txt'
	docker compose exec -T localstack bash -c '. /scripts/wait/wait-until-tagged.sh valid.txt'
	docker compose exec -T localstack awslocal s3api get-object-tagging --bucket uploads-bucket --key valid.txt | jq -e '(.TagSet[] | select(.Key == "virus-scan-status")).Value == "ok"'
	docker compose exec -T localstack bash -c 'echo "X5O!P%@AP[4\PZX54(P^)7CC)7}\$$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!\$$H+H*" | awslocal s3 cp - s3://uploads-bucket/invalid.txt'
	docker compose exec -T localstack bash -c '. /scripts/wait/wait-until-tagged.sh invalid.txt'
	docker compose exec -T localstack awslocal s3api get-object-tagging --bucket uploads-bucket --key invalid.txt | jq -e '(.TagSet[] | select(.Key == "virus-scan-status")).Value == "infected"'

down:
	docker compose down

lambda-zip-clear:
	rm -fr ./build
.PHONY: lambda-zip-clear

lambda-zip-prep: lambda-zip-clear
	mkdir -p ./build && \
	cp ./scripts/build-zip/build-lambda-zip.sh ./build/ && \
	cp ./go.mod ./go.sum ./build
	cp -R ./cmd/opg-s3-antivirus ./build/ && \
	chmod +x ./build/build-lambda-zip.sh
.PHONY: zip-prep

lambda-zip-build: lambda-zip-prep
	docker run --rm \
		--platform linux/amd64 \
		-v `pwd`/build:/app:Z \
		golang:1.22.2-alpine \
		/bin/sh -c "cd /app && ./build-lambda-zip.sh"
.PHONY: lambda-zip-build

layer-zip-clean:
	rm -fr ./build
.PHONY: layer-zip-clear

layer-zip-prep:
	mkdir -p ./build && \
	cp ./scripts/build-zip/build-layer-zip.sh  ./build/ && \
	cp clamd.conf ./build/ && \
	chmod +x ./build/build-layer-zip.sh
.PHONY: layer-zip-prep

layer-zip-build: layer-zip-prep
	docker run --rm \
		--platform linux/amd64 \
		-v `pwd`/build:/app:Z \
		amazonlinux:2023.4.20240416.0 \
		/bin/bash -c "cd /app && ./build-layer-zip.sh"
.PHONY: layer-zip-build

layer-zip-test-prep:
	rm -fr ./build/test && \
	rm -fr ./build/bin && \
	rm -fr ./build/lib && \
	rm -fr ./build/etc && \
	mkdir -p ./build/test && \
	cp ./scripts/build-zip/test-layer-zip.sh ./build/test/ && \
	cp build/lambda_layer.zip ./build/test/ && \
	chmod +x ./build/test/test-layer-zip.sh
.PHONY: layer-zip-test-prep

layer-zip-test: layer-zip-test-prep
	docker run --rm \
	--platform linux/amd64 \
		-v `pwd`/build:/app:Z \
		amazonlinux:2023.4.20240416.0 \
		/bin/bash -c "cd /app && ./test/test-layer-zip.sh"
.PHONY: layer-test
