package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockScanner struct {
	mock.Mock
}

func (m *mockScanner) StartDaemon() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockScanner) ScanFile(path string) (bool, error) {
	args := m.Called(path)
	return args.Bool(0), args.Error(1)
}

type mockS3Tagger struct {
	mock.Mock
}

func (m *mockS3Tagger) GetObjectTagging(ctx context.Context, params *s3.GetObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
	args := m.Called(*params.Bucket, *params.Key)

	ptrTags := args.Get(0).([]*types.Tag)
	valTags := make([]types.Tag, len(ptrTags))
	for i, ptr := range ptrTags {
		if ptr != nil {
			valTags[i] = *ptr
		}
	}

	return &s3.GetObjectTaggingOutput{
		TagSet: valTags,
	}, args.Error(1)
}

func (m *mockS3Tagger) PutObjectTagging(ctx context.Context, params *s3.PutObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
	ptrTags := make([]*types.Tag, len(params.Tagging.TagSet))
	for i, v := range params.Tagging.TagSet {
		ptrTags[i] = &v
	}

	args := m.Called(*params.Bucket, *params.Key, ptrTags)

	return &s3.PutObjectTaggingOutput{}, args.Error(0)
}

func (m *mockS3Tagger) GetObject(ctx context.Context, input *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	args := m.Called(*input.Bucket, *input.Key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func createTestEvent() ObjectCreatedEvent {
	event := ObjectCreatedEvent{}
	eventRecord := EventRecord{}
	eventRecord.S3.Bucket.Name = "my-bucket"
	eventRecord.S3.Object.Key = "file%2Dkey"
	event.Records = append(event.Records, eventRecord)

	return event
}

func TestHandleEvent(t *testing.T) {
	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObject", "my-bucket", "file-key").Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader([]byte("file content"))),
	}, nil)

	scanner := new(mockScanner)
	scanner.
		On("ScanFile", mock.Anything).
		Return(false, nil)

	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*types.Tag{}, nil)
	mockS3.On("PutObjectTagging", "my-bucket", "file-key", []*types.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("failed")},
	}).Return(nil)

	l := &Lambda{
		tagKey: "VIRUS_SCAN",
		tagValues: LambdaTagValues{
			fail: "failed",
		},
		scanner: scanner,
		s3:      mockS3,
	}

	response, err := l.HandleEvent(context.Background(), createTestEvent())

	assert.Equal(t, nil, err)
	assert.Equal(t, MyResponse{"scanning complete, tagged with failed"}, response)

	mock.AssertExpectationsForObjects(t, scanner, mockS3)
}

func TestHandleEventPass(t *testing.T) {
	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObject", "my-bucket", "file-key").Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader([]byte("file content"))),
	}, nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", mock.Anything).Return(true, nil)

	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*types.Tag{}, nil)
	mockS3.On("PutObjectTagging", "my-bucket", "file-key", []*types.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("okay")},
	}).Return(nil)

	l := &Lambda{
		tagKey: "VIRUS_SCAN",
		tagValues: LambdaTagValues{
			pass: "okay",
		},
		scanner: scanner,
		s3:      mockS3,
	}

	response, err := l.HandleEvent(context.Background(), createTestEvent())

	assert.Equal(t, nil, err)
	assert.Equal(t, MyResponse{"scanning complete, tagged with okay"}, response)

	mock.AssertExpectationsForObjects(t, scanner, mockS3)
}

func TestHandleEventHandlesDuplicateTags(t *testing.T) {
	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObject", "my-bucket", "file-key").Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader([]byte("file content"))),
	}, nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", mock.Anything).Return(false, nil)

	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*types.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("okay")},
		{Key: aws.String("upload-source"), Value: aws.String("online")},
	}, nil)
	mockS3.On("PutObjectTagging", "my-bucket", "file-key", []*types.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("fail")},
		{Key: aws.String("upload-source"), Value: aws.String("online")},
	}).Return(nil)

	l := &Lambda{
		tagKey: "VIRUS_SCAN",
		tagValues: LambdaTagValues{
			fail: "fail",
		},
		scanner: scanner,
		s3:      mockS3,
	}

	response, err := l.HandleEvent(context.Background(), createTestEvent())

	assert.Equal(t, nil, err)
	assert.Equal(t, MyResponse{"scanning complete, tagged with fail"}, response)

	mock.AssertExpectationsForObjects(t, scanner, mockS3)
}

func TestReportsFailedUnescape(t *testing.T) {
	scanner := new(mockScanner)
	mockS3 := new(mockS3Tagger)

	l := &Lambda{
		scanner: scanner,
		s3:      mockS3,
	}

	event := createTestEvent()
	event.Records[0].S3.Object.Key = "bad key%%%"
	response, err := l.HandleEvent(context.Background(), event)

	assert.Equal(t, "failed to unescape object key: invalid URL escape \"%%%\"", err.Error())
	assert.Equal(t, MyResponse{""}, response)

	mock.AssertExpectationsForObjects(t, scanner, mockS3)
}

