#!/usr/bin/env bash
set -euo pipefail

awslocal lambda create-function \
         --function-name myfunction \
         --code ImageUri=antivirus-function:latest \
         --role arn:aws:iam::000000000:role/lambda-ex

echo '{
    "LambdaFunctionConfigurations": [
        {
            "Id": "bucket-av-scan",
            "LambdaFunctionArn": "arn:aws:lambda:eu-west-1:000000000000:function:myfunction",
            "Events": [
                "s3:ObjectCreated:Put"
            ]
        }
    ]
}' > ./notification.json

awslocal s3api put-bucket-notification-configuration \
         --bucket opg-backoffice-datastore-local \
         --notification-configuration file://notification.json
