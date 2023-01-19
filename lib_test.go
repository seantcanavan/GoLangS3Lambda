package golang_s3_lambda

import (
	"bytes"
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

const Region = "us-east-2"
const Name = "file_slash_key_name"
const Bucket = "golang-s3-lambda-test"
const BoundaryValue = "---SEAN_BOUNDARY_VALUE"
const SampleFile = "sample_file.csv"
const EmptyFile = "empty_file.txt"
const SampleFileBytes = 369

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

func TestGetFileHeadersFromLambdaReq(t *testing.T) {
	lambdaReq := generateUploadFileReq()

	fileHeaders, err := GetFileHeadersFromLambdaReq(lambdaReq)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(fileHeaders))
}

func TestDownloadFileFromS3(t *testing.T) {
	t.Run("verify err when region is empty", func(t *testing.T) {
		fileBytes, err := DownloadFileFromS3("", Bucket, Name)
		assert.Equal(t, len(fileBytes), 0)
		assert.True(t, errors.Is(err, ErrParameterRegionEmpty))
	})
	t.Run("verify err when bucket is empty", func(t *testing.T) {
		fileBytes, err := DownloadFileFromS3(Region, "", Name)
		assert.Equal(t, len(fileBytes), 0)
		assert.True(t, errors.Is(err, ErrParameterBucketEmpty))
	})
	t.Run("verify err when name is empty", func(t *testing.T) {
		fileBytes, err := DownloadFileFromS3(Region, Bucket, "")
		assert.Equal(t, len(fileBytes), 0)
		assert.True(t, errors.Is(err, ErrParameterNameEmpty))
	})
	t.Run("verify err when region is invalid", func(t *testing.T) {
		fileBytes, err := DownloadFileFromS3("us-east-sean", Bucket, Name)
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

		emptyFile, err := os.Open(EmptyFile)
		assert.Nil(t, err)

		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(Bucket),
			Key:    aws.String(EmptyFile),
			Body:   emptyFile,
		})
		assert.Nil(t, err)

		fileBytes, err := DownloadFileFromS3(Region, Bucket, EmptyFile)
		assert.Equal(t, len(fileBytes), 0)
		assert.True(t, errors.Is(err, ErrEmptyFileDownloaded))
	})
	t.Run("verify download works with correct inputs", func(t *testing.T) {
		fileBytes, err := DownloadFileFromS3(Region, Bucket, Name)
		assert.Equal(t, len(fileBytes), SampleFileBytes)
		assert.Nil(t, err)
	})
}

func TestUploadFileHeaderToS3(t *testing.T) {
	t.Run("verify err when region is empty", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetFileHeadersFromLambdaReq(lambdaReq)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadFileHeaderToS3(fileHeaders[0], "", Bucket, Name)
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrParameterRegionEmpty))
	})
	t.Run("verify err when bucket is empty", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetFileHeadersFromLambdaReq(lambdaReq)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadFileHeaderToS3(fileHeaders[0], Region, "", Name)
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrParameterBucketEmpty))
	})
	t.Run("verify err when name is empty", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetFileHeadersFromLambdaReq(lambdaReq)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadFileHeaderToS3(fileHeaders[0], Region, Bucket, "")
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrParameterNameEmpty))
	})
	t.Run("verify err when *multipart.FileHeader is empty", func(t *testing.T) {
		uploadRes, err := UploadFileHeaderToS3(&multipart.FileHeader{}, Region, Bucket, Name)
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrOpeningMultiPartFile))
	})
	t.Run("verify err when region is invalid and upload fails", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetFileHeadersFromLambdaReq(lambdaReq)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadFileHeaderToS3(fileHeaders[0], "us-east-sean", Bucket, Name)
		assert.Equal(t, uploadRes, (*UploadRes)(nil))
		assert.True(t, errors.Is(err, ErrUploadingMultiPartFileToS3))
	})
	t.Run("verify upload works with correct inputs", func(t *testing.T) {
		lambdaReq := generateUploadFileReq()

		fileHeaders, err := GetFileHeadersFromLambdaReq(lambdaReq)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(fileHeaders))

		uploadRes, err := UploadFileHeaderToS3(fileHeaders[0], Region, Bucket, Name)
		assert.Nil(t, err)
		assert.Equal(t, filepath.Join(Bucket, Name), uploadRes.S3Path)

		var urlBuilder strings.Builder
		urlBuilder.WriteString("https://")
		urlBuilder.WriteString(Bucket)
		urlBuilder.WriteString(".s3.")
		urlBuilder.WriteString(Region)
		urlBuilder.WriteString(".amazonaws.com/")
		urlBuilder.WriteString(Name)
		assert.Equal(t, urlBuilder.String(), uploadRes.S3URL)
	})
}

func generateUploadFileReq() events.APIGatewayProxyRequest {
	fileBytes, readErr := os.ReadFile(SampleFile)
	if readErr != nil {
		fmt.Println(fmt.Sprintf("readErr [%+v]", readErr))
		log.Panic("should be able to read bytes from " + SampleFile)
	}

	if len(fileBytes) <= 0 {
		log.Panic("should get non zero amount of bytes to read from " + SampleFile)
	}

	if len(fileBytes) != SampleFileBytes {
		log.Panicf("should get exactly [%d] bytes from [%s]", SampleFileBytes, SampleFile)
	}

	var multiPartBytes bytes.Buffer
	writer := multipart.NewWriter(&multiPartBytes)
	boundaryErr := writer.SetBoundary(BoundaryValue)
	if boundaryErr != nil {
		fmt.Println(fmt.Sprintf("boundaryErr [%+v]", boundaryErr))
		log.Panicf("should not error on setting boundary value to [%s]", BoundaryValue)
	}

	part, createFormErr := writer.CreateFormFile("sample_file", SampleFile)
	if createFormErr != nil {
		fmt.Println(fmt.Sprintf("createFormErr [%+v]", createFormErr))
		log.Panic("should not error on creating form file")
	}

	bytesWritten, writeErr := part.Write(fileBytes)
	if writeErr != nil {
		fmt.Println(fmt.Sprintf("writeErr [%+v]", writeErr))
		log.Panic("should not error on writing file bytes to form")
	}

	if bytesWritten != SampleFileBytes {
		log.Panicf("should write exactly [%d] bytes to the form", SampleFileBytes)
	}

	closeErr := writer.Close()
	if closeErr != nil {
		fmt.Println(fmt.Sprintf("closeErr [%+v]", closeErr))
	}

	contentType := writer.FormDataContentType()

	return events.APIGatewayProxyRequest{
		Headers:         map[string]string{"Content-Type": contentType},
		Body:            string(multiPartBytes.Bytes()),
		IsBase64Encoded: false,
	}
}
