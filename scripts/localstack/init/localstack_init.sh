#!/usr/bin/env bash
set -euo pipefail

# Create bucket for definitions
awslocal s3api create-bucket \
    --acl private \
    --region eu-west-1 \
    --create-bucket-configuration LocationConstraint=eu-west-1 \
    --bucket "virus-definitions"

(cd /lambda && zip /tmp/forwarder.zip forwarder.py)
awslocal lambda create-function \
         --function-name s3-antivirus-update \
         --runtime python3.11 \
         --zip-file fileb:///tmp/forwarder.zip \
         --handler forwarder.handler \
         --timeout 120 \
         --role arn:aws:iam::000000000000:role/lambda-ex

# Create Private Bucket
awslocal s3api create-bucket \
    --acl private \
    --region eu-west-1 \
    --create-bucket-configuration LocationConstraint=eu-west-1 \
    --bucket "uploads-bucket"

# Add Public Access Block
awslocal s3api put-public-access-block \
    --public-access-block-configuration "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true" \
    --bucket "uploads-bucket"

# Add Default Encryption
awslocal s3api put-bucket-encryption \
    --bucket "uploads-bucket" \
    --server-side-encryption-configuration '{ "Rules": [ { "ApplyServerSideEncryptionByDefault": { "SSEAlgorithm": "AES256" } } ] }'

# Add Encryption Policy
awslocal s3api put-bucket-policy \
    --policy '{ "Statement": [ { "Sid": "DenyUnEncryptedObjectUploads", "Effect": "Deny", "Principal": { "AWS": "*" }, "Action": "s3:PutObject", "Resource": "arn:aws:s3:::uploads-bucket/*", "Condition":  { "StringNotEquals": { "s3:x-amz-server-side-encryption": "AES256" } } }, { "Sid": "DenyUnEncryptedObjectUploads", "Effect": "Deny", "Principal": { "AWS": "*" }, "Action": "s3:PutObject", "Resource": "arn:aws:s3:::uploads-bucket/*", "Condition":  { "Bool": { "aws:SecureTransport": false } } } ] }' \
    --bucket "uploads-bucket"

(cd /lambda && zip /tmp/forwarder.zip forwarder.py)
awslocal lambda create-function \
         --function-name s3-antivirus \
         --runtime python3.11 \
         --zip-file fileb:///tmp/forwarder.zip \
         --handler forwarder.handler \
         --role arn:aws:iam::000000000000:role/lambda-ex

awslocal lambda wait function-active-v2 --function-name s3-antivirus

echo '{
    "LambdaFunctionConfigurations": [
        {
            "Id": "bucket-av-scan",
            "LambdaFunctionArn": "arn:aws:lambda:eu-west-1:000000000000:function:s3-antivirus",
            "Events": [
                "s3:ObjectCreated:Put"
            ]
        }
    ]
}' > ./notification.json

awslocal s3api put-bucket-notification-configuration \
         --bucket uploads-bucket \
         --notification-configuration file://notification.json
