# GoLangS3Lambda
Upload or Download files from AWS S3 with one function through AWS Lambda and AWS APIGateway.

## How to Build locally
1. `make build`

## How to Test locally
1. `cp .env.example .env` copy the example `.env` file to use as a base
2. `nano .env` insert real values for each environment variable
3. `make test`

## All tests are passing
```
```

## How to Use
1. `go get github.com/seantcanavan/lambda_s3@latest`
2. `import github.com/seantcanavan/lambda_s3`
3. Get the file headers uploaded via AWS Lambda / API Gateway: `headers, err := lambda_s3.GetFileHeadersFromLambdaReq()`
   1. Use `errors.Is` to check for different error cases returned from `GetFileHeadersFromLambdaReq()`
4. Upload the file uploaded file contents via the file headers: `lambda_s3.UploadFileHeaderToS3()`
   1. Use `errors.Is` to check for different error cases returned from `UploadFileHeaderToS3()`
5. Download the file contents via the file's bucket/key combination: `lambda_s3.DownloadFileFromS3()`
   1. Use `errors.Is` to check for different error cases returned from `DownloadFileFromS3()`


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
