package goreq

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/franela/goblin"
	. "github.com/onsi/gomega"
)

func TestClient(t *testing.T) {
	g := goblin.Goblin(t)
	RegisterFailHandler(func(m string, _ ...int) { g.Fail(m) })
	g.Describe("Create Client", func() {

		g.It("Should create a client HTTP default", func() {
			client := NewClient(Options{})

			Expect(client.Timeout).Should(Equal(time.Duration(5 * time.Second)))
			Expect(client.Transport).ShouldNot(BeNil())
			Expect(client.Jar).Should(BeNil())
			Expect(client.CheckRedirect).ShouldNot(BeNil())
			if transport, ok := client.Transport.(*http.Transport); ok {
				Expect(transport.TLSClientConfig.InsecureSkipVerify).Should(Equal(false))
			}

		})

		g.It("Should create a custom client HTTP", func() {
			options := Options{
				Insecure: true,
				Timeout:  time.Duration(20 * time.Second),
			}
			client := NewClient(options)

			Expect(client.Timeout).Should(Equal(options.Timeout))
			Expect(client.Transport).ShouldNot(BeNil())
			Expect(client.Jar).Should(BeNil())
			Expect(client.CheckRedirect).ShouldNot(BeNil())
			if transport, ok := client.Transport.(*http.Transport); ok {
				Expect(transport.TLSClientConfig.InsecureSkipVerify).Should(Equal(options.Insecure))
			}

		})

	})
}
func TestConcurrencyRequest(t *testing.T) {
	g := goblin.Goblin(t)
	RegisterFailHandler(func(m string, _ ...int) { g.Fail(m) })

	g.Describe("Concurrency Request", func() {
		var ts *httptest.Server

		g.Before(func() {
			ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && r.URL.Path == "/foo" {
					w.WriteHeader(200)
					fmt.Fprint(w, "bar")

				} else if r.Method == "GET" && r.URL.Path == "/" {
					w.Header().Add("x-forwarded-for", "test")
					w.Header().Add("Set-Cookie", "foo=bar")
					w.WriteHeader(200)
					w.Write([]byte(""))
				}
			}))
		})

		g.After(func() {
			ts.Close()
		})

		g.It("Should do 10 requests", func() {
			client := NewClient(Options{})

			var wg sync.WaitGroup

			for i := 0; i < 10; i++ {
				wg.Add(1)

				go func() {
					defer wg.Done()
					request := Request{Uri: ts.URL + "/foo"}

					res, err := client.Do(request)

					Expect(err).Should(BeNil())

					str, _ := res.Body.ToString()

					Expect(str).Should(Equal("bar"))
					Expect(res.StatusCode).Should(Equal(200))
				}()
			}
			wg.Wait()
		})

		g.It("Should do 10 requests with proxy", func() {
			client := NewClient(Options{
				Proxy: ts.URL,
			})

			var wg sync.WaitGroup

			for i := 0; i < 10; i++ {
				wg.Add(1)

				go func() {
					defer wg.Done()

					proxiedHost := "www.google.com"

					request := Request{
						Uri: "http://" + proxiedHost,
					}

					res, err := client.Do(request)

					Expect(err).Should(BeNil())
					Expect(res.Header.Get("x-forwarded-for")).Should(Equal("test"))
					Expect(res.req).ShouldNot(BeNil())
					Expect(res.req.URL.Hostname()).Should(Equal(proxiedHost))
				}()
			}
			wg.Wait()
		})

		g.It("Should do 10 requests with TLS", func() {
			client := NewClient(Options{
				Insecure: true,
			})

			var wg sync.WaitGroup

			for i := 0; i < 10; i++ {
				wg.Add(1)

				go func() {
					defer wg.Done()
					request := Request{Uri: ts.URL + "/foo"}

					res, err := client.Do(request)

					Expect(err).Should(BeNil())

					str, _ := res.Body.ToString()

					Expect(str).Should(Equal("bar"))
					Expect(client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify).Should(Equal(true))
					Expect(res.StatusCode).Should(Equal(200))
				}()
			}
			wg.Wait()
		})
	})
}

