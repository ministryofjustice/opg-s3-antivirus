package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/aws/aws-lambda-go/lambda"
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
	Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*manager.Downloader)) (n int64, err error)
}

type Tagger interface {
	GetObjectTagging(ctx context.Context, params *s3.GetObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error)
	PutObjectTagging(ctx context.Context, params *s3.PutObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error)
}

type Scanner interface {
	StartDaemon() error
	ScanFile(path string) (bool, error)
}

type Lambda struct {
	tagKey     string
	tagValues  LambdaTagValues
	scanner    Scanner
	s3         Tagger
	downloader Downloader
}

func (l *Lambda) downloadDefinitions(ctx context.Context, dir, bucket string, files []string) error {
	if err := os.Mkdir(dir, 0750); err != nil && !os.IsExist(err) {
		return err
	}

	for _, key := range files {
		input := &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}

		w := manager.NewWriteAtBuffer([]byte{})
		if _, err := l.downloader.Download(ctx, w, input); err != nil {
			return err
		}

		file, err := os.Create(filepath.Join(dir, key)) //nolint:gosec // variables are fixed so inclusion is not risky
		if err != nil {
			return err
		}
		defer file.Close() //nolint:errcheck // no need to check error when closing file

		if _, err := file.Write(w.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (l *Lambda) downloadFile(ctx context.Context, f io.WriterAt, bucket string, key string) error {
	_, err := l.downloader.Download(ctx, f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}

func (l *Lambda) tagFile(ctx context.Context, bucket string, key string, status string) error {
	tagging, err := l.s3.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
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
		tagging.TagSet = append(tagging.TagSet, types.Tag{
			Key:   aws.String(l.tagKey),
			Value: aws.String(status),
		})
	}

	_, err = l.s3.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Tagging: &types.Tagging{
			TagSet: tagging.TagSet,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to write tags: %w", err)
	}

	return nil
}

func (l *Lambda) HandleEvent(ctx context.Context, event ObjectCreatedEvent) (MyResponse, error) {
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

	if err := l.downloadFile(ctx, f, bucketName, objectKey); err != nil {
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
	if err := l.tagFile(ctx, bucketName, objectKey, statusString); err != nil {
		log.Print(err)
		return MyResponse{}, err
	}

	log.Printf("scanning complete, tagged with %s", statusString)
	return MyResponse{Message: fmt.Sprintf("scanning complete, tagged with %s", statusString)}, nil
}

func main() {
	ctx := context.Background()

	awsRegion := os.Getenv("AWS_REGION")
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(awsRegion),
	)
	if err != nil {
		log.Printf("error building aws config: %v", err)
	}

	if endpoint, ok := os.LookupEnv("AWS_S3_ENDPOINT"); ok {
		cfg.BaseEndpoint = &endpoint
	}

	s3Client := s3.NewFromConfig(cfg, func(u *s3.Options) {
		u.UsePathStyle = true
	})

	l := &Lambda{
		tagKey: os.Getenv("ANTIVIRUS_TAG_KEY"),
		tagValues: LambdaTagValues{
			pass: os.Getenv("ANTIVIRUS_TAG_VALUE_PASS"),
			fail: os.Getenv("ANTIVIRUS_TAG_VALUE_FAIL"),
		},
		scanner:    &ClamAvScanner{},
		s3:         s3Client,
		downloader: manager.NewDownloader(s3Client),
	}

	log.Print("downloading virus definitions")
	err = l.downloadDefinitions(ctx, "/tmp/clamav", os.Getenv("ANTIVIRUS_DEFINITIONS_BUCKET"), []string{"bytecode.cvd", "daily.cvd", "freshclam.dat", "main.cvd"})
	if err != nil {
		log.Printf("downloading new definitions failed: %v", err)
	}

	err = l.scanner.StartDaemon()
	if err != nil {
		log.Printf("error starting damon: %v", err)
	}

	lambda.StartWithOptions(l.HandleEvent, lambda.WithContext(ctx))
}
