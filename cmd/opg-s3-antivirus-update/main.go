package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
)

type Updater interface {
	Update() error
}

type Event struct{}

type Response struct {
	Message string `json:"message"`
}

type Lambda struct {
	bucket          string
	definitionDir   string
	definitionFiles []string
	downloader      s3manageriface.DownloaderAPI
	uploader        s3manageriface.UploaderAPI
	freshclam       Updater
}

func (l *Lambda) downloadDefinitions() error {
	if err := os.Mkdir(l.definitionDir, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	for _, key := range l.definitionFiles {
		input := &s3.GetObjectInput{
			Bucket: aws.String(l.bucket),
			Key:    aws.String(key),
		}

		var buf aws.WriteAtBuffer
		if _, err := l.downloader.Download(&buf, input); err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case s3.ErrCodeNoSuchKey:
					return nil
				}
			}
			return err
		}

		file, err := os.Create(filepath.Join(l.definitionDir, key))
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := file.Write(buf.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (l *Lambda) uploadDefinitions() error {
	for _, key := range l.definitionFiles {
		file, err := os.Open(filepath.Join(l.definitionDir, key))
		if err != nil {
			return err
		}

		input := &s3manager.UploadInput{
			Bucket:               aws.String(l.bucket),
			Key:                  aws.String(key),
			Body:                 file,
			ServerSideEncryption: aws.String("AES256"),
		}

		if _, err := l.uploader.Upload(input); err != nil {
			return err
		}
	}

	return nil
}

func (l *Lambda) HandleEvent(event Event) (Response, error) {
	log.Print("downloading previous definitions")
	if err := l.downloadDefinitions(); err != nil {
		log.Printf("download definitions: %v", err)
		return Response{}, err
	}

	log.Print("running freshclam")
	if err := l.freshclam.Update(); err != nil {
		log.Printf("freshclam update: %v", err)
		return Response{}, err
	}

	log.Print("uploading previous definitions")
	if err := l.uploadDefinitions(); err != nil {
		log.Printf("upload definitions: %v", err)
		return Response{}, err
	}

	log.Printf("clamav definitions updated")
	return Response{Message: "clamav definitions updated"}, nil
}

func main() {
	sess := session.Must(session.NewSession())

	endpoint := os.Getenv("AWS_S3_ENDPOINT")
	sess.Config.Endpoint = &endpoint
	sess.Config.S3ForcePathStyle = aws.Bool(true)

	l := &Lambda{
		bucket:          os.Getenv("ANTIVIRUS_DEFINITIONS_BUCKET"),
		definitionDir:   "/tmp/clamav",
		definitionFiles: []string{"bytecode.cvd", "daily.cvd", "freshclam.dat", "main.cvd"},
		downloader:      s3manager.NewDownloader(sess),
		uploader:        s3manager.NewUploader(sess),
		freshclam:       &Freshclam{},
	}

	lambda.Start(l.HandleEvent)
}
