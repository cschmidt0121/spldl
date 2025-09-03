package splunkclient

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/cschmidt0121/spldl/internal/config"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	auth       config.AuthConfig
}

func (c *Client) Get(path string, queryParams map[string]string) (string, error) {
	url := c.baseURL + path
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	q := request.URL.Query()
	for key, value := range queryParams {
		q.Add(key, value)
	}
	request.URL.RawQuery = q.Encode()

	return c.doRequest(request)
}

func (c *Client) Post(path string, contentType string, queryParams map[string]string, data []byte) (string, error) {
	url := c.baseURL + path
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}

	request.Header.Set("Content-Type", contentType)

	q := request.URL.Query()
	for key, value := range queryParams {
		q.Add(key, value)
	}
	request.URL.RawQuery = q.Encode()

	return c.doRequest(request)
}

func (c *Client) Delete(path string, queryParams map[string]string) (string, error) {
	url := c.baseURL + path
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return "", err
	}

	q := request.URL.Query()
	for key, value := range queryParams {
		q.Add(key, value)
	}
	request.URL.RawQuery = q.Encode()

	return c.doRequest(request)
}

func (c *Client) doRequest(request *http.Request) (string, error) {
	slog.Debug("Making HTTP request", "method", request.Method, "url", request.URL.String())

	switch c.auth.Type {
	case config.AuthHTTPBasic:
		request.SetBasicAuth(c.auth.Username, c.auth.Password)
		slog.Debug("Using HTTP Basic authentication")
	case config.AuthToken:
		request.Header.Set("Authorization", "Bearer "+c.auth.Token)
		slog.Debug("Using Bearer token authentication")
	}

	resp, err := c.httpClient.Do(request)
	if err != nil {
		slog.Debug("HTTP request failed", "error", err, "url", request.URL.String())
		return "", err
	}
	defer resp.Body.Close()

	slog.Debug("HTTP response received", "status_code", resp.StatusCode, "url", request.URL.String())

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug("Failed to read response body", "error", err)
		return "", err
	}

	slog.Debug("HTTP request completed successfully", "response_size", len(body), "url", request.URL.String())
	return string(body), nil
}

func NewClient(config config.ClientConfig) *Client {
	var baseURL string
	if config.UseTLS {
		baseURL = fmt.Sprintf("https://%s:%d", config.Host, config.Port)
	} else {
		baseURL = fmt.Sprintf("http://%s:%d", config.Host, config.Port)
	}

	var tlsConfig *tls.Config
	if config.UseTLS {
		tlsConfig = &tls.Config{InsecureSkipVerify: !config.VerifyTLS}
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
		auth: config.Auth,
	}
}

func NewClientWithHTTPClient(config config.ClientConfig, httpClient *http.Client) *Client {
	var baseURL string
	if config.UseTLS {
		baseURL = fmt.Sprintf("https://%s:%d", config.Host, config.Port)
	} else {
		baseURL = fmt.Sprintf("http://%s:%d", config.Host, config.Port)
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		auth:       config.Auth,
	}
}
