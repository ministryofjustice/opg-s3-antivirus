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
	docker compose build --parallel --no-cache s3-antivirus s3-antivirus-update

scan: setup-directories
	docker compose run --rm trivy image --format table --exit-code 0 311462405659.dkr.ecr.eu-west-1.amazonaws.com/s3-antivirus:latest
	docker compose run --rm trivy image --format sarif --output /test-results/trivy.sarif --exit-code 1 311462405659.dkr.ecr.eu-west-1.amazonaws.com/s3-antivirus:latest

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
