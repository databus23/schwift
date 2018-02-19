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
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestObjectLifecycle(t *testing.T) {
	testWithContainer(t, func(c *Container) {
		objectName := getRandomName()
		o := c.Object(objectName)

		expectString(t, o.Name(), objectName)
		expectString(t, o.FullName(), c.Name()+"/"+objectName)
		if o.Container() != c {
			t.Errorf("expected o.Container() = %#v, got %#v instead\n", c, o.Container())
		}

		exists, err := o.Exists()
		expectSuccess(t, err)
		expectBool(t, exists, false)

		_, err = o.Headers()
		expectError(t, err, "expected 200 response, got 404 instead")
		expectBool(t, Is(err, http.StatusNotFound), true)
		expectBool(t, Is(err, http.StatusNoContent), false)

		//DELETE should be idempotent and not return success on non-existence, but
		//OpenStack LOVES to be inconsistent with everything (including, notably, itself)
		err = o.Delete(nil, nil)
		expectError(t, err, "expected 204 response, got 404 instead: <html><h1>Not Found</h1><p>The resource could not be found.</p></html>")

		err = o.Upload(bytes.NewReader([]byte("test")), nil, nil)
		expectSuccess(t, err)

		exists, err = o.Exists()
		expectSuccess(t, err)
		expectBool(t, exists, true)

		err = o.Delete(nil, nil)
		expectSuccess(t, err)
	})
}

func TestObjectUpload(t *testing.T) {
	testWithContainer(t, func(c *Container) {
		validateUploadedFile := func(obj *Object, expected []byte) {
			str, err := obj.Download(nil, nil).AsString()
			expectSuccess(t, err)
			expectString(t, str, string(expected))
			obj.Invalidate()
			hdr, err := obj.Headers()
			expectSuccess(t, err)
			expectString(t, hdr.Etag().Get(), etagOf(expected))
		}

		//test upload with bytes.Reader
		obj := c.Object("upload1")
		err := obj.Upload(bytes.NewReader(objectExampleContent), nil, nil)
		expectSuccess(t, err)
		validateUploadedFile(obj, objectExampleContent)

		//test upload with bytes.Buffer
		obj = c.Object("upload2")
		err = obj.Upload(bytes.NewBuffer(objectExampleContent), nil, nil)
		expectSuccess(t, err)
		validateUploadedFile(obj, objectExampleContent)

		//test upload with opaque io.Reader
		obj = c.Object("upload3")
		err = obj.Upload(opaqueReader{bytes.NewReader(objectExampleContent)}, nil, nil)
		expectSuccess(t, err)
		validateUploadedFile(obj, objectExampleContent)

		//test upload with io.Writer
		obj = c.Object("upload4")
		err = obj.UploadWithWriter(nil, nil, func(w io.Writer) error {
			_, err := w.Write(objectExampleContent)
			return err
		})
		expectSuccess(t, err)
		validateUploadedFile(obj, objectExampleContent)

		//test upload with empty reader (should create zero-byte-sized object)
		obj = c.Object("upload5")
		err = obj.Upload(eofReader{}, nil, nil)
		expectSuccess(t, err)
		validateUploadedFile(obj, nil)

		//test upload without reader (should create zero-byte-sized object)
		obj = c.Object("upload6")
		err = obj.Upload(nil, nil, nil)
		expectSuccess(t, err)
		validateUploadedFile(obj, nil)
	})
}

type eofReader struct{}

func (r eofReader) Read([]byte) (int, error) {
	return 0, io.EOF
}

type opaqueReader struct {
	b *bytes.Reader
}

func (r opaqueReader) Read(buf []byte) (int, error) {
	return r.b.Read(buf)
}

func TestObjectDownload(t *testing.T) {
	testWithContainer(t, func(c *Container) {
		//upload example object
		obj := c.Object("example")
		err := obj.Upload(bytes.NewReader(objectExampleContent), nil, nil)
		expectSuccess(t, err)

		//test download as string
		str, err := obj.Download(nil, nil).AsString()
		expectSuccess(t, err)
		expectString(t, str, string(objectExampleContent))

		//test download as byte slice
		buf, err := obj.Download(nil, nil).AsByteSlice()
		expectSuccess(t, err)
		expectString(t, string(buf), string(objectExampleContent))

		//test download as io.ReadCloser slice
		reader, err := obj.Download(nil, nil).AsReadCloser()
		expectSuccess(t, err)
		buf = make([]byte, 4)
		_, err = reader.Read(buf)
		expectSuccess(t, err)
		expectString(t, string(buf), string(objectExampleContent[0:4]))
		_, err = reader.Read(buf)
		expectSuccess(t, err)
		expectString(t, string(buf), string(objectExampleContent[4:8]))
		buf, err = ioutil.ReadAll(reader)
		expectSuccess(t, err)
		expectString(t, string(buf), string(objectExampleContent[8:]))
	})
}

func TestObjectUpdate(t *testing.T) {
	testWithContainer(t, func(c *Container) {
		obj := c.Object("example")

		//test that metadata update fails for non-existing object
		newHeaders := make(ObjectHeaders)
		newHeaders.ContentType().Set("application/json")
		err := obj.Update(newHeaders, nil)
		expectBool(t, Is(err, http.StatusNotFound), true)
		expectError(t, err, "expected 202 response, got 404 instead: <html><h1>Not Found</h1><p>The resource could not be found.</p></html>")

		//create object
		err = obj.Upload(nil, nil, nil)
		expectSuccess(t, err)

		hdr, err := obj.Headers()
		expectSuccess(t, err)
		expectString(t, hdr.ContentType().Get(), "application/octet-stream")

		//now the metadata update should work
		err = obj.Update(newHeaders, nil)
		expectSuccess(t, err)
		obj.Invalidate()
		hdr, err = obj.Headers()
		expectSuccess(t, err)
		expectString(t, hdr.ContentType().Get(), "application/json")
	})
}
