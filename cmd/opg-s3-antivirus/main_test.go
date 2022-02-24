package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDownloader struct {
	mock.Mock
}

func (m *mockDownloader) Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error) {
	args := m.Called(w, *input.Bucket, *input.Key)
	return 0, args.Error(0)
}

type mockScanner struct {
	mock.Mock
}

func (m *mockScanner) ScanFile(path string) (bool, error) {
	args := m.Called(path)
	return args.Bool(0), args.Error(1)
}

type mockS3Tagger struct {
	*s3.S3
	mock.Mock
}

func (m *mockS3Tagger) GetObjectTagging(input *s3.GetObjectTaggingInput) (*s3.GetObjectTaggingOutput, error) {
	args := m.Called(*input.Bucket, *input.Key)

	return &s3.GetObjectTaggingOutput{
		TagSet: args.Get(0).([]*s3.Tag),
	}, args.Error(1)
}

func (m *mockS3Tagger) PutObjectTagging(input *s3.PutObjectTaggingInput) (*s3.PutObjectTaggingOutput, error) {
	args := m.Called(*input.Bucket, *input.Key, input.Tagging.TagSet)

	return &s3.PutObjectTaggingOutput{}, args.Error(0)
}

func createTestEvent() ObjectCreatedEvent {
	event := ObjectCreatedEvent{}
	eventRecord := EventRecord{}
	eventRecord.S3.Bucket.Name = "my-bucket"
	eventRecord.S3.Object.Key = "file-key"
	event.Records = append(event.Records, eventRecord)

	return event
}

func TestHandleEvent(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", mock.Anything, "my-bucket", "file-key").Return(nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", tmpFilePath).Return(false, nil)

	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*s3.Tag{}, nil)
	mockS3.On("PutObjectTagging", "my-bucket", "file-key", []*s3.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("failed")},
	}).Return(nil)

	l := &Lambda{
		tagKey: "VIRUS_SCAN",
		tagValues: LambdaTagValues{
			fail: "failed",
		},
		downloader: downloader,
		scanner:    scanner,
		s3:         mockS3,
	}

	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, nil, err)
	assert.Equal(t, MyResponse{"scanning complete, tagged with failed"}, response)

	mock.AssertExpectationsForObjects(t, downloader, scanner, mockS3)
}

func TestHandleEventPass(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", mock.Anything, "my-bucket", "file-key").Return(nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", tmpFilePath).Return(true, nil)

	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*s3.Tag{}, nil)
	mockS3.On("PutObjectTagging", "my-bucket", "file-key", []*s3.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("okay")},
	}).Return(nil)

	l := &Lambda{
		tagKey: "VIRUS_SCAN",
		tagValues: LambdaTagValues{
			pass: "okay",
		},
		downloader: downloader,
		scanner:    scanner,
		s3:         mockS3,
	}

	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, nil, err)
	assert.Equal(t, MyResponse{"scanning complete, tagged with okay"}, response)

	mock.AssertExpectationsForObjects(t, downloader, scanner, mockS3)
}

func TestHandleEventHandlesDuplicateTags(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", mock.Anything, "my-bucket", "file-key").Return(nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", tmpFilePath).Return(false, nil)

	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*s3.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("okay")},
		{Key: aws.String("upload-source"), Value: aws.String("online")},
	}, nil)
	mockS3.On("PutObjectTagging", "my-bucket", "file-key", []*s3.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("fail")},
		{Key: aws.String("upload-source"), Value: aws.String("online")},
	}).Return(nil)

	l := &Lambda{
		tagKey: "VIRUS_SCAN",
		tagValues: LambdaTagValues{
			fail: "fail",
		},
		downloader: downloader,
		scanner:    scanner,
		s3:         mockS3,
	}

	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, nil, err)
	assert.Equal(t, MyResponse{"scanning complete, tagged with fail"}, response)

	mock.AssertExpectationsForObjects(t, downloader, scanner, mockS3)
}

func TestReportsFailedDownload(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", mock.Anything, "my-bucket", "file-key").Return(errors.New("file does not exist"))

	scanner := new(mockScanner)

	mockS3 := new(mockS3Tagger)

	l := &Lambda{
		downloader: downloader,
		scanner:    scanner,
		s3:         mockS3,
	}

	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, "failed to download file, file does not exist", err.Error())
	assert.Equal(t, MyResponse{""}, response)

	mock.AssertExpectationsForObjects(t, downloader, scanner, mockS3)
}

