package golang_s3_lambda

import (
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const DefaultMaxFileSizeBytes = 50000000 // 50 megabytes

func GetFileHeadersFromLambdaReq(lambdaReq events.APIGatewayProxyRequest) ([]*multipart.FileHeader, int, error) {
	//parse the lambda body
	contentType := lambdaReq.Headers["Content-Type"]
	if contentType == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("request contained no Content-Type header")
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("request contained no boundary value to parse from Content-Type headers")
	}

	stringReader := strings.NewReader(lambdaReq.Body)
	multipartReader := multipart.NewReader(stringReader, boundary)

	var maxFileSizeBytes int64
	maxFileSizeBytesStr := os.Getenv("MAX_FILE_SIZE_BYTES")
	if maxFileSizeBytesStr == "" {
		maxFileSizeBytes = DefaultMaxFileSizeBytes
	} else {
		maxFileSizeBytes, err = strconv.ParseInt(maxFileSizeBytesStr, 10, 64)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
	}

	form, err := multipartReader.ReadForm(maxFileSizeBytes)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	var files []*multipart.FileHeader

	for currentFileName := range form.File {
		files = append(files, form.File[currentFileName][0])
	}

	return files, http.StatusOK, nil
}

func UploadFileHeaderToS3(fileHeader *multipart.FileHeader, region, bucket, name string) (string, int, error) {
	if region == "" {
		return "", http.StatusBadRequest, fmt.Errorf("cannot upload to S3 with missing required parameter region [%s]", region)
	}

	if bucket == "" {
		return "", http.StatusBadRequest, fmt.Errorf("cannot upload to S3 with missing required parameter bucket [%s]", bucket)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", http.StatusInternalServerError, err
	}

	var fileContents []byte
	_, err = file.Read(fileContents)
	if err != nil {
		return "", http.StatusInternalServerError, err
	}

	// https://stackoverflow.com/q/47621804/584947
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)

	uploader := s3manager.NewUploader(awsSession)

	uploadOutput, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(name),
		Body:   file,
	})
	if err != nil {
		return "", http.StatusInternalServerError, err
	}

	return uploadOutput.Location, http.StatusOK, nil
}

func DownloadFileFromS3(region, bucket, name string) ([]byte, int, error) {
	if region == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("cannot download from S3 with missing required parameter region [%s]", region)
	}

	if bucket == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("cannot download from S3 with missing required parameter bucket [%s]", bucket)
	}

	if name == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("cannot download fron S3 with missing required parameter name [%s]", name)
	}

	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	downloader := s3manager.NewDownloader(awsSession)

	var fileBytes []byte
	writeAtBuffer := aws.NewWriteAtBuffer(fileBytes)

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(name),
	}

	// functional options pattern
	bytesDownloaded, err := downloader.Download(writeAtBuffer, getObjectInput, func(downloader *s3manager.Downloader) {
		downloader.Concurrency = 0
	})

	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	if bytesDownloaded == 0 {
		return nil, http.StatusInternalServerError, fmt.Errorf("downloaded [%d] bytes. Expected non-zero", bytesDownloaded)
	}

	return writeAtBuffer.Bytes(), http.StatusOK, nil
}
