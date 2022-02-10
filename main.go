package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type EventRecord struct {
	S3 struct {
		Bucket struct {
			Name string `json:name`
		} `json:bucket`
		Object struct {
			Key string `json:key`
		} `json:object`
	} `json:s3`
}

type ObjectCreatedEvent struct {
	Records []EventRecord `json:Records`
}

type MyResponse struct {
	Message string `json:"message"`
}

func HandleLambdaEvent(event ObjectCreatedEvent) (MyResponse, error) {
  bucketName := event.Records[0].S3.Bucket.Name
  objectKey := event.Records[0].S3.Object.Key

	sess := session.Must(session.NewSession())

	endpoint := os.Getenv("AWS_S3_ENDPOINT")
	sess.Config.Endpoint = &endpoint
	sess.Config.S3ForcePathStyle = aws.Bool(true)

	downloader := s3manager.NewDownloader(sess)

	filename := "/tmp/file"
	f, err := os.Create(filename)
	if err != nil {
		log.Printf("failed to create file %q, %v", filename, err)
		return MyResponse{}, fmt.Errorf("failed to create file %q, %v", filename, err)
	}

	_, err = downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		log.Printf("failed to download file, %v", err)
		return MyResponse{}, fmt.Errorf("failed to download file, %v", err)
	}

	// scan with clamav
	status := "ok"
	cmd := exec.Command("./bin/clamscan", filename)
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				status = "infected"
			} else {
				log.Printf("failed to scan file, %v", err)
				return MyResponse{}, fmt.Errorf("failed to scan file, %v", err)
			}
		} else {
			log.Printf("failed to scan file, %v", err)
			return MyResponse{}, fmt.Errorf("failed to scan file, %v", err)
		}
	}

	// tag document
	s3client := s3.New(sess)

	tagging, err := s3client.GetObjectTagging(&s3.GetObjectTaggingInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		log.Printf("failed to get tags, %v", err)
		return MyResponse{}, fmt.Errorf("failed to get tags, %v", err)
	}

	_, err = s3client.PutObjectTagging(&s3.PutObjectTaggingInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Tagging: &s3.Tagging{
			TagSet: append(tagging.TagSet, &s3.Tag{
				Key:   aws.String("virus-scan-status"),
				Value: aws.String(status),
			}),
		},
	})

	if err != nil {
		log.Printf("failed to write tags, %v", err)
		return MyResponse{}, fmt.Errorf("failed to write tags, %v", err)
	}

	log.Printf("scanning complete, tagged with %s", status)
	return MyResponse{Message: fmt.Sprintf("scanning complete, tagged with %s", status)}, nil
}

func main() {
	lambda.Start(HandleLambdaEvent)
}
