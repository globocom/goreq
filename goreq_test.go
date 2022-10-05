package goreq

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/franela/goblin"
	. "github.com/onsi/gomega"
)

type Query struct {
	Limit int
	Skip  int
}

func TestRequest(t *testing.T) {

	query := Query{
		Limit: 3,
		Skip:  5,
	}

	valuesQuery := url.Values{}
	valuesQuery.Set("name", "marcos")
	valuesQuery.Add("friend", "jonas")
	valuesQuery.Add("friend", "peter")

	g := goblin.Goblin(t)

	RegisterFailHandler(func(m string, _ ...int) { g.Fail(m) })

	g.Describe("Request", func() {

		g.Describe("General request methods", func() {
			var ts *httptest.Server
			var requestHeaders http.Header

			g.Before(func() {
				ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestHeaders = r.Header
					if (r.Method == "GET" || r.Method == "OPTIONS" || r.Method == "TRACE" || r.Method == "PATCH" || r.Method == "FOOBAR") && r.URL.Path == "/foo" {
						w.WriteHeader(200)
						fmt.Fprint(w, "bar")
					}
					if r.Method == "GET" && r.URL.Path == "/getquery" {
						w.WriteHeader(200)
						fmt.Fprint(w, fmt.Sprintf("%v", r.URL))
					}
					if r.Method == "GET" && r.URL.Path == "/getbody" {
						w.WriteHeader(200)
						io.Copy(w, r.Body)
					}
					if r.Method == "POST" && r.URL.Path == "/" {
						w.Header().Add("Location", ts.URL+"/123")
						w.WriteHeader(201)
						io.Copy(w, r.Body)
					}
					if r.Method == "POST" && r.URL.Path == "/getquery" {
						w.WriteHeader(200)
						fmt.Fprint(w, fmt.Sprintf("%v", r.URL))
					}
					if r.Method == "PUT" && r.URL.Path == "/foo/123" {
						w.WriteHeader(200)
						io.Copy(w, r.Body)
					}
					if r.Method == "DELETE" && r.URL.Path == "/foo/123" {
						w.WriteHeader(204)
					}
					if r.Method == "GET" && r.URL.Path == "/getcookies" {
						defer r.Body.Close()
						w.WriteHeader(200)
						fmt.Fprint(w, requestHeaders.Get("Cookie"))
					}
					if r.Method == "GET" && r.URL.Path == "/setcookies" {
						defer r.Body.Close()
						w.Header().Add("Set-Cookie", "foobar=42 ; Path=/")
						w.WriteHeader(200)
					}
					if r.Method == "GET" && r.URL.Path == "/compressed" {
						defer r.Body.Close()
						b := "{\"foo\":\"bar\",\"fuu\":\"baz\"}"
						gw := gzip.NewWriter(w)
						defer gw.Close()
						if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
							w.Header().Add("Content-Encoding", "gzip")
						}
						w.WriteHeader(200)
						gw.Write([]byte(b))
					}
					if r.Method == "GET" && r.URL.Path == "/compressed_deflate" {
						defer r.Body.Close()
						b := "{\"foo\":\"bar\",\"fuu\":\"baz\"}"
						gw := zlib.NewWriter(w)
						defer gw.Close()
						if strings.Contains(r.Header.Get("Content-Encoding"), "deflate") {
							w.Header().Add("Content-Encoding", "deflate")
						}
						w.WriteHeader(200)
						gw.Write([]byte(b))
					}
					if r.Method == "GET" && r.URL.Path == "/compressed_and_return_compressed_without_header" {
						defer r.Body.Close()
						b := "{\"foo\":\"bar\",\"fuu\":\"baz\"}"
						gw := gzip.NewWriter(w)
						defer gw.Close()
						w.WriteHeader(200)
						gw.Write([]byte(b))
					}
					if r.Method == "GET" && r.URL.Path == "/compressed_deflate_and_return_compressed_without_header" {
						defer r.Body.Close()
						b := "{\"foo\":\"bar\",\"fuu\":\"baz\"}"
						gw := zlib.NewWriter(w)
						defer gw.Close()
						w.WriteHeader(200)
						gw.Write([]byte(b))
					}
					if r.Method == "POST" && r.URL.Path == "/compressed" && r.Header.Get("Content-Encoding") == "gzip" {
						defer r.Body.Close()
						gr, _ := gzip.NewReader(r.Body)
						defer gr.Close()
						b, _ := ioutil.ReadAll(gr)
						w.WriteHeader(201)
						w.Write(b)
					}
					if r.Method == "POST" && r.URL.Path == "/compressed_deflate" && r.Header.Get("Content-Encoding") == "deflate" {
						defer r.Body.Close()
						gr, _ := zlib.NewReader(r.Body)
						defer gr.Close()
						b, _ := ioutil.ReadAll(gr)
						w.WriteHeader(201)
						w.Write(b)
					}
					if r.Method == "POST" && r.URL.Path == "/compressed_and_return_compressed" {
						defer r.Body.Close()
						w.Header().Add("Content-Encoding", "gzip")
						w.WriteHeader(201)
						io.Copy(w, r.Body)
					}
					if r.Method == "POST" && r.URL.Path == "/compressed_deflate_and_return_compressed" {
						defer r.Body.Close()
						w.Header().Add("Content-Encoding", "deflate")
						w.WriteHeader(201)
						io.Copy(w, r.Body)
					}
					if r.Method == "POST" && r.URL.Path == "/compressed_deflate_and_return_compressed_without_header" {
						defer r.Body.Close()
						w.WriteHeader(201)
						io.Copy(w, r.Body)
					}
					if r.Method == "POST" && r.URL.Path == "/compressed_and_return_compressed_without_header" {
						defer r.Body.Close()
						w.WriteHeader(201)
						io.Copy(w, r.Body)
					}
					if (r.Method == "GET") && r.URL.Path == "/timeout" {
						defer r.Body.Close()
						w.WriteHeader(200)
						io.Copy(w, r.Body)
					}
				}))
			})

			g.After(func() {
				ts.Close()
			})

			g.Describe("GET", func() {
				g.It("Should do a GET", func() {
					client := NewClient(Options{})
					request := Request{Uri: ts.URL + "/foo"}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("bar"))
					Expect(res.StatusCode).Should(Equal(200))
				})

				g.It("Should return ContentLength", func() {
					client := NewClient(Options{})
					request := Request{Uri: ts.URL + "/foo"}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("bar"))
					Expect(res.StatusCode).Should(Equal(200))
					Expect(res.ContentLength).Should(Equal(int64(3)))
				})

				g.It("Should do a GET with querystring", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:         ts.URL + "/getquery",
						QueryString: query,
					}

					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("/getquery?limit=3&skip=5"))
					Expect(res.StatusCode).Should(Equal(200))
				})

				g.It("Should support url.Values in querystring", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:         ts.URL + "/getquery",
						QueryString: valuesQuery,
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("/getquery?friend=jonas&friend=peter&name=marcos"))
					Expect(res.StatusCode).Should(Equal(200))
				})

				g.It("Should support sending string body", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:  ts.URL + "/getbody",
						Body: "foo",
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("foo"))
					Expect(res.StatusCode).Should(Equal(200))
				})

				g.It("Shoulds support sending a Reader body", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:  ts.URL + "/getbody",
						Body: strings.NewReader("foo"),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("foo"))
					Expect(res.StatusCode).Should(Equal(200))
				})

				g.It("Support sending any object that is json encodable", func() {
					client := NewClient(Options{})

					obj := map[string]string{"foo": "bar"}
					request := Request{
						Uri:  ts.URL + "/getbody",
						Body: obj,
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(200))
				})

				g.It("Support sending an array of bytes body", func() {
					client := NewClient(Options{})

					body := []byte{'f', 'o', 'o'}
					request := Request{
						Uri:  ts.URL + "/getbody",
						Body: body,
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("foo"))
					Expect(res.StatusCode).Should(Equal(200))
				})

				g.It("Should return an error when body is not JSON encodable", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:  ts.URL + "/getbody",
						Body: math.NaN(),
					}
					res, err := client.Do(request)

					Expect(res).Should(BeNil())
					Expect(err).ShouldNot(BeNil())
				})

				g.It("Should return a gzip reader if Content-Encoding is 'gzip'", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:         ts.URL + "/compressed",
						Compression: Gzip(),
					}
					res, err := client.Do(request)

					b, _ := ioutil.ReadAll(res.Body)

					Expect(err).Should(BeNil())
					Expect(res.Body.compressedReader).ShouldNot(BeNil())
					Expect(res.Body.reader).ShouldNot(BeNil())
					Expect(string(b)).Should(Equal("{\"foo\":\"bar\",\"fuu\":\"baz\"}"))
					Expect(res.Body.compressedReader).ShouldNot(BeNil())
					Expect(res.Body.reader).ShouldNot(BeNil())
				})

				g.It("Should close reader and compresserReader on Body close", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:         ts.URL + "/compressed",
						Compression: Gzip(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())

					_, e := ioutil.ReadAll(res.Body.reader)
					Expect(e).Should(BeNil())
					_, e = ioutil.ReadAll(res.Body.compressedReader)
					Expect(e).Should(BeNil())

					_, e = ioutil.ReadAll(res.Body.reader)
					//when reading body again it doesnt error
					Expect(e).Should(BeNil())

					res.Body.Close()
					_, e = ioutil.ReadAll(res.Body.reader)
					//error because body is already closed
					Expect(e).ShouldNot(BeNil())

					_, e = ioutil.ReadAll(res.Body.compressedReader)
					//compressedReaders dont error on reading when closed
					Expect(e).Should(BeNil())
				})

				g.It("Should not return a gzip reader if Content-Encoding is not 'gzip'", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:         ts.URL + "/compressed_and_return_compressed_without_header",
						Compression: Gzip(),
					}
					res, err := client.Do(request)

					b, _ := ioutil.ReadAll(res.Body)

					Expect(err).Should(BeNil())
					Expect(string(b)).ShouldNot(Equal("{\"foo\":\"bar\",\"fuu\":\"baz\"}"))
				})

				g.It("Should return a deflate reader if Content-Encoding is 'deflate'", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:         ts.URL + "/compressed_deflate",
						Compression: Deflate(),
					}
					res, err := client.Do(request)

					b, _ := ioutil.ReadAll(res.Body)

					Expect(err).Should(BeNil())
					Expect(string(b)).Should(Equal("{\"foo\":\"bar\",\"fuu\":\"baz\"}"))
				})

				g.It("Should not return a delfate reader if Content-Encoding is not 'deflate'", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:         ts.URL + "/compressed_deflate_and_return_compressed_without_header",
						Compression: Deflate(),
					}
					res, err := client.Do(request)

					b, _ := ioutil.ReadAll(res.Body)

					Expect(err).Should(BeNil())
					Expect(string(b)).ShouldNot(Equal("{\"foo\":\"bar\",\"fuu\":\"baz\"}"))
				})

				g.It("Should return a deflate reader when using zlib if Content-Encoding is 'deflate'", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:         ts.URL + "/compressed_deflate",
						Compression: Zlib(),
					}
					res, err := client.Do(request)

					b, _ := ioutil.ReadAll(res.Body)

					Expect(err).Should(BeNil())
					Expect(string(b)).Should(Equal("{\"foo\":\"bar\",\"fuu\":\"baz\"}"))
				})

				g.It("Should not return a delfate reader when using zlib if Content-Encoding is not 'deflate'", func() {
					client := NewClient(Options{})
					request := Request{
						Uri:         ts.URL + "/compressed_deflate_and_return_compressed_without_header",
						Compression: Zlib(),
					}
					res, err := client.Do(request)

					b, _ := ioutil.ReadAll(res.Body)

					Expect(err).Should(BeNil())
					Expect(string(b)).ShouldNot(Equal("{\"foo\":\"bar\",\"fuu\":\"baz\"}"))
				})

				g.It("Should send cookies from the cookiejar", func() {
					uri, err := url.Parse(ts.URL + "/getcookies")
					Expect(err).Should(BeNil())

					jar, err := cookiejar.New(nil)
					Expect(err).Should(BeNil())

					jar.SetCookies(uri, []*http.Cookie{
						{
							Name:  "bar",
							Value: "foo",
							Path:  "/",
						},
					})

					client := NewClient(Options{CookieJar: jar})

					request := Request{
						Uri: ts.URL + "/getcookies",
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("bar=foo"))
					Expect(res.StatusCode).Should(Equal(200))
					Expect(res.ContentLength).Should(Equal(int64(7)))
				})

				g.It("Should send cookies added with .AddCookie", func() {
					client := NewClient(Options{})

					c1 := &http.Cookie{Name: "c1", Value: "v1"}
					c2 := &http.Cookie{Name: "c2", Value: "v2"}

					request := Request{Uri: ts.URL + "/getcookies"}
					request.AddCookie(c1)
					request.AddCookie(c2)

					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("c1=v1; c2=v2"))
					Expect(res.StatusCode).Should(Equal(200))
					Expect(res.ContentLength).Should(Equal(int64(12)))
				})

				g.It("Should populate the cookiejar", func() {
					uri, err := url.Parse(ts.URL + "/setcookies")
					Expect(err).Should(BeNil())

					jar, _ := cookiejar.New(nil)
					Expect(err).Should(BeNil())

					client := NewClient(Options{
						CookieJar: jar,
					})

					request := Request{
						Uri: ts.URL + "/setcookies",
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())

					Expect(res.Header.Get("Set-Cookie")).Should(Equal("foobar=42 ; Path=/"))

					cookies := jar.Cookies(uri)
					Expect(len(cookies)).Should(Equal(1))

					cookie := cookies[0]
					Expect(*cookie).Should(Equal(http.Cookie{
						Name:  "foobar",
						Value: "42",
					}))
				})

				g.It("Should cancel the request", func() {
					ctx, cancel := context.WithCancel(context.Background())

					client := NewClient(Options{})
					request := Request{
						Uri:     ts.URL + "/timeout",
						Context: ctx,
					}
					res, err := client.Do(request)

					cancel()

					<-ctx.Done()

					Expect(err).Should(BeNil())
					Expect(res.StatusCode).Should(Equal(200))
				})
			})

			g.Describe("POST", func() {
				g.It("Should send a string", func() {
					client := NewClient(Options{})
					request := Request{
						Method: "POST",
						Uri:    ts.URL,
						Body:   "foo",
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("foo"))
					Expect(res.StatusCode).Should(Equal(201))
					Expect(res.Header.Get("Location")).Should(Equal(ts.URL + "/123"))
				})

				g.It("Should send a Reader", func() {
					client := NewClient(Options{})
					request := Request{
						Method: "POST",
						Uri:    ts.URL,
						Body:   strings.NewReader("foo"),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("foo"))
					Expect(res.StatusCode).Should(Equal(201))
					Expect(res.Header.Get("Location")).Should(Equal(ts.URL + "/123"))
				})

				g.It("Send any object that is json encodable", func() {
					client := NewClient(Options{})

					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method: "POST",
						Uri:    ts.URL,
						Body:   obj,
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
					Expect(res.Header.Get("Location")).Should(Equal(ts.URL + "/123"))
				})

				g.It("Send an array of bytes", func() {
					client := NewClient(Options{})

					body := []byte{'f', 'o', 'o'}
					request := Request{
						Method: "POST",
						Uri:    ts.URL,
						Body:   body,
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("foo"))
					Expect(res.StatusCode).Should(Equal(201))
					Expect(res.Header.Get("Location")).Should(Equal(ts.URL + "/123"))
				})

				g.It("Should return an error when body is not JSON encodable", func() {
					client := NewClient(Options{})
					request := Request{
						Method: "POST",
						Uri:    ts.URL,
						Body:   math.NaN(),
					}
					res, err := client.Do(request)

					Expect(res).Should(BeNil())
					Expect(err).ShouldNot(BeNil())
				})

				g.It("Should do a POST with querystring", func() {
					client := NewClient(Options{})
					body := []byte{'f', 'o', 'o'}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/getquery",
						Body:        body,
						QueryString: query,
					}

					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("/getquery?limit=3&skip=5"))
					Expect(res.StatusCode).Should(Equal(200))
				})

				g.It("Should send body as gzip if compressed", func() {
					client := NewClient(Options{})
					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/compressed",
						Body:        obj,
						Compression: Gzip(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
				})

				g.It("Should send body as deflate if compressed", func() {
					client := NewClient(Options{})
					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/compressed_deflate",
						Body:        obj,
						Compression: Deflate(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
				})

				g.It("Should send body as deflate using zlib if compressed", func() {
					client := NewClient(Options{})
					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/compressed_deflate",
						Body:        obj,
						Compression: Zlib(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					str, _ := res.Body.ToString()
					Expect(str).Should(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
				})

				g.It("Should send body as gzip if compressed and parse return body", func() {
					client := NewClient(Options{})
					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/compressed_and_return_compressed",
						Body:        obj,
						Compression: Gzip(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					b, _ := ioutil.ReadAll(res.Body)
					Expect(string(b)).Should(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
				})

				g.It("Should send body as deflate if compressed and parse return body", func() {
					client := NewClient(Options{})
					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/compressed_deflate_and_return_compressed",
						Body:        obj,
						Compression: Deflate(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					b, _ := ioutil.ReadAll(res.Body)
					Expect(string(b)).Should(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
				})

				g.It("Should send body as deflate using zlib if compressed and parse return body", func() {
					client := NewClient(Options{})
					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/compressed_deflate_and_return_compressed",
						Body:        obj,
						Compression: Zlib(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					b, _ := ioutil.ReadAll(res.Body)
					Expect(string(b)).Should(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
				})

				g.It("Should send body as gzip if compressed and not parse return body if header not set ", func() {
					client := NewClient(Options{})
					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/compressed_and_return_compressed_without_header",
						Body:        obj,
						Compression: Gzip(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					b, _ := ioutil.ReadAll(res.Body)
					Expect(string(b)).ShouldNot(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
				})

				g.It("Should send body as deflate if compressed and not parse return body if header not set ", func() {
					client := NewClient(Options{})
					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/compressed_deflate_and_return_compressed_without_header",
						Body:        obj,
						Compression: Deflate(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					b, _ := ioutil.ReadAll(res.Body)
					Expect(string(b)).ShouldNot(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
				})

				g.It("Should send body as deflate using zlib if compressed and not parse return body if header not set ", func() {
					client := NewClient(Options{})
					obj := map[string]string{"foo": "bar"}
					request := Request{
						Method:      "POST",
						Uri:         ts.URL + "/compressed_deflate_and_return_compressed_without_header",
						Body:        obj,
						Compression: Zlib(),
					}
					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					b, _ := ioutil.ReadAll(res.Body)
					Expect(string(b)).ShouldNot(Equal(`{"foo":"bar"}`))
					Expect(res.StatusCode).Should(Equal(201))
				})
			})

			g.It("Should do a PUT", func() {
				client := NewClient(Options{})
				request := Request{
					Method: "PUT",
					Uri:    ts.URL + "/foo/123",
					Body:   "foo",
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				str, _ := res.Body.ToString()
				Expect(str).Should(Equal("foo"))
				Expect(res.StatusCode).Should(Equal(200))
			})

			g.It("Should do a DELETE", func() {
				client := NewClient(Options{})
				request := Request{
					Method: "DELETE",
					Uri:    ts.URL + "/foo/123",
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				Expect(res.StatusCode).Should(Equal(204))
			})

			g.It("Should do a OPTIONS", func() {
				client := NewClient(Options{})
				request := Request{
					Method: "OPTIONS",
					Uri:    ts.URL + "/foo",
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				str, _ := res.Body.ToString()
				Expect(str).Should(Equal("bar"))
				Expect(res.StatusCode).Should(Equal(200))
			})

			g.It("Should do a PATCH", func() {
				client := NewClient(Options{})
				request := Request{
					Method: "PATCH",
					Uri:    ts.URL + "/foo",
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				str, _ := res.Body.ToString()
				Expect(str).Should(Equal("bar"))
				Expect(res.StatusCode).Should(Equal(200))
			})

			g.It("Should do a TRACE", func() {
				client := NewClient(Options{})
				request := Request{
					Method: "TRACE",
					Uri:    ts.URL + "/foo",
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				str, _ := res.Body.ToString()
				Expect(str).Should(Equal("bar"))
				Expect(res.StatusCode).Should(Equal(200))
			})

			g.It("Should do a custom method", func() {
				client := NewClient(Options{})
				request := Request{
					Method: "FOOBAR",
					Uri:    ts.URL + "/foo",
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				str, _ := res.Body.ToString()
				Expect(str).Should(Equal("bar"))
				Expect(res.StatusCode).Should(Equal(200))
			})

			g.Describe("Responses", func() {
				g.It("Should handle strings", func() {
					client := NewClient(Options{})
					request := Request{
						Method: "POST",
						Uri:    ts.URL,
						Body:   "foo bar",
					}
					res, _ := client.Do(request)

					str, _ := res.Body.ToString()
					Expect(str).Should(Equal("foo bar"))
				})

				g.It("Should handle io.ReaderCloser", func() {
					client := NewClient(Options{})
					request := Request{
						Method: "POST",
						Uri:    ts.URL,
						Body:   "foo bar",
					}
					res, _ := client.Do(request)

					body, _ := ioutil.ReadAll(res.Body)
					Expect(string(body)).Should(Equal("foo bar"))
				})

				g.It("Should handle parsing JSON", func() {
					client := NewClient(Options{})
					request := Request{
						Method: "POST",
						Uri:    ts.URL,
						Body:   `{"foo": "bar"}`,
					}
					res, _ := client.Do(request)

					var foobar map[string]string

					res.Body.FromJsonTo(&foobar)

					Expect(foobar).Should(Equal(map[string]string{"foo": "bar"}))
				})

				g.It("Should return the original request response", func() {
					client := NewClient(Options{})
					request := Request{
						Method: "POST",
						Uri:    ts.URL,
						Body:   `{"foo": "bar"}`,
					}
					res, _ := client.Do(request)

					Expect(res.Response).ShouldNot(BeNil())
				})
			})

		})

		g.Describe("Misc", func() {
			g.It("Should set default golang user agent when not explicitly passed", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("User-Agent")).ShouldNot(BeZero())
					Expect(r.Host).Should(Equal("foobar.com"))

					w.WriteHeader(200)
				}))
				defer ts.Close()

				client := NewClient(Options{})
				request := Request{
					Uri:  ts.URL,
					Host: "foobar.com",
				}
				res, err := client.Do(request)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(res.StatusCode).Should(Equal(200))
			})

			g.It("Should offer to set request headers", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("User-Agent")).Should(Equal("foobaragent"))
					Expect(r.Host).Should(Equal("foobar.com"))
					Expect(r.Header.Get("Accept")).Should(Equal("application/json"))
					Expect(r.Header.Get("Content-Type")).Should(Equal("application/json"))
					Expect(r.Header.Get("X-Custom")).Should(Equal("foobar"))
					Expect(r.Header.Get("X-Custom2")).Should(Equal("barfoo"))

					w.WriteHeader(200)
				}))
				defer ts.Close()

				client := NewClient(Options{})
				request := Request{
					Uri:         ts.URL,
					Accept:      "application/json",
					ContentType: "application/json",
					UserAgent:   "foobaragent",
					Host:        "foobar.com",
				}
				request.AddHeader("X-Custom", "foobar")
				request.AddHeader("X-Custom2", "barfoo")

				res, err := client.Do(request)

				Expect(res.StatusCode).Should(Equal(200))
				Expect(err).ShouldNot(HaveOccurred())
			})

			g.It("Should call hook before request", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("X-Custom")).Should(Equal("foobar"))

					w.WriteHeader(200)
				}))
				defer ts.Close()

				client := NewClient(Options{})
				hook := func(goreq *Request, httpreq *http.Request) {
					httpreq.Header.Add("X-Custom", "foobar")
				}
				request := Request{
					Uri:             ts.URL,
					OnBeforeRequest: hook,
				}
				res, _ := client.Do(request)

				Expect(res.StatusCode).Should(Equal(200))
			})

			g.It("Should not create a body by default", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					b, _ := ioutil.ReadAll(r.Body)
					Expect(b).Should(HaveLen(0))
					w.WriteHeader(200)
				}))
				defer ts.Close()

				client := NewClient(Options{})
				request := Request{
					Uri:  ts.URL,
					Host: "foobar.com",
				}
				client.Do(request)
			})

			g.It("Should change transport TLS config if Request.Insecure is set", func() {
				ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
				}))
				defer ts.Close()

				client := NewClient(Options{
					Insecure: true,
				})
				request := Request{
					Uri:  ts.URL,
					Host: "foobar.com",
				}
				res, _ := client.Do(request)

				if transport, ok := client.Transport.(*http.Transport); ok {
					Expect(transport.TLSClientConfig.InsecureSkipVerify).Should(Equal(true))
				}
				Expect(res.StatusCode).Should(Equal(200))
			})
			g.It("Should work if a different transport is specified", func() {
				ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
				}))
				defer ts.Close()

				dialer := &net.Dialer{Timeout: 1000 * time.Millisecond}
				transport := &http.Transport{
					DialContext:     dialer.DialContext,
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}

				client := NewClient(Options{})
				client.Transport = transport

				request := Request{
					Uri:  ts.URL,
					Host: "foobar.com",
				}
				res, _ := client.Do(request)

				if transport, ok := client.Transport.(*http.Transport); ok {
					Expect(transport.TLSClientConfig.InsecureSkipVerify).Should(Equal(true))
				}
				Expect(res.StatusCode).Should(Equal(200))
			})
			g.It("GetRequest should return the underlying httpRequest ", func() {
				req := Request{
					Host: "foobar.com",
				}

				request, _ := req.NewRequest()
				Expect(request).ShouldNot(BeNil())
				Expect(request.Host).Should(Equal(req.Host))
			})
		})

		g.Describe("Errors", func() {
			var ts *httptest.Server

			g.Before(func() {
				ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && r.URL.Path == "/" {
						w.Header().Add("Location", ts.URL+"/123")
						w.WriteHeader(201)
						io.Copy(w, r.Body)
					}
				}))
			})

			g.After(func() {
				ts.Close()
			})
			g.It("Should throw an error when FromJsonTo fails", func() {
				client := NewClient(Options{})
				request := Request{
					Method: "POST",
					Uri:    ts.URL,
					Body:   `{"foo" "bar"}`,
				}
				res, _ := client.Do(request)

				var foobar map[string]string

				err := res.Body.FromJsonTo(&foobar)
				Expect(err).Should(HaveOccurred())
			})
			g.It("Should handle Url parsing errors", func() {
				client := NewClient(Options{})
				request := Request{
					Uri: ":",
				}
				_, err := client.Do(request)

				Expect(err).ShouldNot(BeNil())
			})
			g.It("Should handle DNS errors", func() {
				client := NewClient(Options{})
				request := Request{
					Uri: "http://.localhost",
				}
				_, err := client.Do(request)

				Expect(err).ShouldNot(BeNil())
			})
		})

		g.Describe("Proxy", func() {
			var ts *httptest.Server
			g.Before(func() {
				ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "GET" && r.URL.Path == "/" {
						w.Header().Add("x-forwarded-for", "test")
						w.Header().Add("Set-Cookie", "foo=bar")
						w.WriteHeader(200)
						w.Write([]byte(""))
					} else if r.Method == "GET" && r.URL.Path == "/redirect_test/301" {
						http.Redirect(w, r, "/", 301)
					}
				}))

			})

			g.After(func() {
				ts.Close()
			})

			g.It("Should use Proxy", func() {
				proxiedHost := "www.google.com"
				client := NewClient(Options{
					Proxy: ts.URL,
				})
				request := Request{
					Uri: "http://" + proxiedHost,
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				Expect(res.Header.Get("x-forwarded-for")).Should(Equal("test"))
				Expect(res.Response.Request).ShouldNot(BeNil())
				Expect(res.req.URL.Hostname()).Should(Equal(proxiedHost))
			})

			g.It("Should not redirect if MaxRedirects is not set", func() {
				client := NewClient(Options{
					Proxy: ts.URL,
				})
				request := Request{
					Uri: ts.URL + "/redirect_test/301",
				}
				res, err := client.Do(request)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(res.StatusCode).Should(Equal(301))
			})

			g.It("Should use Proxy authentication", func() {
				proxiedHost := "www.google.com"
				uri := strings.Replace(ts.URL, "http://", "http://user:pass@", -1)

				client := NewClient(Options{
					Proxy: uri,
				})
				request := Request{
					Uri: "http://" + proxiedHost,
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				Expect(res.Header.Get("x-forwarded-for")).Should(Equal("test"))
			})

			g.It("Should propagate cookies", func() {
				proxiedHost, _ := url.Parse("http://www.google.com")
				jar, _ := cookiejar.New(nil)

				client := NewClient(Options{
					Proxy:     ts.URL,
					CookieJar: jar,
				})
				request := Request{
					Uri: proxiedHost.String(),
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				Expect(res.Header.Get("x-forwarded-for")).Should(Equal("test"))

				Expect(jar.Cookies(proxiedHost)).Should(HaveLen(1))
				Expect(jar.Cookies(proxiedHost)[0].Name).Should(Equal("foo"))
				Expect(jar.Cookies(proxiedHost)[0].Value).Should(Equal("bar"))
			})

			g.It("Should use ProxyConnectHeader authentication", func() {
				proxyHeaders := make(http.Header)
				proxyHeaders.Add("X-TEST-HEADER", "TEST")

				client := NewClient(Options{
					Proxy:               ts.URL,
					Insecure:            true,
					ProxyConnectHeaders: proxyHeaders,
				})
				request := Request{
					Uri: "http://10.255.255.1",
				}

				_, err := client.Do(request)

				if transport, ok := client.Transport.(*http.Transport); ok {
					Expect(transport.ProxyConnectHeader.Get("X-TEST-HEADER")).Should(Equal("TEST"))
				}
				Expect(err).Should(BeNil())
			})

		})

		g.Describe("BasicAuth", func() {
			var ts *httptest.Server

			g.Before(func() {
				ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/basic_auth" {
						authArray := r.Header["Authorization"]
						if len(authArray) > 0 {
							auth := strings.TrimSpace(authArray[0])
							w.WriteHeader(200)
							fmt.Fprint(w, auth)
						} else {
							w.WriteHeader(401)
							fmt.Fprint(w, "private")
						}
					}
				}))

			})

			g.After(func() {
				ts.Close()
			})

			g.It("Should support basic http authorization", func() {
				client := NewClient(Options{})
				request := Request{
					Uri:               ts.URL + "/basic_auth",
					BasicAuthUsername: "username",
					BasicAuthPassword: "password",
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				str, _ := res.Body.ToString()
				Expect(res.StatusCode).Should(Equal(200))
				expectedStr := "Basic " + base64.StdEncoding.EncodeToString([]byte("username:password"))
				Expect(str).Should(Equal(expectedStr))
			})

			g.It("Should fail when basic http authorization is required and not provided", func() {
				client := NewClient(Options{})
				request := Request{
					Uri: ts.URL + "/basic_auth",
				}
				res, err := client.Do(request)

				Expect(err).Should(BeNil())
				str, _ := res.Body.ToString()
				Expect(res.StatusCode).Should(Equal(401))
				Expect(str).Should(Equal("private"))
			})
		})
	})
}