func TestTimeout(t *testing.T) {
	g := goblin.Goblin(t)
	RegisterFailHandler(func(m string, _ ...int) { g.Fail(m) })

	g.Describe("Timeouts", func() {
		var ts *httptest.Server
		var client Client
		var request Request

		g.Before(func() {
			client = NewClient(Options{})
			request = Request{Uri: "http://10.255.255.1"}

			ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if (r.Method == "GET" || r.Method == "OPTIONS" || r.Method == "TRACE" || r.Method == "PATCH" || r.Method == "FOOBAR") && r.URL.Path == "/foo" {
					w.WriteHeader(200)
					fmt.Fprint(w, "bar")
				}
			}))
		})

		g.After(func() {
			ts.Close()
		})

		g.Describe("Connection timeouts", func() {
			g.It("Should connect timeout after a default of 1000 ms", func() {
				start := time.Now()
				res, err := client.Do(request)
				elapsed := time.Since(start)

				Expect(elapsed).Should(BeNumerically("<", 1100*time.Millisecond))
				Expect(elapsed).Should(BeNumerically(">=", 1000*time.Millisecond))
				Expect(res.Response).Should(BeNil())
				Expect(err.(*Error).Timeout()).Should(BeTrue())
			})

			g.It("Should connect timeout after a custom amount of time", func() {
				client.setConnectTimeout(100 * time.Millisecond)

				start := time.Now()
				res, err := client.Do(request)
				elapsed := time.Since(start)

				Expect(elapsed).Should(BeNumerically("<", 150*time.Millisecond))
				Expect(elapsed).Should(BeNumerically(">=", 100*time.Millisecond))
				Expect(res.Response).Should(BeNil())
				Expect(err.(*Error).Timeout()).Should(BeTrue())
			})
			g.It("Should connect timeout after a custom amount of time even with method set", func() {
				client.setConnectTimeout(100 * time.Millisecond)

				start := time.Now()
				res, err := client.Do(request)
				elapsed := time.Since(start)

				Expect(elapsed).Should(BeNumerically("<", 150*time.Millisecond))
				Expect(elapsed).Should(BeNumerically(">=", 100*time.Millisecond))
				Expect(res.Response).Should(BeNil())
				Expect(err.(*Error).Timeout()).Should(BeTrue())
			})
		})

		g.Describe("Request timeout", func() {
			var ts *httptest.Server
			stop := make(chan bool)

			g.Before(func() {
				ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					<-stop
					// just wait for someone to tell you when to end the request. this is used to simulate a slow server
				}))

				client = NewClient(Options{})
				request = Request{Uri: ts.URL}

			})

			g.After(func() {
				stop <- true
				ts.Close()
			})

			g.It("Should request timeout after a custom amount of time", func() {
				client.setConnectTimeout(1000 * time.Millisecond)
				client.Timeout = 500 * time.Millisecond

				start := time.Now()
				res, err := client.Do(request)
				elapsed := time.Since(start)

				Expect(elapsed).Should(BeNumerically("<", 550*time.Millisecond))
				Expect(elapsed).Should(BeNumerically(">=", 500*time.Millisecond))
				Expect(res.Response).Should(BeNil())
				Expect(err.(*Error).Timeout()).Should(BeTrue())
			})
			g.It("Should request timeout after a custom amount of time even with proxy", func() {
				proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(2000 * time.Millisecond)
					w.WriteHeader(200)
				}))

				client.setConnectTimeout(1000 * time.Millisecond)
				client.Timeout = 500 * time.Millisecond
				client.setProxy(proxy.URL, nil)

				start := time.Now()
				res, err := client.Do(request)
				elapsed := time.Since(start)

				Expect(elapsed).Should(BeNumerically("<", 550*time.Millisecond))
				Expect(elapsed).Should(BeNumerically(">=", 500*time.Millisecond))
				Expect(res.Response).Should(BeNil())
				Expect(err.(*Error).Timeout()).Should(BeTrue())
			})
		})
	})
}