func TestReportsFailedScan(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", mock.Anything, "my-bucket", "file-key").Return(nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", tmpFilePath).Return(false, errors.New("clamav returned exit code 82"))

	mockS3 := new(mockS3Tagger)

	l := &Lambda{
		downloader: downloader,
		scanner:    scanner,
		s3:         mockS3,
	}

	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, "clamav returned exit code 82", err.Error())
	assert.Equal(t, MyResponse{""}, response)

	mock.AssertExpectationsForObjects(t, downloader, scanner, mockS3)
}

func TestReportsFailedGetTags(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", mock.Anything, "my-bucket", "file-key").Return(nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", tmpFilePath).Return(false, nil)

	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*s3.Tag{}, errors.New("file does not exist"))

	l := &Lambda{
		downloader: downloader,
		scanner:    scanner,
		s3:         mockS3,
	}

	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, "failed to get tags, file does not exist", err.Error())
	assert.Equal(t, MyResponse{""}, response)

	mock.AssertExpectationsForObjects(t, downloader, scanner, mockS3)
}

func TestReportsFailedPutTags(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", mock.Anything, "my-bucket", "file-key").Return(nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", tmpFilePath).Return(false, nil)

	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*s3.Tag{}, nil)
	mockS3.On("PutObjectTagging", "my-bucket", "file-key", []*s3.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("fail")},
	}).Return(errors.New("invalid tag"))

	l := &Lambda{
		tagKey: "VIRUS_SCAN",
		tagValues: LambdaTagValues{
			fail: "fail",
		},
		downloader: downloader,
		scanner:    scanner,
		s3:         mockS3,
	}

	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, "failed to write tags, invalid tag", err.Error())
	assert.Equal(t, MyResponse{""}, response)

	mock.AssertExpectationsForObjects(t, downloader, scanner, mockS3)
}

func TestDownloadDefinitions(t *testing.T) {
	assert := assert.New(t)

	tempdir, err := os.MkdirTemp("", "opg-s3-antivirus")
	if !assert.Nil(err) {
		return
	}
	defer os.RemoveAll(tempdir)

	downloader := &mockDownloader{}

	l := &Lambda{
		downloader: downloader,
	}

	downloader.
		On("Download", mock.Anything, "a-bucket", "a").
		Run(func(args mock.Arguments) {
			w := args[0].(io.WriterAt)
			_, _ = w.WriteAt([]byte("hello"), 0)
		}).
		Return(nil)

	downloader.
		On("Download", mock.Anything, "a-bucket", "b").
		Run(func(args mock.Arguments) {
			w := args[0].(io.WriterAt)
			_, _ = w.WriteAt([]byte("there"), 0)
		}).
		Return(nil)

	err = l.downloadDefinitions(tempdir, "a-bucket", []string{"a", "b"})
	assert.Nil(err)

	fileA, _ := os.ReadFile(filepath.Join(tempdir, "a"))
	assert.Equal([]byte("hello"), fileA)

	fileB, _ := os.ReadFile(filepath.Join(tempdir, "b"))
	assert.Equal([]byte("there"), fileB)

	mock.AssertExpectationsForObjects(t, downloader)
}

func TestDownloadDefinitionsWhenError(t *testing.T) {
	assert := assert.New(t)

	tempdir, err := os.MkdirTemp("", "opg-s3-antivirus")
	if !assert.Nil(err) {
		return
	}
	defer os.RemoveAll(tempdir)

	expectedErr := errors.New("what")

	downloader := &mockDownloader{}

	l := &Lambda{
		downloader: downloader,
	}

	downloader.
		On("Download", mock.Anything, "a-bucket", "a").
		Return(expectedErr)

	err = l.downloadDefinitions(tempdir, "a-bucket", []string{"a", "b"})
	assert.Equal(expectedErr, err)

	_, err = os.Stat(filepath.Join(tempdir, "a"))
	assert.True(os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(tempdir, "b"))
	assert.True(os.IsNotExist(err))

	mock.AssertExpectationsForObjects(t, downloader)
}
