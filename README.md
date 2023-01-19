# Lambda S3
Upload or Download files from AWS S3 with one function through AWS Lambda and AWS APIGateway.

## How to Build locally
1. `make build`

## How to Test locally
1. `cp .env.example .env` copy the example `.env` file to use as a base
2. `nano .env` insert real values for each environment variable
3. `make test`

## How to Use
1. `go get github.com/seantcanavan/lambda_s3@latest`
2. `import github.com/seantcanavan/lambda_s3`
3. Get the file headers uploaded via AWS Lambda / API Gateway: `headers, err := lambda_s3.GetFileHeadersFromLambdaReq()`
4. Upload the uploaded file contents via the file headers: `lambda_s3.UploadFileHeaderToS3()`
5. Download the file contents via the file's bucket/key combination: `lambda_s3.DownloadFileFromS3()`
6. Use `errors.Is` to check for different error cases returned from `GetFileHeadersFromLambdaReq`, `UploadFileHeaderToS3`, and `DownloadFileFromS3`
   1. Check below for sample code on how to implement the functions and use `errors.Is`

## Sample Upload Lambda Handler Example
``` go
func UploadLambda(ctx context.Context, lambdaReq events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// get the file header and multipart form data from Lambda
	files, err := lambda_s3.GetFileHeadersFromLambdaReq(lambdaReq, MaxFileSizeBytes)
	if err != nil {
		switch {
		case
			errors.Is(err, lambda_s3.ErrContentTypeHeaderMissing),
			errors.Is(err, lambda_s3.ErrParsingMediaType),
			errors.Is(err, lambda_s3.ErrBoundaryValueMissing):
			return ERROR - bad request
		default:
			return ERROR - internal server
		}
	}

	fileName := uuid.New().String() // generate a UUID for filename to guarantee uniqueness

	// take the first file uploaded via HTTP and upload it to S3
	// you can also loop through all the files in files to upload each individually:
	// for _, currentFile := range files { Upload... }
	uploadResult, err := lambda_s3.UploadFileHeaderToS3(files[0], os.Getenv("REGION_AWS"), os.Getenv("FILE_BUCKET"), fileName)
	if err != nil {
		switch {
		case
			errors.Is(err, lambda_s3.ErrParameterRegionEmpty),
			errors.Is(err, lambda_s3.ErrParameterBucketEmpty),
			errors.Is(err, lambda_s3.ErrParameterNameEmpty):
			return ERROR - bad request
		default:
			return ERROR - internal server
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:            "",
	}, nil
}
```

## Sample Download Lambda Handler Example
``` go
func DownloadLambda(ctx context.Context, lambdaReq events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	fileID := lambdaReq.PathParameters["id"]
	if fileID == "" {
		return ERROR - bad request
	}

	file, err := GetFileByID(ctx, fileID) // get the file object from your database to retrieve its unique name
	if err != nil {
		return ERROR - not found
	}

	fileBytes, err := lambda_s3.DownloadFileFromS3(os.Getenv("REGION_AWS"), os.Getenv("FILE_BUCKET"), file.Name)
	if err != nil {
		switch {
		case
			errors.Is(err, lambda_s3.ErrParameterRegionEmpty),
			errors.Is(err, lambda_s3.ErrParameterBucketEmpty),
			errors.Is(err, lambda_s3.ErrParameterNameEmpty),
			errors.Is(err, lambda_s3.ErrEmptyFileDownloaded):
			return lmdrouter.HandleHTTPError(http.StatusBadRequest, err)
		default:
			return lmdrouter.HandleHTTPError(http.StatusInternalServerError, err)
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": file.ContentType,
		},
		Body:            base64.StdEncoding.EncodeToString(fileBytes),
		IsBase64Encoded: true,
	}, nil
}
```

## All tests are passing
```
Thu Jan 19 03:51 PM lambda_s3: make test
go test -v
=== RUN   TestGetFileHeadersFromLambdaReq
--- PASS: TestGetFileHeadersFromLambdaReq (0.00s)
=== RUN   TestDownloadFileFromS3
=== RUN   TestDownloadFileFromS3/verify_err_when_region_is_empty
=== RUN   TestDownloadFileFromS3/verify_err_when_bucket_is_empty
=== RUN   TestDownloadFileFromS3/verify_err_when_name_is_empty
=== RUN   TestDownloadFileFromS3/verify_err_when_region_is_invalid
=== RUN   TestDownloadFileFromS3/verify_err_when_target_file_is_empty
=== RUN   TestDownloadFileFromS3/verify_download_works_with_correct_inputs
--- PASS: TestDownloadFileFromS3 (0.86s)
    --- PASS: TestDownloadFileFromS3/verify_err_when_region_is_empty (0.00s)
    --- PASS: TestDownloadFileFromS3/verify_err_when_bucket_is_empty (0.00s)
    --- PASS: TestDownloadFileFromS3/verify_err_when_name_is_empty (0.00s)
    --- PASS: TestDownloadFileFromS3/verify_err_when_region_is_invalid (0.49s)
    --- PASS: TestDownloadFileFromS3/verify_err_when_target_file_is_empty (0.30s)
    --- PASS: TestDownloadFileFromS3/verify_download_works_with_correct_inputs (0.06s)
=== RUN   TestUploadFileHeaderToS3
=== RUN   TestUploadFileHeaderToS3/verify_err_when_region_is_empty
=== RUN   TestUploadFileHeaderToS3/verify_err_when_bucket_is_empty
=== RUN   TestUploadFileHeaderToS3/verify_err_when_name_is_empty
=== RUN   TestUploadFileHeaderToS3/verify_err_when_*multipart.FileHeader_is_empty
=== RUN   TestUploadFileHeaderToS3/verify_err_when_region_is_invalid_and_upload_fails
=== RUN   TestUploadFileHeaderToS3/verify_upload_works_with_correct_inputs
--- PASS: TestUploadFileHeaderToS3 (0.61s)
    --- PASS: TestUploadFileHeaderToS3/verify_err_when_region_is_empty (0.00s)
    --- PASS: TestUploadFileHeaderToS3/verify_err_when_bucket_is_empty (0.00s)
    --- PASS: TestUploadFileHeaderToS3/verify_err_when_name_is_empty (0.00s)
    --- PASS: TestUploadFileHeaderToS3/verify_err_when_*multipart.FileHeader_is_empty (0.00s)
    --- PASS: TestUploadFileHeaderToS3/verify_err_when_region_is_invalid_and_upload_fails (0.55s)
    --- PASS: TestUploadFileHeaderToS3/verify_upload_works_with_correct_inputs (0.06s)
PASS
ok  	github.com/seantcanavan/lambda_s3	1.470s
```