func TestRedirectPolicy(t *testing.T) {
	g := goblin.Goblin(t)
	RegisterFailHandler(func(m string, _ ...int) { g.Fail(m) })

	g.Describe("Redirects", func() {
		var ts *httptest.Server

		g.Before(func() {
			ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && r.URL.Path == "/redirect_test/301" {
					http.Redirect(w, r, "/redirect_test/302", http.StatusMovedPermanently)
				}
				if r.Method == "GET" && r.URL.Path == "/redirect_test/302" {
					http.Redirect(w, r, "/redirect_test/303", http.StatusFound)
				}
				if r.Method == "GET" && r.URL.Path == "/redirect_test/303" {
					http.Redirect(w, r, "/redirect_test/307", http.StatusSeeOther)
				}
				if r.Method == "GET" && r.URL.Path == "/redirect_test/307" {
					http.Redirect(w, r, "/getquery", http.StatusTemporaryRedirect)
				}
				if r.Method == "GET" && r.URL.Path == "/redirect_test/destination" {
					http.Redirect(w, r, ts.URL+"/destination", http.StatusMovedPermanently)
				}
				if r.Method == "GET" && r.URL.Path == "/destination" {
					w.WriteHeader(200)
					fmt.Fprint(w, "bar")
				}
			}))
		})

		g.After(func() {
			ts.Close()
		})

		g.It("Should not follow by default", func() {
			client := NewClient(Options{})

			request := Request{
				Uri: ts.URL + "/redirect_test/301",
			}

			res, err := client.Do(request)

			Expect(res.StatusCode).Should(Equal(301))
			Expect(err).ShouldNot(HaveOccurred())
		})

		g.It("Should not follow if method is explicitly specified", func() {
			client := NewClient(Options{})
			request := Request{
				Method: "GET",
				Uri:    ts.URL + "/redirect_test/301",
			}

			res, err := client.Do(request)

			Expect(res.StatusCode).Should(Equal(301))
			Expect(err).ShouldNot(HaveOccurred())
		})

		g.It("Should throw an error if MaxRedirect limit is exceeded", func() {
			client := NewClient(Options{
				MaxRedirects: 1,
			})

			request := Request{
				Method: "GET",
				Uri:    ts.URL + "/redirect_test/301",
			}

			res, err := client.Do(request)

			Expect(res.StatusCode).Should(Equal(302))
			Expect(err).Should(HaveOccurred())
		})

		g.It("Should copy request headers when redirecting if specified", func() {
			client := NewClient(Options{
				MaxRedirects: 4,
			})

			request := Request{
				Method: "GET",
				Uri:    ts.URL + "/redirect_test/301",
			}
			request.AddHeader("Testheader", "TestValue")

			res, _ := client.Do(request)

			Expect(res.StatusCode).Should(Equal(200))
			Expect(res.req.Header.Get("Testheader")).Should(Equal("TestValue"))
		})

		g.It("Should follow only specified number of MaxRedirects", func() {
			client := NewClient(Options{
				MaxRedirects: 1,
			})

			request := Request{
				Uri: ts.URL + "/redirect_test/301",
			}

			res, _ := client.Do(request)

			Expect(res.StatusCode).Should(Equal(302))

			client.setLimitRedirect(2)
			res, _ = client.Do(request)

			Expect(res.StatusCode).Should(Equal(303))

			client.setLimitRedirect(3)
			res, _ = client.Do(request)

			Expect(res.StatusCode).Should(Equal(307))

			client.setLimitRedirect(4)
			res, _ = client.Do(request)

			Expect(res.StatusCode).Should(Equal(200))
		})

		g.It("Should return final URL of the response when redirecting", func() {
			client := NewClient(Options{
				MaxRedirects: 2,
			})

			request := Request{
				Uri: ts.URL + "/redirect_test/destination",
			}

			res, _ := client.Do(request)

			Expect(res.Request.URL.String()).Should(Equal(ts.URL + "/destination"))
			Expect(res.Uri).Should(Equal(ts.URL + "/destination"))
		})
	})
}
