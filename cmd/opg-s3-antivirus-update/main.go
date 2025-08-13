package main

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-lambda-go/lambda"
)

type Updater interface {
	Update() error
}

type Event struct{}

type Response struct {
	Message string `json:"message"`
}

type Downloader interface {
	Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*manager.Downloader)) (n int64, err error)
}

type Uploader interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

type Lambda struct {
	bucket          string
	definitionDir   string
	definitionFiles []string
	downloader      Downloader
	uploader        Uploader
	freshclam       Updater
}

func (l *Lambda) downloadDefinitions(ctx context.Context) error {
	if err := os.Mkdir(l.definitionDir, 0750); err != nil && !os.IsExist(err) {
		return err
	}

	for _, key := range l.definitionFiles {
		input := &s3.GetObjectInput{
			Bucket: aws.String(l.bucket),
			Key:    aws.String(key),
		}

		w := manager.NewWriteAtBuffer([]byte{})
		if _, err := l.downloader.Download(ctx, w, input); err != nil {
			var nske *types.NoSuchKey
			if errors.As(err, &nske) {
				return nil
			}
			return err
		}

		file, err := os.Create(filepath.Join(l.definitionDir, key)) //nolint:gosec // variables are fixed so inclusion is not risky
		if err != nil {
			return err
		}

		defer func() {
			err := file.Close()
			if err != nil {
				log.Printf("error whilst closing file: %s", err.Error())
			}
		}()

		if _, err := file.Write(w.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (l *Lambda) uploadDefinitions(ctx context.Context) error {
	for _, key := range l.definitionFiles {
		file, err := os.Open(filepath.Join(l.definitionDir, key)) //nolint:gosec // variables are fixed so inclusion is not risky
		if err != nil {
			return err
		}

		input := &s3.PutObjectInput{
			Bucket:               aws.String(l.bucket),
			Key:                  aws.String(key),
			Body:                 file,
			ServerSideEncryption: types.ServerSideEncryptionAes256,
		}

		if _, err := l.uploader.Upload(ctx, input); err != nil {
			return err
		}
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
		downloader:      manager.NewDownloader(s3Client),
		uploader:        manager.NewUploader(s3Client),
		freshclam:       &Freshclam{},
	}

	lambda.StartWithOptions(l.HandleEvent, lambda.WithContext(ctx))
}
