package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDownloader struct {
	Downloader
	mock.Mock
}

func (m *mockDownloader) Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*manager.Downloader)) (n int64, err error) {
	args := m.Called(w, *input.Bucket, *input.Key)
	return 0, args.Error(0)
}

type mockUploader struct {
	Uploader
	mock.Mock
}

func (m *mockUploader) Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	args := m.Called(*input.Bucket, *input.Key, input.Body, types.ServerSideEncryptionAes256)
	return nil, args.Error(0)
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

	downloader := &mockDownloader{}
	uploader := &mockUploader{}
	freshclam := &mockUpdater{}

	l := &Lambda{
		bucket:          "a-bucket",
		definitionDir:   tempdir,
		definitionFiles: []string{"a", "b"},
		downloader:      downloader,
		uploader:        uploader,
		freshclam:       freshclam,
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

	freshclam.
		On("Update").Return(nil)

	uploader.
		On("Upload", "a-bucket", "a", mock.Anything, types.ServerSideEncryptionAes256).
		Run(func(args mock.Arguments) {
			r := args[2].(io.Reader)
			data, _ := io.ReadAll(r)
			assert.Equal([]byte("hello"), data)
		}).
		Return(nil)

	uploader.
		On("Upload", "a-bucket", "b", mock.Anything, types.ServerSideEncryptionAes256).
		Run(func(args mock.Arguments) {
			r := args[2].(io.Reader)
			data, _ := io.ReadAll(r)
			assert.Equal([]byte("there"), data)
		}).
		Return(nil)

	response, err := l.HandleEvent(context.Background(), Event{})
	assert.Nil(err)
	assert.Equal(Response{Message: "clamav definitions updated"}, response)

	mock.AssertExpectationsForObjects(t, downloader, uploader, freshclam)
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

	downloader := &mockDownloader{}
	uploader := &mockUploader{}
	freshclam := &mockUpdater{}

	l := &Lambda{
		bucket:          "a-bucket",
		definitionDir:   tempdir,
		definitionFiles: []string{"a", "b"},
		downloader:      downloader,
		uploader:        uploader,
		freshclam:       freshclam,
	}

	downloader.
		On("Download", mock.Anything, "a-bucket", "a").
		Return(&types.NoSuchKey{})

	freshclam.
		On("Update").Return(nil)

	uploader.
		On("Upload", "a-bucket", "a", mock.Anything, types.ServerSideEncryptionAes256).
		Return(nil)

	uploader.
		On("Upload", "a-bucket", "b", mock.Anything, types.ServerSideEncryptionAes256).
		Return(nil)

	response, err := l.HandleEvent(context.Background(), Event{})
	assert.Nil(err)
	assert.Equal(Response{Message: "clamav definitions updated"}, response)

	mock.AssertExpectationsForObjects(t, downloader, uploader, freshclam)
}
