#!/usr/bin/env bash
set -euo pipefail

trap 'catch' ERR
catch() {
    echo "---- docker logs ----"
    docker compose -f docker/docker-compose.yml logs -t --no-color | sort -u -k 3
}

docker compose -f docker/docker-compose.yml up --wait localstack
docker compose -f docker/docker-compose.yml exec -T localstack bash -c '. /scripts/wait/wait-until-s3-ready.sh'

docker compose -f docker/docker-compose.yml exec -T localstack awslocal lambda invoke --endpoint http://antivirus-update-function:8080 --no-sign-request  --cli-read-timeout 120 --function-name function --payload '{}' /dev/stdout
docker compose -f docker/docker-compose.yml exec -T localstack awslocal s3api list-objects --bucket virus-definitions
docker compose -f docker/docker-compose.yml restart antivirus-function

sleep 10
docker compose -f docker/docker-compose.yml exec -T localstack bash -c 'echo "Test file" | awslocal s3 cp - s3://uploads-bucket/valid.txt'

docker compose -f docker/docker-compose.yml exec -T localstack bash -c '. /scripts/wait/wait-until-tagged.sh valid.txt'
docker compose -f docker/docker-compose.yml exec -T localstack awslocal s3api get-object-tagging --bucket uploads-bucket --key valid.txt | jq -e '(.TagSet[] | select(.Key == "virus-scan-status")).Value == "ok"'

docker compose -f docker/docker-compose.yml exec -T localstack bash -c 'echo "X5O!P%@AP[4\PZX54(P^)7CC)7}\$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!\$H+H*" | awslocal s3 cp - s3://uploads-bucket/invalid.txt'

docker compose -f docker/docker-compose.yml exec -T localstack bash -c '. /scripts/wait/wait-until-tagged.sh invalid.txt'
docker compose -f docker/docker-compose.yml exec -T localstack awslocal s3api get-object-tagging --bucket uploads-bucket --key invalid.txt | jq -e '(.TagSet[] | select(.Key == "virus-scan-status")).Value == "infected"'
