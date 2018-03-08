/******************************************************************************
*
*  Copyright 2018 Stefan Majewsky <majewsky@gmx.net>
*
*  Licensed under the Apache License, Version 2.0 (the "License");
*  you may not use this file except in compliance with the License.
*  You may obtain a copy of the License at
*
*      http://www.apache.org/licenses/LICENSE-2.0
*
*  Unless required by applicable law or agreed to in writing, software
*  distributed under the License is distributed on an "AS IS" BASIS,
*  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*  See the License for the specific language governing permissions and
*  limitations under the License.
*
******************************************************************************/

package schwift

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var (
	//ErrChecksumMismatch is returned by Object.Upload() when the Etag in the
	//server response does not match the uploaded data.
	ErrChecksumMismatch = errors.New("Etag on uploaded object does not match MD5 checksum of uploaded data")
	//ErrNoContainerName is returned by Request.Do() if ObjectName is given, but
	//ContainerName is empty.
	ErrNoContainerName = errors.New("missing container name")
	//ErrMalformedContainerName is returned by Request.Do() if ContainerName
	//contains slashes.
	ErrMalformedContainerName = errors.New("container name may not contain slashes")
	//ErrNotSupported is returned by bulk operations, large object operations,
	//etc. if the server does not support the requested operation.
	ErrNotSupported = errors.New("operation not supported by this Swift server")
)

//UnexpectedStatusCodeError is generated when a request to Swift does not yield
//a response with the expected successful status code.
type UnexpectedStatusCodeError struct {
	ExpectedStatusCodes []int
	ActualResponse      *http.Response
	ResponseBody        []byte
}

//Error implements the builtin/error interface.
func (e UnexpectedStatusCodeError) Error() string {
	codeStrs := make([]string, len(e.ExpectedStatusCodes))
	for idx, code := range e.ExpectedStatusCodes {
		codeStrs[idx] = strconv.Itoa(code)
	}
	msg := fmt.Sprintf("expected %s response, got %d instead",
		strings.Join(codeStrs, "/"),
		e.ActualResponse.StatusCode,
	)
	if len(e.ResponseBody) > 0 {
		msg += ": " + string(e.ResponseBody)
	}
	return msg
}

//BulkObjectError is the error message for a single object in a bulk operation.
//It is not generated individually, only as part of BulkUploadError and BulkDeleteError.
type BulkObjectError struct {
	ContainerName string
	ObjectName    string
	StatusCode    int
}

//Error implements the builtin/error interface.
func (e BulkObjectError) Error() string {
	return fmt.Sprintf("%s/%s: %d %s",
		e.ContainerName, e.ObjectName,
		e.StatusCode, http.StatusText(e.StatusCode),
	)
}

//BulkUploadError is returned by Account.BulkUpload() when the archive was
//uploaded and unpacked successfully, but some (or all) files could not be
//saved in Swift.
type BulkUploadError struct {
	//StatusCode contains the overall HTTP status code of the operation.
	StatusCode int
	//ArchiveError contains the error that occurred while unpacking the archive,
	//or if the archive as a whole was not acceptable. If may be empty if no
	//error occurred at this point.
	ArchiveError string
	//ObjectErrors contains errors that occurred while trying to save an
	//individual file from the archive. It may be empty.
	ObjectErrors []BulkObjectError
}

//Error implements the builtin/error interface. To fit into one line, it
//condenses the ObjectErrors into a count.
func (e BulkUploadError) Error() string {
	result := fmt.Sprintf("%d %s", e.StatusCode, http.StatusText(e.StatusCode))
	if e.ArchiveError != "" {
		result += ": " + e.ArchiveError
	}
	if len(e.ObjectErrors) > 0 {
		result += fmt.Sprintf(" (+%d object errors)", len(e.ObjectErrors))
	}
	return result
}

//Is checks if the given error is an UnexpectedStatusCodeError for that status
//code. For example:
//
//	err := container.Delete(nil, nil)
//	if err != nil {
//	    if schwift.Is(err, http.StatusNotFound) {
//	        //container does not exist -> just what we wanted
//	        return nil
//	    } else {
//	        //report unexpected error
//	        return err
//	    }
//	}
func Is(err error, code int) bool {
	if e, ok := err.(UnexpectedStatusCodeError); ok {
		return e.ActualResponse.StatusCode == code
	}
	return false
}

//MalformedHeaderError is generated when a response from Swift contains a
//malformed header.
type MalformedHeaderError struct {
	Key        string
	ParseError error
}

//Error implements the builtin/error interface.
func (e MalformedHeaderError) Error() string {
	return "Bad header " + e.Key + ": " + e.ParseError.Error()
}
