# Examples

Here are some useful code snippets to help you get started with the S3 Antivirus Scanner Lambda Function.

## Fetching a version of the Lambda Function Zip and Lambda Layer Package

```shell

wget -O myFunction.zip https://github.com/ministryofjustice/opg-s3-antivirus/releases/download/v0.594.0/myFunction-amd64.zip
wget -O myFunction.zip.sha256sum https://github.com/ministryofjustice/opg-s3-antivirus/releases/download/v0.594.0/myFunction-amd64.zip.sha256sum
sha256sum -c "myFunction.zip.sha256sum"
wget -O lambda_layer.zip https://github.com/ministryofjustice/opg-s3-antivirus/releases/download/v0.594.0/lambda_layer-amd64.zip
wget -O lambda_layer.zip.sha256sum https://github.com/ministryofjustice/opg-s3-antivirus/releases/download/v0.594.0/lambda_layer-amd64.zip.sha256sum
sha256sum -c "lambda_layer.zip.sha256sum"

```

## Deploying the zip package to AWS Lambda with Terraform

```hcl
resource "aws_lambda_layer_version" "lambda_layer" {
  filename                 = "${path.module}/lambda_layer.zip"
  layer_name               = "clamav"
  description              = "ClamAV Antivirus Layer"
  source_code_hash         = filebase64sha256("${path.module}/lambda_layer.zip")
  compatible_architectures = ["x86_64"]
  compatible_runtimes      = ["provided.al2023"]
  provider                 = aws.region
}

resource "aws_lambda_function" "zip_lambda_function" {
  function_name    = "zip-s3-antivirus"
  description      = "Function to scan S3 objects for viruses"
  filename         = "${path.module}/myFunction.zip"
  handler          = "bootstrap"
  source_code_hash = filebase64sha256("${path.module}/myFunction.zip")

  architectures = ["x86_64"]
  runtime       = "provided.al2023"
  timeout       = 300
  memory_size   = 4096
  publish       = true

  layers = [
    aws_lambda_layer_version.lambda_layer.arn
  ]

  role = var.lambda_task_role.arn

  tracing_config {
    mode = "Active"
  }

  logging_config {
    log_group  = var.aws_cloudwatch_log_group.name
    log_format = "JSON"
  }

  dynamic "environment" {
    for_each = length(keys(var.environment_variables)) == 0 ? [] : [true]
    content {
      variables = var.environment_variables
    }
  }
  provider = aws.region
}
```