package msgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/s-hammon/p"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/time/rate"
)

const (
	DefaultBaseURL = "https://graph.microsoft.com/v1.0"
	DefaultAuthURL = "https://login.microsoftonline.com/"
	DefaultScopes  = "https://graph.microsoft.com/.default"

	DefaultTimeoutSeconds         = 5
	DefaultRequestsPerSecondLimit = 100
	DefaultBurst                  = DefaultRequestsPerSecondLimit * 2
)

type Client struct {
	BaseURL  string
	TenantID string
	ClientID string
	c        *http.Client
	limiter  *rate.Limiter
	logger   *slog.Logger
	scopes   []string
}

func NewClient(ctx context.Context, tenantId, clientId, clientSecret string, opts ...Option) *Client {
	c := &Client{
		BaseURL:  DefaultBaseURL,
		TenantID: tenantId,
		ClientID: clientId,
		limiter:  rate.NewLimiter(rate.Limit(DefaultRequestsPerSecondLimit), DefaultBurst),
		logger:   defaultLogger().With("component", "msgraph"),
		scopes:   []string{DefaultScopes},
	}

	for _, opt := range opts {
		opt(c)
	}

	adCfg := &clientcredentials.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		TokenURL:     DefaultAuthURL + p.Format("%s/oauth2/v2.0/token", tenantId),
		Scopes:       c.scopes,
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConns = 100
	tr.MaxIdleConnsPerHost = 100
	tr.IdleConnTimeout = 90 * time.Second

	bc := &http.Client{Transport: tr}

	ts := adCfg.TokenSource(ctx)
	ts = oauth2.ReuseTokenSource(nil, ts)

	// ctx = context.WithValue(ctx, oauth2.HTTPClient, bc)

	// c.c = adCfg.Client(ctx)
	c.c = oauth2.NewClient(
		context.WithValue(ctx, oauth2.HTTPClient, bc),
		ts,
	)

	return c
}

type Option func(*Client)

func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) { c.logger = logger }
}

func AddScopes(scopes ...string) Option {
	return func(c *Client) { c.scopes = append(c.scopes, scopes...) }
}

func OverrideScopes(scopes ...string) Option {
	return func(c *Client) { c.scopes = scopes }
}

func defaultLogger() *slog.Logger { return slog.Default() }

// Client.Get will error on any HTTP error or non-200 response codes
func (c *Client) Get(ctx context.Context, elems ...string) (io.ReadCloser, error) {
	u := c.joinPath(elems...)
	ctx = context.WithValue(ctx, pathKey, strings.Join(elems, "/"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func preview(b []byte, maxBytes int) string {
	n := min(len(b), maxBytes)
	return string(b[:n])
}

func (c *Client) joinPath(elems ...string) string {
	u, _ := url.JoinPath(c.BaseURL, elems...)
	return u
}

type batchReq struct {
	Requests []BatchRequest `json:"requests"`
}

func (c *Client) Batch(ctx context.Context, requests []BatchRequest) (iter.Seq2[ResponseObject, error], error) {
	u := c.joinPath("$batch")
	ctx = context.WithValue(ctx, pathKey, "$batch")
	const batchSize = 20

	if len(requests) == 0 {
		return func(yield func(ResponseObject, error) bool) {}, nil
	}

	return func(yield func(ResponseObject, error) bool) {
		for batch := range slices.Chunk(requests, batchSize) {
			b, _ := json.Marshal(batchReq{batch})
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
			req.Header.Add("Content-Type", "application/json")

			resp, err := c.do(req)
			if err != nil {
				yield(ResponseObject{}, err)
				return
			}

			for item, err := range BatchJSONSeq(resp.Body) {
				if !yield(item, err) {
					return
				}
			}
		}
	}, nil
}

type ctxKey string

const pathKey ctxKey = "msgraph_path"

func (c *Client) do(req *http.Request) (*http.Response, error) {
	path, _ := req.Context().Value(pathKey).(string)

	waitStart := time.Now()
	if err := c.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	waitDur := time.Since(waitStart)

	start := time.Now()
	resp, err := c.c.Do(req) // #nosec G704
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("request failed",
			"method", http.MethodGet,
			"url", path,
			"error", err,
			"duration", duration,
		)
		return nil, err
	}

	c.logger.Debug("response",
		"method", req.Method,
		"url", path,
		"status", resp.StatusCode,
		"duration", duration,
		"rate_limit_wait", waitDur,
	)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		c.logger.Warn("non-200 response",
			"url", path,
			"status", resp.StatusCode,
			// TODO: have an actual struct for this
			"body_preview", preview(body, 512),
		)
		return nil, fmt.Errorf("request returned non-200 code (%d)", resp.StatusCode)
	}

	return resp, nil
}

func BatchJSONSeq(body io.ReadCloser) iter.Seq2[ResponseObject, error] {
	return func(yield func(ResponseObject, error) bool) {
		defer func() {
			io.Copy(io.Discard, body)
			body.Close()
		}()
		dec := json.NewDecoder(body)
		const arrayKey = "responses"

		// Advance the decoder to the start of the "value" array
		for {
			t, err := dec.Token()
			if err == io.EOF {
				return
			}
			if err != nil {
				var zero ResponseObject
				yield(zero, err)
				return
			}

			// Look for the key name (e.g., "value")
			if s, ok := t.(string); ok && s == arrayKey {
				break
			}
		}

		// Expect the '[' token to start the array
		if _, err := dec.Token(); err != nil {
			var zero ResponseObject
			yield(zero, err)
			return
		}

		// Stream the items inside the array
		for dec.More() {
			var item ResponseObject
			if err := dec.Decode(&item); err != nil {
				var zero ResponseObject
				if !yield(zero, err) {
					return
				}
				break
			}
			if !yield(item, nil) {
				return
			}
		}
	}
}
