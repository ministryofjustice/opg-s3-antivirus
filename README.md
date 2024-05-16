# opg-s3-antivirus

opg-s3-antivirus is a lambda function that scans files uploaded to an S3 bucket for viruses. It uses the ClamAV antivirus engine to scan files.

The lambda function is triggered by put object events in the S3 bucket.

Once scanned, the function adds a tag `virus-scan-status` to the object in S3 with the result of the scan, either `ok` or `infected`.

## Antivirus Scan Function

You can find examples of how to use the scan lambda function in [docs/examples.md](docs/examples.md).

The zip version of the scan lamdba function and it's layer are base on the al2023 runtime, and the x86_64 architecture.

## Antivirus Definitions Update Function

The update function is an image based lambda function that updates the ClamAV definitions.

## Contact

Should you wish to talk to others about using this service, you can find help in the #ss-opg-s3-antivirus slack channel.