func Test_paramParse(t *testing.T) {
	type Form struct {
		A string
		B string
		c string
	}

	type AnnotedForm struct {
		Foo  string `url:"foo_bar"`
		Baz  string `url:"bad,omitempty"`
		Norf string `url:"norf,omitempty"`
		Qux  string `url:"-"`
	}

	type EmbedForm struct {
		AnnotedForm `url:",squash"`
		Form        `url:",squash"`
		Corge       string `url:"corge"`
	}

	g := goblin.Goblin(t)
	RegisterFailHandler(func(m string, _ ...int) { g.Fail(m) })
	var form = Form{}
	var aform = AnnotedForm{}
	var eform = EmbedForm{}
	var values = url.Values{}
	const result = "a=1&b=2"
	g.Describe("QueryString ParamParse", func() {
		g.Before(func() {
			form.A = "1"
			form.B = "2"
			form.c = "3"
			aform.Foo = "xyz"
			aform.Norf = "abc"
			aform.Qux = "def"
			eform.Form = form
			eform.AnnotedForm = aform
			eform.Corge = "xxx"
			values.Add("a", "1")
			values.Add("b", "2")
		})
		g.It("Should accept struct and ignores unexported field", func() {
			str, err := paramParse(form)
			Expect(err).Should(BeNil())
			Expect(str).Should(Equal(result))
		})
		g.It("Should accept struct and use the field annotations", func() {
			str, err := paramParse(aform)
			Expect(err).Should(BeNil())
			Expect(str).Should(Equal("foo_bar=xyz&norf=abc"))
		})
		g.It("Should accept pointer of struct", func() {
			str, err := paramParse(&form)
			Expect(err).Should(BeNil())
			Expect(str).Should(Equal(result))
		})
		g.It("Should accept recursive pointer of struct", func() {
			f := &form
			ff := &f
			str, err := paramParse(ff)
			Expect(err).Should(BeNil())
			Expect(str).Should(Equal(result))
		})
		g.It("Should accept embedded struct", func() {
			str, err := paramParse(eform)
			Expect(err).Should(BeNil())
			Expect(str).Should(Equal("a=1&b=2&corge=xxx&foo_bar=xyz&norf=abc"))
		})
		g.It("Should accept interface{} which forcely converted by struct", func() {
			str, err := paramParse(interface{}(&form))
			Expect(err).Should(BeNil())
			Expect(str).Should(Equal(result))
		})

		g.It("Should accept url.Values", func() {
			str, err := paramParse(values)
			Expect(err).Should(BeNil())
			Expect(str).Should(Equal(result))
		})
		g.It("Should accept &url.Values", func() {
			str, err := paramParse(&values)
			Expect(err).Should(BeNil())
			Expect(str).Should(Equal(result))
		})
	})

}
