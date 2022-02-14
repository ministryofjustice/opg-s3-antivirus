package main

import (
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/stretchr/testify/assert"
)

type mockDownloader struct {
	calls int
	last  struct {
		bucket string
		key    string
	}
	response struct {
		err error
	}
}

func (m *mockDownloader) Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error) {
	m.calls++
	m.last.bucket = *input.Bucket
	m.last.key = *input.Key

	return 0, m.response.err
}

type mockScanner struct {
	calls    int
	response struct {
		pass bool
		err  error
	}
	last struct {
		path string
	}
}

func (m *mockScanner) ScanFile(path string) (bool, error) {
	m.last.path = path
	m.calls++

	return m.response.pass, m.response.err
}

type mockS3Tagger struct {
	*s3.S3

	get struct {
		calls int
		last  struct {
			bucket string
			key    string
		}
		response struct {
			err    error
			tagSet []*s3.Tag
		}
	}

	put struct {
		calls int
		last  struct {
			bucket string
			key    string
			tagSet []*s3.Tag
		}
		response struct {
			err error
		}
	}
}

func (m *mockS3Tagger) GetObjectTagging(input *s3.GetObjectTaggingInput) (*s3.GetObjectTaggingOutput, error) {
	m.get.calls++
	m.get.last.bucket = *input.Bucket
	m.get.last.key = *input.Key

	return &s3.GetObjectTaggingOutput{
		TagSet: m.get.response.tagSet,
	}, m.get.response.err
}

func (m *mockS3Tagger) PutObjectTagging(input *s3.PutObjectTaggingInput) (*s3.PutObjectTaggingOutput, error) {
	m.put.calls++
	m.put.last.bucket = *input.Bucket
	m.put.last.key = *input.Key
	m.put.last.tagSet = input.Tagging.TagSet

	return &s3.PutObjectTaggingOutput{}, m.put.response.err
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
	downloader := &mockDownloader{}
	scanner := &mockScanner{}
	mockS3 := &mockS3Tagger{}

	l := &Lambda{}
	l.tagKey = "VIRUS_SCAN"
	l.tagValues.fail = "failed"
	l.downloader = downloader
	l.scanner = scanner
	l.s3 = mockS3
	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, nil, err)
	assert.Equal(t, "my-bucket", downloader.last.bucket)
	assert.Equal(t, "file-key", downloader.last.key)
	assert.Equal(t, "my-bucket", mockS3.put.last.bucket)
	assert.Equal(t, "file-key", mockS3.put.last.key)
	assert.Equal(t, []*s3.Tag{{Key: aws.String("VIRUS_SCAN"), Value: aws.String("failed")}}, mockS3.put.last.tagSet)
	assert.Equal(t, MyResponse{"scanning complete, tagged with failed"}, response)
}

func TestHandleEventPass(t *testing.T) {
	downloader := &mockDownloader{}
	scanner := &mockScanner{}
	scanner.response.pass = true
	mockS3 := &mockS3Tagger{}

	l := &Lambda{}
	l.tagKey = "VIRUS_SCAN"
	l.tagValues.pass = "okay"
	l.downloader = downloader
	l.scanner = scanner
	l.s3 = mockS3
	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, nil, err)
	assert.Equal(t, "my-bucket", downloader.last.bucket)
	assert.Equal(t, "file-key", downloader.last.key)
	assert.Equal(t, "my-bucket", mockS3.put.last.bucket)
	assert.Equal(t, "file-key", mockS3.put.last.key)
	assert.Equal(t, []*s3.Tag{{Key: aws.String("VIRUS_SCAN"), Value: aws.String("okay")}}, mockS3.put.last.tagSet)
	assert.Equal(t, MyResponse{"scanning complete, tagged with okay"}, response)
}

func TestHandleEventAppendsTags(t *testing.T) {
	downloader := &mockDownloader{}
	scanner := &mockScanner{}
	mockS3 := &mockS3Tagger{}
	mockS3.get.response.tagSet = []*s3.Tag{
		{
			Key:   aws.String("VIRUS_SCAN"),
			Value: aws.String("okay"),
		},
		{
			Key:   aws.String("upload-source"),
			Value: aws.String("online"),
		},
	}

	l := &Lambda{}
	l.tagKey = "VIRUS_SCAN"
	l.tagValues.fail = "fail"
	l.downloader = downloader
	l.scanner = scanner
	l.s3 = mockS3
	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, nil, err)
	assert.Equal(t, "my-bucket", mockS3.put.last.bucket)
	assert.Equal(t, "file-key", mockS3.put.last.key)
	assert.Equal(t, []*s3.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("okay")},
		{Key: aws.String("upload-source"), Value: aws.String("online")},
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("fail")},
	}, mockS3.put.last.tagSet)
	assert.Equal(t, MyResponse{"scanning complete, tagged with fail"}, response)
}

func TestReportsFailedDownload(t *testing.T) {
	downloader := &mockDownloader{}
	downloader.response.err = errors.New("file does not exist")
	scanner := &mockScanner{}
	mockS3 := &mockS3Tagger{}

	l := &Lambda{}
	l.downloader = downloader
	l.scanner = scanner
	l.s3 = mockS3
	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, "failed to download file, file does not exist", err.Error())
	assert.Equal(t, MyResponse{""}, response)
	assert.Equal(t, "my-bucket", downloader.last.bucket)
	assert.Equal(t, "file-key", downloader.last.key)
	assert.Equal(t, 0, mockS3.get.calls)
	assert.Equal(t, 0, scanner.calls)
}

func TestReportsFailedScan(t *testing.T) {
	downloader := &mockDownloader{}
	scanner := &mockScanner{}
	scanner.response.err = errors.New("clamav returned exit code 82")
	mockS3 := &mockS3Tagger{}

	l := &Lambda{}
	l.downloader = downloader
	l.scanner = scanner
	l.s3 = mockS3
	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, "clamav returned exit code 82", err.Error())
	assert.Equal(t, MyResponse{""}, response)
	assert.Equal(t, tmpFilePath, scanner.last.path)
	assert.Equal(t, 0, mockS3.get.calls)
}

func TestReportsFailedGetTags(t *testing.T) {
	downloader := &mockDownloader{}
	scanner := &mockScanner{}
	mockS3 := &mockS3Tagger{}
	mockS3.get.response.err = errors.New("file does not exist")

	l := &Lambda{}
	l.downloader = downloader
	l.scanner = scanner
	l.s3 = mockS3
	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, "failed to get tags, file does not exist", err.Error())
	assert.Equal(t, MyResponse{""}, response)
	assert.Equal(t, "my-bucket", mockS3.get.last.bucket)
	assert.Equal(t, "file-key", mockS3.get.last.key)
	assert.Equal(t, 0, mockS3.put.calls)
}

func TestReportsFailedPutTags(t *testing.T) {
	downloader := &mockDownloader{}
	scanner := &mockScanner{}
	mockS3 := &mockS3Tagger{}
	mockS3.put.response.err = errors.New("invalid tag")

	l := &Lambda{}
	l.downloader = downloader
	l.scanner = scanner
	l.s3 = mockS3
	response, err := l.HandleEvent(createTestEvent())

	assert.Equal(t, "failed to write tags, invalid tag", err.Error())
	assert.Equal(t, MyResponse{""}, response)
	assert.Equal(t, "my-bucket", mockS3.put.last.bucket)
	assert.Equal(t, "file-key", mockS3.put.last.key)
}
