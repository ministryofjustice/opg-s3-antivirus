#! /usr/bin/env bash
set -uo pipefail

echo "Waiting for \"$1\" to be tagged"

iterations=0

buckets=""

while [[ "$iterations" -lt 120 ]]
do
  tags=$(awslocal s3api get-object-tagging --bucket uploads-bucket --key $1 2> /dev/null) || true

  if [[ $tags = *virus-scan-status* ]]
  then
    exit 0
  fi

  ((iterations++))
  printf '.'
  sleep 1
done

echo "Waited $iterations seconds for the file before giving up"
echo "tag results:"
echo "----------------------------------"
echo "$tags"
echo "----------------------------------"

exit 1
