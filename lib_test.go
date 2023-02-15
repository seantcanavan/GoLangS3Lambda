package lambda_s3

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/jgroeneveld/trial/assert"
	"github.com/joho/godotenv"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	BoundaryValue       = "---SEAN_BOUNDARY_VALUE"
	EmptyFileName       = "empty_file.txt"
	MaxFileSizeBytes    = 50000000 // 50 megabytes
	Region              = "us-east-2"
	S3Bucket            = "golang-s3-lambda-test"
	S3DeleteFileName    = "delete_me_dude"
	S3FileName          = "file_slash_key_name"
	SampleFileName      = "sample_file.csv"
	SampleFileSizeBytes = 369
)

func TestMain(m *testing.M) {
	setup()
	m.Run()
}

func setup() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Unable to load .env file: %s", err)
	}
}

func TestDelete(t *testing.T) {
	lambdaReq := generateUploadFileReq()

	fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(fileHeaders))

	uploadRes, err := UploadHeader(fileHeaders[0], Region, S3Bucket, S3DeleteFileName)
	assert.Nil(t, err)
	assert.Equal(t, filepath.Join(S3Bucket, S3DeleteFileName), uploadRes.S3Path)

	var urlBuilder strings.Builder
	urlBuilder.WriteString("https://")
	urlBuilder.WriteString(S3Bucket)
	urlBuilder.WriteString(".s3.")
	urlBuilder.WriteString(Region)
	urlBuilder.WriteString(".amazonaws.com/")
	urlBuilder.WriteString(S3DeleteFileName)
	assert.Equal(t, urlBuilder.String(), uploadRes.S3URL)

	deleteErr := Delete(Region, S3Bucket, S3DeleteFileName)
	assert.Nil(t, deleteErr)

	_, downloadErr := Download(Region, S3Bucket, S3DeleteFileName)
	assert.NotNil(t, downloadErr)
	assert.True(t, errors.Is(ErrDownloadingS3File, downloadErr))
}

func TestDownload(t *testing.T) {
	t.Run("verify err when region is empty", func(t *testing.T) {
		fileBytes, err := Download("", S3Bucket, S3FileName)
		assert.Equal(t, len(fileBytes), 0)
		assert.True(t, errors.Is(err, ErrParameterRegionEmpty))
	})
	t.Run("verify err when bucket is empty", func(t *testing.T) {
		fileBytes, err := Download(Region, "", S3FileName)
		assert.Equal(t, len(fileBytes), 0)
		assert.True(t, errors.Is(err, ErrParameterBucketEmpty))
	})
	t.Run("verify err when name is empty", func(t *testing.T) {
		fileBytes, err := Download(Region, S3Bucket, "")
		assert.Equal(t, len(fileBytes), 0)
		assert.True(t, errors.Is(err, ErrParameterNameEmpty))
	})
	t.Run("verify err when region is invalid", func(t *testing.T) {
		fileBytes, err := Download("us-east-sean", S3Bucket, S3FileName)
		assert.Equal(t, len(fileBytes), 0)
		assert.True(t, errors.Is(err, ErrDownloadingS3File))
	})
	t.Run("verify err when target file is empty", func(t *testing.T) {
		// first upload a totally empty file
		awsSession, err := session.NewSession(&aws.Config{
			Region: aws.String(Region)},
		)
		assert.Nil(t, err)

		uploader := s3manager.NewUploader(awsSession)

		emptyFile, err := os.Open(EmptyFileName)
		assert.Nil(t, err)

		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(S3Bucket),
			Key:    aws.String(EmptyFileName),
			Body:   emptyFile,
		})
		assert.Nil(t, err)

		fileBytes, err := Download(Region, S3Bucket, EmptyFileName)
		assert.Equal(t, len(fileBytes), 0)
		assert.True(t, errors.Is(err, ErrEmptyFileDownloaded))
	})
	t.Run("verify Download works with correct inputs", func(t *testing.T) {
		fileBytes, err := Download(Region, S3Bucket, S3FileName)
		assert.Equal(t, len(fileBytes), SampleFileSizeBytes)
		assert.Nil(t, err)
	})
}

