package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/aws/aws-lambda-go/lambda"
)

type Updater interface {
	Update() error
}

type Event struct{}

type Response struct {
	Message string `json:"message"`
}

type StorageClient interface {
	GetObject(ctx context.Context, input *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, input *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type Lambda struct {
	bucket          string
	definitionDir   string
	definitionFiles []string
	storageClient   StorageClient
	freshclam       Updater
}

func (l *Lambda) downloadDefinitions(ctx context.Context) error {
	if err := os.Mkdir(l.definitionDir, 0750); err != nil && !os.IsExist(err) {
		return err
	}

	root, err := os.OpenRoot(l.definitionDir)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // no need to check error when closing root

	for _, key := range l.definitionFiles {
		input := &s3.GetObjectInput{
			Bucket: aws.String(l.bucket),
			Key:    aws.String(key),
		}

		output, err := l.storageClient.GetObject(ctx, input)
		if err != nil {
			var nske *types.NoSuchKey
			if errors.As(err, &nske) {
				return nil
			}
			return err
		}

		file, err := root.Create(key)
		if err != nil {
			return err
		}

		if _, err := io.Copy(file, output.Body); err != nil {
			_ = file.Close()
			return err
		}

		_ = output.Body.Close()
		_ = file.Close()
	}

	return nil
}

func (l *Lambda) uploadDefinitions(ctx context.Context) error {
	root, err := os.OpenRoot(l.definitionDir)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // no need to check error when closing root

	for _, key := range l.definitionFiles {
		file, err := root.Open(key)
		if err != nil {
			return err
		}

		input := &s3.PutObjectInput{
			Bucket:               aws.String(l.bucket),
			Key:                  aws.String(key),
			Body:                 file,
			ServerSideEncryption: types.ServerSideEncryptionAes256,
		}

		if _, err := l.storageClient.PutObject(ctx, input); err != nil {
			_ = file.Close()
			return err
		}

		_ = file.Close()
	}

	return nil
}

func (l *Lambda) HandleEvent(ctx context.Context, event Event) (Response, error) {
	log.Print("downloading previous definitions")
	if err := l.downloadDefinitions(ctx); err != nil {
		log.Printf("download definitions: %v", err)
		return Response{}, err
	}

	log.Print("running freshclam")
	if err := l.freshclam.Update(); err != nil {
		log.Printf("freshclam update: %v", err)
		return Response{}, err
	}

	log.Print("uploading previous definitions")
	if err := l.uploadDefinitions(ctx); err != nil {
		log.Printf("upload definitions: %v", err)
		return Response{}, err
	}

	log.Printf("clamav definitions updated")
	return Response{Message: "clamav definitions updated"}, nil
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
		bucket:          os.Getenv("ANTIVIRUS_DEFINITIONS_BUCKET"),
		definitionDir:   "/tmp/clamav",
		definitionFiles: []string{"bytecode.cvd", "daily.cvd", "freshclam.dat", "main.cvd"},
		storageClient:   s3Client,
		freshclam:       &Freshclam{},
	}

	lambda.StartWithOptions(l.HandleEvent, lambda.WithContext(ctx))
}
