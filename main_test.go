package main

import (
	"errors"
	"io"
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
	args := m.Called(*input.Bucket, *input.Key)
	return 0, args.Error(0)
}

func (m *mockS3Tagger) GetObject(input *s3.GetObjectInput) (out *s3.GetObjectOutput, err error) {
	return &s3.GetObjectOutput{}, nil
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
	downloader.On("Download", "my-bucket", "file-key").Return(nil)

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

	downloader.AssertExpectations(t)
	scanner.AssertExpectations(t)
	mockS3.AssertExpectations(t)
}

func TestHandleEventPass(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", "my-bucket", "file-key").Return(nil)

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

	downloader.AssertExpectations(t)
	scanner.AssertExpectations(t)
	mockS3.AssertExpectations(t)
}

func TestHandleEventAppendsTags(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", "my-bucket", "file-key").Return(nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", tmpFilePath).Return(false, nil)

	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*s3.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("okay")},
		{Key: aws.String("upload-source"), Value: aws.String("online")},
	}, nil)
	mockS3.On("PutObjectTagging", "my-bucket", "file-key", []*s3.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("okay")},
		{Key: aws.String("upload-source"), Value: aws.String("online")},
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("fail")},
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

	downloader.AssertExpectations(t)
	scanner.AssertExpectations(t)
	mockS3.AssertExpectations(t)
}

func TestReportsFailedDownload(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", "my-bucket", "file-key").Return(errors.New("file does not exist"))

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

	downloader.AssertExpectations(t)
	scanner.AssertExpectations(t)
	mockS3.AssertExpectations(t)
}

func TestReportsFailedScan(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", "my-bucket", "file-key").Return(nil)

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

	downloader.AssertExpectations(t)
	scanner.AssertExpectations(t)
	mockS3.AssertExpectations(t)
}

func TestReportsFailedGetTags(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", "my-bucket", "file-key").Return(nil)

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

	downloader.AssertExpectations(t)
	scanner.AssertExpectations(t)
	mockS3.AssertExpectations(t)
}

func TestReportsFailedPutTags(t *testing.T) {
	downloader := new(mockDownloader)
	downloader.On("Download", "my-bucket", "file-key").Return(nil)

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

	downloader.AssertExpectations(t)
	scanner.AssertExpectations(t)
	mockS3.AssertExpectations(t)
}
