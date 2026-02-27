package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockStorageClient struct {
	mock.Mock
}

func (m *mockStorageClient) GetObject(ctx context.Context, input *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	args := m.Called(*input.Bucket, *input.Key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func (m *mockStorageClient) PutObject(ctx context.Context, input *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	body, _ := io.ReadAll(input.Body)
	args := m.Called(*input.Bucket, *input.Key, body, input.ServerSideEncryption)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.PutObjectOutput), args.Error(1)
}

type mockUpdater struct {
	mock.Mock
}

func (m *mockUpdater) Update() error {
	args := m.Called()
	return args.Error(0)
}

func TestHandleEvent(t *testing.T) {
	assert := assert.New(t)

	tempdir, err := os.MkdirTemp("", "opg-s3-antivirus-update")
	if !assert.Nil(err) {
		return
	}
	defer os.RemoveAll(tempdir) //nolint:errcheck // no need to check OS error in this test

	storageClient := &mockStorageClient{}
	freshclam := &mockUpdater{}

	l := &Lambda{
		bucket:          "a-bucket",
		definitionDir:   tempdir,
		definitionFiles: []string{"a", "b"},
		storageClient:   storageClient,
		freshclam:       freshclam,
	}

	storageClient.
		On("GetObject", "a-bucket", "a").
		Return(&s3.GetObjectOutput{
			Body: io.NopCloser(bytes.NewReader([]byte("hello"))),
		}, nil)

	storageClient.
		On("GetObject", "a-bucket", "b").
		Return(&s3.GetObjectOutput{
			Body: io.NopCloser(bytes.NewReader([]byte("there"))),
		}, nil)

	freshclam.
		On("Update").Return(nil)

	storageClient.
		On("PutObject", "a-bucket", "a", []byte("hello"), types.ServerSideEncryptionAes256).
		Return(&s3.PutObjectOutput{}, nil)

	storageClient.
		On("PutObject", "a-bucket", "b", []byte("there"), types.ServerSideEncryptionAes256).
		Return(&s3.PutObjectOutput{}, nil)

	response, err := l.HandleEvent(context.Background(), Event{})
	assert.Nil(err)
	assert.Equal(Response{Message: "clamav definitions updated"}, response)

	mock.AssertExpectationsForObjects(t, storageClient, freshclam)
}

func TestHandleEventFirstRun(t *testing.T) {
	assert := assert.New(t)

	tempdir, err := os.MkdirTemp("", "opg-s3-antivirus-update")
	if !assert.Nil(err) {
		return
	}
	defer os.RemoveAll(tempdir) //nolint:errcheck // no need to check OS error in this test

	for _, name := range []string{"a", "b"} {
		file, err := os.Create(filepath.Join(tempdir, name)) //nolint:gosec // tempdir is a constrained variable
		if !assert.Nil(err) {
			return
		}
		err = file.Close()
		assert.Nil(err)
	}

	storageClient := &mockStorageClient{}
	freshclam := &mockUpdater{}

	l := &Lambda{
		bucket:          "a-bucket",
		definitionDir:   tempdir,
		definitionFiles: []string{"a", "b"},
		storageClient:   storageClient,
		freshclam:       freshclam,
	}

	storageClient.
		On("GetObject", "a-bucket", "a").
		Return(nil, &types.NoSuchKey{})

	freshclam.
		On("Update").Return(nil)

	storageClient.
		On("PutObject", "a-bucket", "a", mock.Anything, types.ServerSideEncryptionAes256).
		Return(&s3.PutObjectOutput{}, nil)

	storageClient.
		On("PutObject", "a-bucket", "b", mock.Anything, types.ServerSideEncryptionAes256).
		Return(&s3.PutObjectOutput{}, nil)

	response, err := l.HandleEvent(context.Background(), Event{})
	assert.Nil(err)
	assert.Equal(Response{Message: "clamav definitions updated"}, response)

	mock.AssertExpectationsForObjects(t, storageClient, freshclam)
}
