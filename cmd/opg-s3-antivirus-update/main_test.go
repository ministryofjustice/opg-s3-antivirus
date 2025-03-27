package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDownloader struct {
	*s3manager.Downloader
	mock.Mock
}

func (m *mockDownloader) Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (int64, error) {
	args := m.Called(w, *input.Bucket, *input.Key)
	return 0, args.Error(0)
}

type mockUploader struct {
	*s3manager.Uploader
	mock.Mock
}

func (m *mockUploader) Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	args := m.Called(*input.Bucket, *input.Key, input.Body, *input.ServerSideEncryption)
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
		On("Upload", "a-bucket", "a", mock.Anything, "AES256").
		Run(func(args mock.Arguments) {
			r := args[2].(io.Reader)
			data, _ := io.ReadAll(r)
			assert.Equal([]byte("hello"), data)
		}).
		Return(nil)

	uploader.
		On("Upload", "a-bucket", "b", mock.Anything, "AES256").
		Run(func(args mock.Arguments) {
			r := args[2].(io.Reader)
			data, _ := io.ReadAll(r)
			assert.Equal([]byte("there"), data)
		}).
		Return(nil)

	response, err := l.HandleEvent(Event{})
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
		Return(awserr.New(s3.ErrCodeNoSuchKey, "", errors.New("")))

	freshclam.
		On("Update").Return(nil)

	uploader.
		On("Upload", "a-bucket", "a", mock.Anything, "AES256").
		Return(nil)

	uploader.
		On("Upload", "a-bucket", "b", mock.Anything, "AES256").
		Return(nil)

	response, err := l.HandleEvent(Event{})
	assert.Nil(err)
	assert.Equal(Response{Message: "clamav definitions updated"}, response)

	mock.AssertExpectationsForObjects(t, downloader, uploader, freshclam)
}
