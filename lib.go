// Package lambda_s3 provides simple utility files to upload or download files from AWS S3
// through AWS Lambda using APIGateway Proxy requests from the AWS Go SDK. First, file headers
// are parsed from the lambda request which is where file uploads are stored from HTTP requests.
// Then, using those file headers, the bytes can be extracted and uploaded to S3 with a given file name.
// Finally, knowing the name of a file and the bucket it's contained in, said file(s) can also be downloaded.
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
	"net/http"
	"path/filepath"
	"strings"
)

var (
	ErrBoundaryValueMissing       = errors.New("request contained no boundary value in the Content-Type header")
	ErrContentTypeHeaderMissing   = errors.New("request contained no Content-Type header")
	ErrDownloadingS3File          = errors.New("unable to download the given file from S3")
	ErrEmptyFileDownloaded        = errors.New("the provided S3 file to download is empty")
	ErrNewAWSSession              = errors.New("error creating new AWS Session")
	ErrOpeningMultiPartFile       = errors.New("unable to open *multipart.FileHeader")
	ErrParameterBucketEmpty       = errors.New("required parameter bucket is empty")
	ErrParameterNameEmpty         = errors.New("required parameter name is empty")
	ErrParameterRegionEmpty       = errors.New("required parameter region is empty")
	ErrParsingMediaType           = errors.New("error parsing media type from Content-Type header. Make sure your request is formatted correctly")
	ErrReadingMultiPartFile       = errors.New("unable to read *multipart.FileHeader")
	ErrReadingMultiPartForm       = errors.New("reading of multipart form failed. verify input size is <= maxFileSizeBytes")
	ErrUploadingMultiPartFileToS3 = errors.New("unable to upload *multipart.FileHeader bytes to S3")
)

func Delete(region, bucket, name string) error {
	if region == "" {
		return ErrParameterRegionEmpty
	}

	if bucket == "" {
		return ErrParameterBucketEmpty
	}

	if name == "" {
		return ErrParameterNameEmpty
	}

	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		return ErrNewAWSSession
	}

	batcher := s3manager.NewBatchDelete(awsSession, func(batchDelete *s3manager.BatchDelete) {
		batchDelete.BatchSize = 1
	})

	objects := []s3manager.BatchDeleteObject{
		{
			Object: &s3.DeleteObjectInput{
				Key:    aws.String(name),
				Bucket: aws.String(bucket),
			},
		},
	}

	return batcher.Delete(aws.BackgroundContext(), &s3manager.DeleteObjectsIterator{Objects: objects})
}

// Download accepts an AWS Region, the name of an S3 bucket, and the key or name of a file to download.
// It will create a new AWS Session in the specified region and proceed to try to download the file.
// All three parameters, region, bucket, and name are required.
// If the download is successful, it will return a byte array containing the bytes for the file.
func Download(region, bucket, name string) ([]byte, error) {
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

// GetHeaders accepts a lambda request directly from AWS Lambda after it has been proxied through
// API Gateway. It returns an array of *multipart.FileHeader values. One for each file uploaded to Lambda.
func GetHeaders(lambdaReq events.APIGatewayProxyRequest, maxFileSizeBytes int64) ([]*multipart.FileHeader, error) {
	//parse the lambda body
	// workaround for case-sensitive headers. thanks AWS!
	// https://github.com/aws/aws-lambda-go/issues/117
	headers := http.Header{}

	for header, values := range lambdaReq.Headers {
		headers.Add(header, values)
	}

	contentType := headers.Get("Content-Type")
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

// UploadHeader takes a single *multipart.FileHeader from the Lambda request and uploads it to S3.
// It the upload is successful it returns the full path to the file in S3 as well as the URL for web access in UploadRes.
func UploadHeader(fileHeader *multipart.FileHeader, region, bucket, name string) (*UploadRes, error) {
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