func TestReportsFailedDownload(t *testing.T) {
	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObject", "my-bucket", "file-key").Return(nil, errors.New("file does not exist"))

	scanner := new(mockScanner)

	l := &Lambda{
		scanner: scanner,
		s3:      mockS3,
	}

	response, err := l.HandleEvent(context.Background(), createTestEvent())

	assert.Equal(t, "failed to download file: file does not exist", err.Error())
	assert.Equal(t, MyResponse{""}, response)

	mock.AssertExpectationsForObjects(t, scanner, mockS3)
}

func TestReportsFailedScan(t *testing.T) {
	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObject", "my-bucket", "file-key").Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader([]byte("file content"))),
	}, nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", mock.Anything).Return(false, errors.New("clamav returned exit code 82"))

	l := &Lambda{
		scanner: scanner,
		s3:      mockS3,
	}

	response, err := l.HandleEvent(context.Background(), createTestEvent())

	assert.Equal(t, "clamav returned exit code 82", err.Error())
	assert.Equal(t, MyResponse{""}, response)

	mock.AssertExpectationsForObjects(t, scanner, mockS3)
}

func TestReportsFailedGetTags(t *testing.T) {
	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObject", "my-bucket", "file-key").Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader([]byte("file content"))),
	}, nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", mock.Anything).Return(false, nil)

	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*types.Tag{}, errors.New("file does not exist"))

	l := &Lambda{
		scanner: scanner,
		s3:      mockS3,
	}

	response, err := l.HandleEvent(context.Background(), createTestEvent())

	assert.Equal(t, "failed to get tags: file does not exist", err.Error())
	assert.Equal(t, MyResponse{""}, response)

	mock.AssertExpectationsForObjects(t, scanner, mockS3)
}

func TestReportsFailedPutTags(t *testing.T) {
	mockS3 := new(mockS3Tagger)
	mockS3.On("GetObject", "my-bucket", "file-key").Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader([]byte("file content"))),
	}, nil)

	scanner := new(mockScanner)
	scanner.On("ScanFile", mock.Anything).Return(false, nil)

	mockS3.On("GetObjectTagging", "my-bucket", "file-key").Return([]*types.Tag{}, nil)
	mockS3.On("PutObjectTagging", "my-bucket", "file-key", []*types.Tag{
		{Key: aws.String("VIRUS_SCAN"), Value: aws.String("fail")},
	}).Return(errors.New("invalid tag"))

	l := &Lambda{
		tagKey: "VIRUS_SCAN",
		tagValues: LambdaTagValues{
			fail: "fail",
		},
		scanner: scanner,
		s3:      mockS3,
	}

	response, err := l.HandleEvent(context.Background(), createTestEvent())

	assert.Equal(t, "failed to write tags: invalid tag", err.Error())
	assert.Equal(t, MyResponse{""}, response)

	mock.AssertExpectationsForObjects(t, scanner, mockS3)
}

func TestDownloadDefinitions(t *testing.T) {
	assert := assert.New(t)

	tempdir, err := os.MkdirTemp("", "opg-s3-antivirus")
	if !assert.Nil(err) {
		return
	}
	defer os.RemoveAll(tempdir) //nolint:errcheck // no need to check OS error in this test

	mockS3 := &mockS3Tagger{}

	l := &Lambda{
		s3: mockS3,
	}

	mockS3.
		On("GetObject", "a-bucket", "a").
		Return(&s3.GetObjectOutput{
			Body: io.NopCloser(bytes.NewReader([]byte("hello"))),
		}, nil)

	mockS3.
		On("GetObject", "a-bucket", "b").
		Return(&s3.GetObjectOutput{
			Body: io.NopCloser(bytes.NewReader([]byte("there"))),
		}, nil)

	err = l.downloadDefinitions(context.Background(), tempdir, "a-bucket", []string{"a", "b"})
	assert.Nil(err)

	fileA, _ := os.ReadFile(filepath.Join(tempdir, "a")) //nolint:gosec // tempdir is a constrained variable
	assert.Equal([]byte("hello"), fileA)

	fileB, _ := os.ReadFile(filepath.Join(tempdir, "b")) //nolint:gosec // tempdir is a constrained variable
	assert.Equal([]byte("there"), fileB)

	mock.AssertExpectationsForObjects(t, mockS3)
}

func TestDownloadDefinitionsWhenError(t *testing.T) {
	assert := assert.New(t)

	tempdir, err := os.MkdirTemp("", "opg-s3-antivirus")
	if !assert.Nil(err) {
		return
	}
	defer os.RemoveAll(tempdir) //nolint:errcheck // no need to check OS error in this test

	expectedErr := errors.New("what")

	mockS3 := &mockS3Tagger{}

	l := &Lambda{
		s3: mockS3,
	}

	mockS3.
		On("GetObject", "a-bucket", "a").
		Return(nil, expectedErr)

	err = l.downloadDefinitions(context.Background(), tempdir, "a-bucket", []string{"a", "b"})
	assert.Equal(expectedErr, err)

	_, err = os.Stat(filepath.Join(tempdir, "a"))
	assert.True(errors.Is(err, fs.ErrNotExist))

	_, err = os.Stat(filepath.Join(tempdir, "b"))
	assert.True(errors.Is(err, fs.ErrNotExist))

	mock.AssertExpectationsForObjects(t, mockS3)
}
