package golang_s3_lambda

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/jgroeneveld/trial/assert"
	"github.com/joho/godotenv"
	"log"
	"math/big"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

const Region = "us-east-2"
const Name = "file_slash_key_name"
const Bucket = "golang-s3-lambda-test"
const BoundaryValue = "---SEAN_BOUNDARY_VALUE"
const SampleFile = "sample_file.csv"
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
	lambdaReq := generateRandomAPIGatewayProxyRequest()

	fileHeaders, httpStatus, err := GetFileHeadersFromLambdaReq(lambdaReq)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, httpStatus)
	assert.Equal(t, 1, len(fileHeaders))
}

func TestDownloadFileFromS3(t *testing.T) {

}

func TestUploadFileHeaderToS3(t *testing.T) {
	lambdaReq := generateRandomAPIGatewayProxyRequest()

	fileHeaders, httpStatus, err := GetFileHeadersFromLambdaReq(lambdaReq)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, httpStatus)
	assert.Equal(t, 1, len(fileHeaders))

	s3Path, httpStatus, err := UploadFileHeaderToS3(fileHeaders[0], Region, Bucket, Name)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, httpStatus)
	assert.Equal(t, filepath.Join(Bucket, Name), s3Path)
}

func generateRandomAPIGatewayProxyRequest() events.APIGatewayProxyRequest {
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
		Resource:                        generateRandomString(10),
		Path:                            generateRandomString(10),
		HTTPMethod:                      generateRandomString(10),
		Headers:                         map[string]string{"Content-Type": contentType},
		MultiValueHeaders:               map[string][]string{"multiValueHeaders": {"hello there"}},
		QueryStringParameters:           map[string]string{"queryStringParameters": "value"},
		MultiValueQueryStringParameters: map[string][]string{"multiValueQueryStringParameters": {"hello there"}},
		PathParameters:                  map[string]string{"pathParameters": "value"},
		StageVariables:                  map[string]string{"stageVariables": "value"},
		RequestContext: events.APIGatewayProxyRequestContext{
			AccountID:     generateRandomString(10),
			ResourceID:    generateRandomString(10),
			OperationName: generateRandomString(10),
			Stage:         generateRandomString(10),
			DomainName:    generateRandomString(10),
			DomainPrefix:  generateRandomString(10),
			RequestID:     generateRandomString(10),
			Protocol:      generateRandomString(10),
			Identity: events.APIGatewayRequestIdentity{
				CognitoIdentityPoolID:         generateRandomString(10),
				AccountID:                     generateRandomString(10),
				CognitoIdentityID:             generateRandomString(10),
				Caller:                        generateRandomString(10),
				APIKey:                        generateRandomString(10),
				APIKeyID:                      generateRandomString(10),
				AccessKey:                     generateRandomString(10),
				SourceIP:                      generateRandomString(10),
				CognitoAuthenticationType:     generateRandomString(10),
				CognitoAuthenticationProvider: generateRandomString(10),
				UserArn:                       generateRandomString(10),
				UserAgent:                     generateRandomString(10),
				User:                          generateRandomString(10),
			},
			ResourcePath:     generateRandomString(10),
			Path:             generateRandomString(10),
			Authorizer:       map[string]interface{}{"hi there": "sean"},
			HTTPMethod:       generateRandomString(10),
			RequestTime:      generateRandomString(10),
			RequestTimeEpoch: 0,
			APIID:            generateRandomString(10),
		},
		Body:            string(multiPartBytes.Bytes()),
		IsBase64Encoded: false,
	}
}

func generateRandomString(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	ret := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return ""
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret)
}
