#! /usr/bin/env bash
set -euo pipefail

echo "Waiting for buckets"

iterations=0

buckets=""

while [[ "$iterations" -lt 300 ]]
do
  buckets=$(awslocal s3 ls)

  if [[ $buckets = *opg-backoffice-datastore-local* ]]
  then
    echo "Found all expected buckets after $iterations seconds"
    exit 0
  fi

  ((iterations++))
  sleep 1
done

echo "Waited $iterations seconds for buckets before giving up"
echo "s3 ls results:"
echo "----------------------------------"
echo "$buckets"
echo "----------------------------------"

exit 1
