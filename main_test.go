package main

import (
	genericErrors "errors"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type TestMinioClient struct {
	data map[string]string
	name string
}

func (c TestMinioClient) GetName() string {
	return c.name
}

func (c TestMinioClient) GetObject(objectId string) (io.Reader, error) {
	reader := strings.NewReader("")
	if data, ok := c.data[objectId]; ok {
		return strings.NewReader(data), nil
	}
	return reader, genericErrors.New("no content")
}
func (c TestMinioClient) PutObject(objectId string, reader io.Reader, contentLength int64) (int64, error) {
	buf := new(strings.Builder)
	n, err := io.Copy(buf, reader)
	// check errors
	c.data[objectId] = buf.String()
	return n, err
}
func (c TestMinioClient) EnsureBucketExists() error {
	return nil
}

type homeworkTest struct {
	title           string
	request         *http.Request
	variables       map[string]string
	expectedCode    int
	expectedContent string
}

func createRequest(method string, url string, body string) *http.Request {
	request, _ := http.NewRequest(method, url, strings.NewReader(body))
	return request
}

func TestBasicRequests(t *testing.T) {
	minioClients = []MinioClient{TestMinioClient{
		data: make(map[string]string),
		name: "client1",
	}}

	tests := []homeworkTest{
		{
			title:           "Get missing file",
			request:         createRequest("GET", "http://none/object/2", ""),
			variables:       map[string]string{"id": "2"},
			expectedCode:    404,
			expectedContent: "",
		},
		{
			title:           "Put new file",
			request:         createRequest("PUT", "http://none/object/3", "TESTING"),
			variables:       map[string]string{"id": "3"},
			expectedCode:    200,
			expectedContent: "",
		},
		{
			title:           "Get new file",
			request:         createRequest("GET", "http://none/object/3", ""),
			variables:       map[string]string{"id": "3"},
			expectedCode:    200,
			expectedContent: "TESTING",
		},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			testSingleRequest(t, test)
		})
	}
}

func testSingleRequest(t *testing.T, test homeworkTest) {
	request := test.request
	vars := test.variables
	expectedCode := test.expectedCode
	expectedContent := test.expectedContent
	request = mux.SetURLVars(request, vars)
	rw := httptest.NewRecorder()

	if strings.Compare(request.Method, "GET") == 0 {
		GetHandler(rw, request)
	} else if strings.Compare(request.Method, "PUT") == 0 {
		PutHandler(rw, request)
	} else {
		t.Error("Unexpected HTTP verb")
	}

	if rw.Code != expectedCode {
		t.Error("Expected code", expectedCode, ", got ", rw.Code)
	}
	content := rw.Body.String()
	if len(expectedContent) > 0 && strings.Compare(content, expectedContent) != 0 {
		t.Error("Expected to get content ", expectedContent, ", got ", content)
	}
}

func TestMultipleClients(t *testing.T) {
	testClient1 := TestMinioClient{
		data: make(map[string]string),
		name: "client1",
	}
	testClient2 := TestMinioClient{
		data: make(map[string]string),
		name: "client2",
	}
	minioClients = []MinioClient{
		testClient1,
		testClient2,
	}

	tests := []homeworkTest{
		{
			title:           "Put file 1",
			request:         createRequest("PUT", "http://none/object/1", "content1"),
			variables:       map[string]string{"id": "1"},
			expectedCode:    200,
			expectedContent: "",
		},
		{
			title:           "Put file 2",
			request:         createRequest("PUT", "http://none/object/2", "content2"),
			variables:       map[string]string{"id": "2"},
			expectedCode:    200,
			expectedContent: "",
		},
		{
			title:           "Put file 3",
			request:         createRequest("PUT", "http://none/object/3", "content3"),
			variables:       map[string]string{"id": "3"},
			expectedCode:    200,
			expectedContent: "",
		},
		{
			title:           "Put file 4",
			request:         createRequest("PUT", "http://none/object/4", "content4"),
			variables:       map[string]string{"id": "4"},
			expectedCode:    200,
			expectedContent: "",
		},
		{
			title:           "Get file 1",
			request:         createRequest("GET", "http://none/object/1", ""),
			variables:       map[string]string{"id": "1"},
			expectedCode:    200,
			expectedContent: "content1",
		},
		{
			title:           "Get file 2",
			request:         createRequest("GET", "http://none/object/2", ""),
			variables:       map[string]string{"id": "2"},
			expectedCode:    200,
			expectedContent: "content2",
		},
		{
			title:           "Get file 3",
			request:         createRequest("GET", "http://none/object/3", ""),
			variables:       map[string]string{"id": "3"},
			expectedCode:    200,
			expectedContent: "content3",
		},
		{
			title:           "Get file 4",
			request:         createRequest("GET", "http://none/object/4", ""),
			variables:       map[string]string{"id": "4"},
			expectedCode:    200,
			expectedContent: "content4",
		},
	}

	for _, test := range tests {
		testSingleRequest(t, test)
	}

	size1 := len(testClient1.data)
	size2 := len(testClient2.data)
	log.Info("Data sizes after 4 puts: ", size1, " and ", size2)
	if size1+size2 != 4 {
		t.Error("Sum of sizes after 4 puts isn't 4")
	}
	if size1 == 4 || size2 == 0 {
		t.Error("Hash function doesn't distribute data to multiple hosts")
	}
}
