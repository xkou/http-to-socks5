package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type RewriteTransport struct {
	Transport http.RoundTripper
}

func (t *RewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.Transport.RoundTrip(req)
}

func getProxyClient(proxy string, ignoreSsl bool) *http.Client {
	proxyUrl, _ := url.Parse(proxy)
	myClient := &http.Client{
		Transport: &RewriteTransport{
			&http.Transport{
				Proxy: http.ProxyURL(proxyUrl),
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: ignoreSsl,
				},
			},
		},
	}

	return myClient
}

func newProxyHandler(httpClient *http.Client, targetHost string, headersMap map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get path from request and append to target url
		target, _ := url.Parse(targetHost + r.URL.Path)
		if r.URL.RawQuery != "" {
			target.RawQuery = r.URL.RawQuery
		}

		// create new request
		req, err := http.NewRequest(r.Method, target.String(), r.Body)
		if err != nil {
			log.Println(err)
			return
		}

		// add default headers
		for k, v := range headersMap {
			req.Header.Set(k, v)
		}

		// copy headers from original request to new request

		// send request to target url
		resp, err := httpClient.Do(req)
		if err != nil {
			log.Println(err)
			return
		}

		// copy headers from response to original response
		for k, v := range resp.Header {
			w.Header()[k] = v
		}

		// copy status code from response to original response
		w.WriteHeader(resp.StatusCode)

		// copy body from response to original response using io.Copy
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Println(err)
			return
		}

		// close response body
		err = resp.Body.Close()
		if err != nil {
			log.Println(err)
			return
		}
	})
}

var listen = flag.String("l", "0.0.0.0:5100", "listen on this address defautl 0.0.0.0:5100")
var proxy = flag.String("p", "socks5://127.0.0.1:1080", "proxy server")
var target = flag.String("t", "127.0.0.1:80", "target server")

func main() {
	flag.Parse()
	// get server url from env
	serverUrl := *listen

	// get socks5 proxy from env
	proxyUrl := *proxy

	targetHost := *target

	ignoreSsl := os.Getenv("IGNORE_SSL")
	if ignoreSsl == "" {
		ignoreSsl = "false"
	}
	ignoreSslBool, err := strconv.ParseBool(ignoreSsl)
	if err != nil {
		log.Fatal(err)
	}

	// get default headers from env
	// example: "Content-Type:application/json,Authorization...."
	headersMap := make(map[string]string)
	headers := os.Getenv("DEFAULT_HEADERS")
	if headers != "" {
		for _, header := range strings.Split(headers, ",") {
			headerParts := strings.Split(header, ":")
			headersMap[headerParts[0]] = headerParts[1]
		}
	}

	httpClient := getProxyClient(proxyUrl, ignoreSslBool)

	//proxy all outgoing http requests to socks5 proxy
	log.Fatal(http.ListenAndServe(serverUrl, newProxyHandler(httpClient, targetHost, headersMap)))
}
