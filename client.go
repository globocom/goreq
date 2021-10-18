package goreq

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/imdario/mergo"
)

const (
	defaultDialTimeout       = 1 * time.Second
	defaultDialKeepAlive     = 30 * time.Second
	defaultClientTimeout     = 5 * time.Second
	defaultClientMaxRedirect = 0
	defaultClientTLS         = false
)

//Options for create a client.
type Options struct {
	Timeout             time.Duration
	Insecure            bool
	MaxRedirects        int
	CookieJar           http.CookieJar
	Proxy               string
	ProxyConnectHeaders http.Header
	MaxIdleConnsPerHost int
}

//AddProxyConnectHeader add an Proxy connect header.
func (options Options) AddProxyConnectHeader(name string, value string) {
	if options.ProxyConnectHeaders == nil {
		options.ProxyConnectHeaders = make(http.Header)
	}
	options.ProxyConnectHeaders.Add(name, value)
}

//Client for do request in http.
type Client struct {
	*http.Client
}

var (
	defaultClientOptions = Options{
		Timeout:      defaultClientTimeout,
		Insecure:     defaultClientTLS,
		MaxRedirects: defaultClientMaxRedirect,
	}
)

// NewClient create a new client HTTP.
func NewClient(options Options) (client Client) {
	mergo.Merge(&options, defaultClientOptions)

	client = Client{newDefaultClient(options)}

	if options.Proxy != "" {
		client.setProxy(options.Proxy, options.ProxyConnectHeaders)
	}

	if options.MaxRedirects > 0 {
		client.setLimitRedirect(options.MaxRedirects)
	}

	return client
}

func newDefaultClient(options Options) *http.Client {
	dialer := &net.Dialer{
		Timeout:   defaultDialTimeout,
		KeepAlive: defaultDialKeepAlive,
	}
	transport := &http.Transport{
		DialContext: dialer.DialContext,
		Proxy:       http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: options.Insecure,
		},
		MaxIdleConnsPerHost: options.MaxIdleConnsPerHost,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   options.Timeout,
		Jar:       options.CookieJar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (client Client) setProxy(proxyURL string, proxyHeaders http.Header) *Error {
	url, err := url.Parse(proxyURL)
	if err != nil {
		return &Error{Err: err}
	}
	proxy := http.ProxyURL(url)

	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.Proxy = proxy
		transport.ProxyConnectHeader = proxyHeaders
	}

	return nil
}

func (client Client) setLimitRedirect(maxRedirects int) {
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return errors.New("GoReq: Error redirecting. MaxRedirects reached")
		}

		return nil
	}
}

func (client Client) check() error {
	if client.Timeout == time.Duration(0) {
		return errors.New("GoReq: Client without timeout")
	}
	return nil
}

func (client Client) setConnectTimeout(timeout time.Duration) {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: defaultDialKeepAlive,
	}
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.DialContext = dialer.DialContext
	}
}

//Do wraps DoWithContext using context.Background.
func (client Client) Do(request Request) (*Response, error) {
	return client.DoWithContext(context.Background(), request)
}

//DoWithContext sends an HTTP request and returns an HTTP response,
//following policy (such as redirects, cookies, auth) as configured on the client.
func (client Client) DoWithContext(ctx context.Context, request Request) (*Response, error) {

	if erro := client.check(); erro != nil {
		return nil, erro
	}

	req, err := request.NewRequestWithContext(ctx)
	if err != nil {
		return nil, &Error{Err: err}
	}

	if request.ShowDebug {
		dump, err := httputil.DumpRequest(req, true)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(dump))
	}

	if request.OnBeforeRequest != nil {
		request.OnBeforeRequest(&request, req)
	}

	res, err := client.Client.Do(req)

	if err != nil {
		timeout := false
		if t, ok := err.(itimeout); ok {
			timeout = t.Timeout()
		}
		if ue, ok := err.(*url.Error); ok {
			if t, ok := ue.Err.(itimeout); ok {
				timeout = t.Timeout()
			}
		}

		var body *Body
		var URL string
		if res != nil {
			body = &Body{reader: res.Body}
			URL = res.Request.URL.String()
		}

		return &Response{res, URL, body, req}, &Error{timeout: timeout, Err: err}
	}

	if request.Compression != nil && strings.Contains(res.Header.Get("Content-Encoding"), request.Compression.ContentEncoding) {
		compressedReader, err := request.Compression.reader(res.Body)
		if err != nil {
			return nil, &Error{Err: err}
		}
		return &Response{res, res.Request.URL.String(), &Body{reader: res.Body, compressedReader: compressedReader}, req}, nil
	}

	return &Response{res, res.Request.URL.String(), &Body{reader: res.Body}, req}, nil
}
