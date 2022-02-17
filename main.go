package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type EventRecord struct {
	S3 struct {
		Bucket struct {
			Name string `json:"name"`
		} `json:"bucket"`
		Object struct {
			Key string `json:"key"`
		} `json:"object"`
	} `json:"s3"`
}

type ObjectCreatedEvent struct {
	Records []EventRecord `json:"Records"`
}

type MyResponse struct {
	Message string `json:"message"`
}

type LambdaTagValues struct {
	pass string
	fail string
}

type Downloader interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

type Scanner interface {
	ScanFile(path string) (bool, error)
}

const tmpFilePath = "/tmp/file"

type Lambda struct {
	tagKey     string
	tagValues  LambdaTagValues
	scanner    Scanner
	s3         s3iface.S3API
	downloader Downloader
}

func (l *Lambda) downloadFile(bucket string, key string) error {
	f, err := os.Create(tmpFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file %q, %w", tmpFilePath, err)
	}

	obj, err := l.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	log.Print(obj)

	if err != nil {
		return fmt.Errorf("failed to GetObject, %w", err)
	}

	_, err = l.downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to download file, %w", err)
	}

	return nil
}

func (l *Lambda) tagFile(bucket string, key string, status string) error {
	tagging, err := l.s3.GetObjectTagging(&s3.GetObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to get tags, %w", err)
	}

	_, err = l.s3.PutObjectTagging(&s3.PutObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Tagging: &s3.Tagging{
			TagSet: append(tagging.TagSet, &s3.Tag{
				Key:   aws.String(l.tagKey),
				Value: aws.String(status),
			}),
		},
	})

	if err != nil {
		return fmt.Errorf("failed to write tags, %w", err)
	}

	return nil
}

func (l *Lambda) HandleEvent(event ObjectCreatedEvent) (MyResponse, error) {
	bucketName := event.Records[0].S3.Bucket.Name
	objectKey := event.Records[0].S3.Object.Key

	err := l.downloadFile(bucketName, objectKey)
	if err != nil {
		log.Print(err)
		return MyResponse{}, err
	}

	status, err := l.scanner.ScanFile(tmpFilePath)
	if err != nil {
		log.Print(err)
		return MyResponse{}, err
	}

	statusString := l.tagValues.fail
	if status {
		statusString = l.tagValues.pass
	}

	log.Printf("tagging object with %s", statusString)

	err = l.tagFile(bucketName, objectKey, statusString)
	if err != nil {
		log.Print(err)
		return MyResponse{}, err
	}

	log.Printf("scanning complete, tagged with %s", statusString)
	return MyResponse{Message: fmt.Sprintf("scanning complete, tagged with %s", statusString)}, nil
}

func main() {
	sess := session.Must(session.NewSession())

	endpoint := os.Getenv("AWS_S3_ENDPOINT")
	sess.Config.Endpoint = &endpoint
	sess.Config.S3ForcePathStyle = aws.Bool(true)

	l := &Lambda{
		tagKey: os.Getenv("ANTIVIRUS_TAG_KEY"),
		tagValues: LambdaTagValues{
			pass: os.Getenv("ANTIVIRUS_TAG_VALUE_PASS"),
			fail: os.Getenv("ANTIVIRUS_TAG_VALUE_FAIL"),
		},
		scanner:    &ClamAvScanner{},
		s3:         s3.New(sess),
		downloader: s3manager.NewDownloader(sess),
	}

	lambda.Start(l.HandleEvent)
}
