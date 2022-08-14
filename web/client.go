package web

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/alpine-hodler/driver/web/transport"
	"golang.org/x/time/rate"
)

var defaultRateLimit = 1 * time.Second
var defaultRateLimitBurstCap = 5

// CoinbaseProClient is a wrapper for http.Client that can be used to make HTTP Requests to the Coinbase Pro API.
type Client struct{ http.Client }

func NewClient(_ context.Context, roundtripper transport.T) (*Client, error) {
	client := new(Client)
	client.Transport = roundtripper
	return client, nil
}

// newHTTPRequest will return a new request.  If the options are set, this function will encode a body if possible.
func newHTTPRequest(method string, u *url.URL) (*http.Request, error) {
	return http.NewRequest(method, u.String(), nil)
}

// parseErrorMessage takes a response and a status and builder an error message to send to the server.
func parseErrorMessage(resp *http.Response) error {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return fmt.Errorf("Status Code %v (%v): %v", resp.StatusCode, resp.Status, string(body))
}

// validateResponse is a switch condition that parses an error response
func validateResponse(res *http.Response) (err error) {
	if res == nil {
		err = fmt.Errorf("no response, check request and env file")
	} else {
		switch res.StatusCode {
		case
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusInternalServerError,
			http.StatusNotFound,
			http.StatusTooManyRequests,
			http.StatusForbidden:
			err = parseErrorMessage(res)
		}
	}
	return
}

type FetchConfig struct {
	Client      *Client
	Method      string
	URL         *url.URL
	RateLimiter *rate.Limiter
}

func (cfg *FetchConfig) validate() error {
	wrapper := func(field string) error { return fmt.Errorf("%q is a required field", field) }
	if cfg.Client == nil {
		return wrapper("Client")
	}
	if cfg.Method == "" {
		return wrapper("Method")
	}
	if cfg.URL == nil {
		return wrapper("URL")
	}
	return nil
}

// Fetch will make an HTTP request using the underlying client and endpoint.
func Fetch(ctx context.Context, cfg *FetchConfig) ([]byte, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// If the rate limiter is not set, set it with defaults.
	if cfg.RateLimiter == nil {
		cfg.RateLimiter = rate.NewLimiter(rate.Limit(defaultRateLimit), defaultRateLimitBurstCap)
	}

	req, err := newHTTPRequest(cfg.Method, cfg.URL)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("error waiting on rate limiter: %v", err)
	}

	resp, err := cfg.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request %+v: %v", req, err)
	}
	defer resp.Body.Close()

	if err := validateResponse(resp); err != nil {
		return nil, err
	}

	return io.ReadAll(resp.Body)
}