func TestGetHeaders(t *testing.T) {
	t.Run("verify err when Content-Type header not set", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()
		lambdaReq.Headers = map[string]string{}

		fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
		assert.Equal(t, len(fileHeaders), 0)
		assert.True(t, errors.Is(err, ErrContentTypeHeaderMissing))
	})
	t.Run("verify err when content type is invalid", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()
		lambdaReq.Headers = map[string]string{"Content-Type": ";;;;;;;;;"}

		fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
		assert.Equal(t, len(fileHeaders), 0)
		assert.True(t, errors.Is(err, ErrParsingMediaType))
	})
	t.Run("verify err when content type has no boundary value", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()
		lambdaReq.Headers = map[string]string{"Content-Type": "blah"}

		fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
		assert.Equal(t, len(fileHeaders), 0)
		assert.True(t, errors.Is(err, ErrBoundaryValueMissing))
	})
	t.Run("verify GetHeaders works with correct inputs", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))
	})
}

func TestUploadHeader(t *testing.T) {
	t.Run("verify err when region is empty", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadHeader(fileHeaders[0], "", S3Bucket, S3FileName)
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrParameterRegionEmpty))
	})
	t.Run("verify err when bucket is empty", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadHeader(fileHeaders[0], Region, "", S3FileName)
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrParameterBucketEmpty))
	})
	t.Run("verify err when name is empty", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadHeader(fileHeaders[0], Region, S3Bucket, "")
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrParameterNameEmpty))
	})
	t.Run("verify err when *multipart.FileHeader is empty", func(t *testing.T) {
		uploadRes, err := UploadHeader(&multipart.FileHeader{}, Region, S3Bucket, S3FileName)
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrOpeningMultiPartFile))
	})
	t.Run("verify err when region is invalid and upload fails", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadHeader(fileHeaders[0], "us-east-sean", S3Bucket, S3FileName)
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrUploadingMultiPartFileToS3))
	})
	t.Run("verify UploadHeader works with correct inputs", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetHeaders(lambdaReq, MaxFileSizeBytes)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadHeader(fileHeaders[0], Region, S3Bucket, S3FileName)
		assert.Nil(t, err)
		assert.Equal(t, filepath.Join(S3Bucket, S3FileName), uploadRes.S3Path)

		var urlBuilder strings.Builder
		urlBuilder.WriteString("https://")
		urlBuilder.WriteString(S3Bucket)
		urlBuilder.WriteString(".s3.")
		urlBuilder.WriteString(Region)
		urlBuilder.WriteString(".amazonaws.com/")
		urlBuilder.WriteString(S3FileName)
		assert.Equal(t, urlBuilder.String(), uploadRes.S3URL)
	})
}

func generateUploadFileReq() events.APIGatewayProxyRequest {
	fileBytes, readErr := os.ReadFile(SampleFileName)
	if readErr != nil {
		fmt.Println(fmt.Sprintf("readErr [%+v]", readErr))
		log.Panic("should be able to read bytes from " + SampleFileName)
	}

	if len(fileBytes) <= 0 {
		log.Panic("should get non zero amount of bytes to read from " + SampleFileName)
	}

	if len(fileBytes) != SampleFileSizeBytes {
		log.Panicf("should get exactly [%d] bytes from [%s]", SampleFileSizeBytes, SampleFileName)
	}

	var multiPartBuffer bytes.Buffer
	writer := multipart.NewWriter(&multiPartBuffer)
	boundaryErr := writer.SetBoundary(BoundaryValue)
	if boundaryErr != nil {
		fmt.Println(fmt.Sprintf("boundaryErr [%+v]", boundaryErr))
		log.Panicf("should not error on setting boundary value to [%s]", BoundaryValue)
	}

	part, createFormErr := writer.CreateFormFile("sample_file", SampleFileName)
	if createFormErr != nil {
		fmt.Println(fmt.Sprintf("createFormErr [%+v]", createFormErr))
		log.Panic("should not error on creating form file")
	}

	bytesWritten, writeErr := part.Write(fileBytes)
	if writeErr != nil {
		fmt.Println(fmt.Sprintf("writeErr [%+v]", writeErr))
		log.Panic("should not error on writing file bytes to form")
	}

	if bytesWritten != SampleFileSizeBytes {
		log.Panicf("should write exactly [%d] bytes to the form", SampleFileSizeBytes)
	}

	closeErr := writer.Close()
	if closeErr != nil {
		fmt.Println(fmt.Sprintf("closeErr [%+v]", closeErr))
	}

	contentType := writer.FormDataContentType()

	return events.APIGatewayProxyRequest{
		Headers:         map[string]string{"Content-Type": contentType},
		Body:            base64.StdEncoding.EncodeToString(multiPartBuffer.Bytes()),
		IsBase64Encoded: true,
	}
}
