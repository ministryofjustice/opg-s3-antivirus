package main

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"

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
	StartDaemon() error
	ScanFile(path string) (bool, error)
}

type Lambda struct {
	tagKey     string
	tagValues  LambdaTagValues
	scanner    Scanner
	s3         s3iface.S3API
	downloader Downloader
}

func (l *Lambda) downloadDefinitions(dir, bucket string, files []string) error {
	if err := os.Mkdir(dir, 0750); err != nil && !os.IsExist(err) {
		return err
	}

	for _, key := range files {
		input := &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}

		var buf aws.WriteAtBuffer
		if _, err := l.downloader.Download(&buf, input); err != nil {
			return err
		}

		file, err := os.Create(filepath.Join(dir, key)) //nolint:gosec // variables are fixed so inclusion is not risky
		if err != nil {
			return err
		}
		defer file.Close() //nolint:errcheck // no need to check error when closing file

		if _, err := file.Write(buf.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (l *Lambda) downloadFile(f io.WriterAt, bucket string, key string) error {
	_, err := l.downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}

func (l *Lambda) tagFile(bucket string, key string, status string) error {
	tagging, err := l.s3.GetObjectTagging(&s3.GetObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	is_tag_set := false
	for index, tag := range tagging.TagSet {
		if *tag.Key == l.tagKey {
			tagging.TagSet[index].Value = aws.String(status)
			is_tag_set = true
		}
	}

	if !is_tag_set {
		tagging.TagSet = append(tagging.TagSet, &s3.Tag{
			Key:   aws.String(l.tagKey),
			Value: aws.String(status),
		})
	}

	_, err = l.s3.PutObjectTagging(&s3.PutObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Tagging: &s3.Tagging{
			TagSet: tagging.TagSet,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to write tags: %w", err)
	}

	return nil
}

func (l *Lambda) HandleEvent(event ObjectCreatedEvent) (MyResponse, error) {
	bucketName := event.Records[0].S3.Bucket.Name
	objectKey, err := url.QueryUnescape(event.Records[0].S3.Object.Key)

	if err != nil {
		return MyResponse{}, fmt.Errorf("failed to unescape object key: %w", err)
	}

	log.Printf("downloading %s from %s", objectKey, bucketName)

	f, err := os.CreateTemp("/tmp", "file")
	if err != nil {
		return MyResponse{}, fmt.Errorf("failed to create file: %w", err)
	}

	defer func() {
		err := os.Remove(f.Name())
		if err != nil {
			log.Printf("error whilst removing file: %s", err.Error())
		}
	}()

	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("error whilst closing file: %s", err.Error())
		}
	}()

	if err := l.downloadFile(f, bucketName, objectKey); err != nil {
		log.Print(err)
		return MyResponse{}, err
	}

	log.Printf("file downloaded, scanning file")

	status, err := l.scanner.ScanFile(f.Name())
	if err != nil {
		log.Print(err)
		return MyResponse{}, err
	}

	statusString := l.tagValues.fail
	if status {
		statusString = l.tagValues.pass
	}

	log.Printf("scan complete, status %s, tagging file", statusString)
	if err := l.tagFile(bucketName, objectKey, statusString); err != nil {
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

	log.Print("downloading virus definitions")
	err := l.downloadDefinitions("/tmp/clamav", os.Getenv("ANTIVIRUS_DEFINITIONS_BUCKET"), []string{"bytecode.cvd", "daily.cvd", "freshclam.dat", "main.cvd"})
	if err != nil {
		log.Printf("downloading new definitions failed: %v", err)
	}

	err = l.scanner.StartDaemon()
	if err != nil {
		log.Printf("error starting damon: %v", err)
	}

	lambda.Start(l.HandleEvent)
}
