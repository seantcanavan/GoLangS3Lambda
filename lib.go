package lambda_s3

import (
	"errors"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strings"
)

var ErrContentTypeHeaderMissing = errors.New("request contained no Content-Type header")
var ErrParsingMediaType = errors.New("error parsing media type from Content-Type header. Make sure your request is formatted correctly")
var ErrBoundaryValueMissing = errors.New("request contained no boundary value in the Content-Type header")
var ErrReadingMultiPartForm = errors.New("reading of multipart form failed. verify input size is <= maxFileSizeBytes")

// GetFileHeadersFromLambdaReq accepts a lambda request directly from AWS Lambda after it has been proxied through
// API Gateway. It returns an array of *multipart.FileHeader values. One for each file uploaded to Lambda.
func GetFileHeadersFromLambdaReq(lambdaReq events.APIGatewayProxyRequest, maxFileSizeBytes int64) ([]*multipart.FileHeader, error) {
	//parse the lambda body
	contentType := lambdaReq.Headers["Content-Type"]
	if contentType == "" {
		return nil, ErrContentTypeHeaderMissing
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, ErrParsingMediaType
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, ErrBoundaryValueMissing
	}

	stringReader := strings.NewReader(lambdaReq.Body)
	multipartReader := multipart.NewReader(stringReader, boundary)

	form, err := multipartReader.ReadForm(maxFileSizeBytes)
	if err != nil {
		return nil, ErrReadingMultiPartForm
	}

	var files []*multipart.FileHeader

	for currentFileName := range form.File {
		files = append(files, form.File[currentFileName][0])
	}

	return files, nil
}

type UploadRes struct {
	S3Path string
	S3URL  string
}

var ErrParameterRegionEmpty = errors.New("required parameter region is empty")
var ErrParameterBucketEmpty = errors.New("required parameter bucket is empty")
var ErrParameterNameEmpty = errors.New("required parameter name is empty")
var ErrOpeningMultiPartFile = errors.New("unable to open *multipart.FileHeader")
var ErrReadingMultiPartFile = errors.New("unable to read *multipart.FileHeader")
var ErrNewAWSSession = errors.New("error creating new AWS Session")
var ErrUploadingMultiPartFileToS3 = errors.New("unable to upload *multipart.FileHeader bytes to S3")

// UploadFileHeaderToS3 takes a single *multipart.FileHeader from the Lambda request and uploads it to S3.
// It the upload is successful it returns the full path to the file in S3 as well as the URL for web access.
func UploadFileHeaderToS3(fileHeader *multipart.FileHeader, region, bucket, name string) (*UploadRes, error) {
	if region == "" {
		return nil, ErrParameterRegionEmpty
	}

	if bucket == "" {
		return nil, ErrParameterBucketEmpty
	}

	if name == "" {
		return nil, ErrParameterNameEmpty
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, ErrOpeningMultiPartFile
	}

	var fileContents []byte
	_, err = file.Read(fileContents)
	if err != nil {
		return nil, ErrReadingMultiPartFile
	}

	// https://stackoverflow.com/q/47621804/584947
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		return nil, ErrNewAWSSession
	}

	uploader := s3manager.NewUploader(awsSession)

	uploadOutput, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(name),
		Body:   file,
	})
	if err != nil {
		return nil, ErrUploadingMultiPartFileToS3
	}

	return &UploadRes{
		S3Path: filepath.Join(bucket, name),
		S3URL:  uploadOutput.Location,
	}, nil
}

var ErrDownloadingS3File = errors.New("unable to download the given file from S3")
var ErrEmptyFileDownloaded = errors.New("the provided S3 file to download is empty")

func DownloadFileFromS3(region, bucket, name string) ([]byte, error) {
	if region == "" {
		return nil, ErrParameterRegionEmpty
	}

	if bucket == "" {
		return nil, ErrParameterBucketEmpty
	}

	if name == "" {
		return nil, ErrParameterNameEmpty
	}

	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		return nil, ErrNewAWSSession
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
		return nil, ErrDownloadingS3File
	}

	if bytesDownloaded == 0 {
		return nil, ErrEmptyFileDownloaded
	}

	return writeAtBuffer.Bytes(), nil
}
