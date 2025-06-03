#! /usr/bin/env bash
set -euo pipefail

echo "Waiting for buckets"

iterations=0

buckets=""

awslocal lambda list-functions
sleep 10
awslocal lambda wait function-active-v2 --region eu-west-1 --function-name s3-antivirus
awslocal lambda wait function-active-v2 --region eu-west-1 --function-name s3-antivirus-update
awslocal lambda list-functions

while [[ "$iterations" -lt 60 ]]
do
  buckets=$(awslocal s3 ls)

  if [[ $buckets = *uploads-bucket* ]] &&
     [[ $buckets = *virus-definitions* ]]
  then
    echo "Found all expected buckets after $iterations seconds"
    exit 0
  fi

  ((iterations++))
  printf '.'
  sleep 1
done

echo "Waited $iterations seconds for buckets before giving up"
echo "s3 ls results:"
echo "----------------------------------"
echo "$buckets"
echo "----------------------------------"

exit 1
