package goreq

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

type itimeout interface {
	Timeout() bool
}

//Request represents an HTTP request to be sent by a client.
type Request struct {
	headers           []headerTuple
	cookies           []*http.Cookie
	Method            string
	Uri               string
	Body              interface{}
	QueryString       interface{}
	ContentType       string
	Accept            string
	Host              string
	UserAgent         string
	Compression       *compression
	BasicAuthUsername string
	BasicAuthPassword string
	ShowDebug         bool
	OnBeforeRequest   func(goreq *Request, httpreq *http.Request)
}

type compression struct {
	writer          func(buffer io.Writer) (io.WriteCloser, error)
	reader          func(buffer io.Reader) (io.ReadCloser, error)
	ContentEncoding string
}

//Response represents the response from an HTTP request.
type Response struct {
	*http.Response
	Uri  string
	Body *Body
	req  *http.Request
}

type headerTuple struct {
	name  string
	value string
}

type Body struct {
	reader           io.ReadCloser
	compressedReader io.ReadCloser
}

type Error struct {
	timeout bool
	Err     error
}

func (e *Error) Timeout() bool {
	return e.timeout
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (b *Body) Read(p []byte) (int, error) {
	if b.compressedReader != nil {
		return b.compressedReader.Read(p)
	}
	return b.reader.Read(p)
}

func (b *Body) Close() error {
	err := b.reader.Close()
	if b.compressedReader != nil {
		return b.compressedReader.Close()
	}
	return err
}

func (b *Body) FromJsonTo(o interface{}) error {
	return json.NewDecoder(b).Decode(o)
}

func (b *Body) ToString() (string, error) {
	body, err := ioutil.ReadAll(b)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func Gzip() *compression {
	reader := func(buffer io.Reader) (io.ReadCloser, error) {
		return gzip.NewReader(buffer)
	}
	writer := func(buffer io.Writer) (io.WriteCloser, error) {
		return gzip.NewWriter(buffer), nil
	}
	return &compression{writer: writer, reader: reader, ContentEncoding: "gzip"}
}

func Deflate() *compression {
	reader := func(buffer io.Reader) (io.ReadCloser, error) {
		return zlib.NewReader(buffer)
	}
	writer := func(buffer io.Writer) (io.WriteCloser, error) {
		return zlib.NewWriter(buffer), nil
	}
	return &compression{writer: writer, reader: reader, ContentEncoding: "deflate"}
}

func Zlib() *compression {
	return Deflate()
}

func paramParse(query interface{}) (string, error) {
	switch query.(type) {
	case url.Values:
		return query.(url.Values).Encode(), nil
	case *url.Values:
		return query.(*url.Values).Encode(), nil
	default:
		var v = &url.Values{}
		err := paramParseStruct(v, query)
		return v.Encode(), err
	}
}

func paramParseStruct(v *url.Values, query interface{}) error {
	var (
		s = reflect.ValueOf(query)
		t = reflect.TypeOf(query)
	)
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Interface {
		s = s.Elem()
		t = s.Type()
	}

	if t.Kind() != reflect.Struct {
		return errors.New("Can not parse QueryString.")
	}

	for i := 0; i < t.NumField(); i++ {
		var name string

		field := s.Field(i)
		typeField := t.Field(i)

		if !field.CanInterface() {
			continue
		}

		urlTag := typeField.Tag.Get("url")
		if urlTag == "-" {
			continue
		}

		name, opts := parseTag(urlTag)

		var omitEmpty, squash bool
		omitEmpty = opts.Contains("omitempty")
		squash = opts.Contains("squash")

		if squash {
			err := paramParseStruct(v, field.Interface())
			if err != nil {
				return err
			}
			continue
		}

		if urlTag == "" {
			name = strings.ToLower(typeField.Name)
		}

		if val := fmt.Sprintf("%v", field.Interface()); !(omitEmpty && len(val) == 0) {
			v.Add(name, val)
		}
	}
	return nil
}

func prepareRequestBody(b interface{}) (io.Reader, error) {
	switch b.(type) {
	case string:
		// treat is as text
		return strings.NewReader(b.(string)), nil
	case io.Reader:
		// treat is as text
		return b.(io.Reader), nil
	case []byte:
		//treat as byte array
		return bytes.NewReader(b.([]byte)), nil
	case nil:
		return nil, nil
	default:
		// try to jsonify it
		j, err := json.Marshal(b)
		if err == nil {
			return bytes.NewReader(j), nil
		}
		return nil, err
	}
}

//AddHeader add header in request.
func (request *Request) AddHeader(name string, value string) {
	if request.headers == nil {
		request.headers = []headerTuple{}
	}
	request.headers = append(request.headers, headerTuple{name: name, value: value})
}

//AddCookie add cookie in request.
func (request *Request) AddCookie(cookie *http.Cookie) {
	request.cookies = append(request.cookies, cookie)
}

func (r Request) addHeaders(headersMap http.Header) {
	if len(r.UserAgent) > 0 {
		headersMap.Add("User-Agent", r.UserAgent)
	}
	if r.Accept != "" {
		headersMap.Add("Accept", r.Accept)
	}
	if r.ContentType != "" {
		headersMap.Add("Content-Type", r.ContentType)
	}
}

//NewRequest returns a new Request given a method, URL, and optional body.
func (r Request) NewRequest() (*http.Request, error) {

	r.valueOrDefault()
	b, e := prepareRequestBody(r.Body)
	if e != nil {
		// there was a problem marshaling the body
		return nil, &Error{Err: e}
	}

	if r.QueryString != nil {
		param, e := paramParse(r.QueryString)
		if e != nil {
			return nil, &Error{Err: e}
		}
		r.Uri = r.Uri + "?" + param
	}

	var bodyReader io.Reader
	if b != nil && r.Compression != nil {
		buffer := bytes.NewBuffer([]byte{})
		readBuffer := bufio.NewReader(b)
		writer, err := r.Compression.writer(buffer)
		if err != nil {
			return nil, &Error{Err: err}
		}
		_, e = readBuffer.WriteTo(writer)
		writer.Close()
		if e != nil {
			return nil, &Error{Err: e}
		}
		bodyReader = buffer
	} else {
		bodyReader = b
	}

	req, err := http.NewRequest(r.Method, r.Uri, bodyReader)
	if err != nil {
		return nil, err
	}
	// add headers to the request
	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	} else {
		req.Host = r.Host	
	}

	r.addHeaders(req.Header)
	if r.Compression != nil {
		req.Header.Add("Content-Encoding", r.Compression.ContentEncoding)
		req.Header.Add("Accept-Encoding", r.Compression.ContentEncoding)
	}
	if r.headers != nil {
		for _, header := range r.headers {
			req.Header.Add(header.name, header.value)
		}
	}

	//use basic auth if required
	if r.BasicAuthUsername != "" {
		req.SetBasicAuth(r.BasicAuthUsername, r.BasicAuthPassword)
	}

	for _, c := range r.cookies {
		req.AddCookie(c)
	}
	return req, nil
}

// Return value if nonempty, def otherwise.
func (request Request) valueOrDefault() {
	if request.Method == "" {
		request.Method = "GET"
	}
}
