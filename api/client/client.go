package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/mudler/edgevpn/pkg/edgevpn/types"
)

type (
	Client struct {
		host       string
		httpClient *http.Client
	}
)

const (
	machineURL    = "/api/machines"
	usersURL      = "/api/users"
	serviceURL    = "/api/services"
	blockchainURL = "/api/blockchain"
	ledgerURL     = "/api/ledger"
	fileURL       = "/api/files"
)

func WithHost(host string) func(c *Client) error {
	return func(c *Client) error {
		c.host = host
		return nil
	}
}

func WithTimeout(d time.Duration) func(c *Client) error {
	return func(c *Client) error {
		c.httpClient.Timeout = d
		return nil
	}
}

func WithHTTPClient(cl *http.Client) func(c *Client) error {
	return func(c *Client) error {
		c.httpClient = cl
		return nil
	}
}

type Option func(c *Client) error

func NewClient(o ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{},
	}
	for _, oo := range o {
		oo(c)
	}
	return c
}

func (c *Client) do(method, endpoint string, params map[string]string) (*http.Response, error) {
	baseURL := fmt.Sprintf("%s/%s", c.host, endpoint)
	req, err := http.NewRequest(method, baseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	q := req.URL.Query()
	for key, val := range params {
		q.Set(key, val)
	}
	req.URL.RawQuery = q.Encode()
	return c.httpClient.Do(req)
}

// Get methods (Services, Users, Files, Ledger, Blockchain, Machines)
func (c *Client) Services() (resp []types.Service, err error) {
	res, err := c.do(http.MethodGet, serviceURL, nil)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return resp, err
	}
	if err = json.Unmarshal(body, &resp); err != nil {
		return resp, err
	}
	return
}

func (c *Client) Files() (data []types.File, err error) {
	res, err := c.do(http.MethodGet, fileURL, nil)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return data, err
	}
	if err = json.Unmarshal(body, &data); err != nil {
		return data, err
	}
	return
}

func (c *Client) Users() (data []types.User, err error) {
	res, err := c.do(http.MethodGet, usersURL, nil)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return data, err
	}
	if err = json.Unmarshal(body, &data); err != nil {
		return data, err
	}
	return
}

func (c *Client) Ledger() (data map[string]string, err error) {
	res, err := c.do(http.MethodGet, ledgerURL, nil)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return data, err
	}
	if err = json.Unmarshal(body, &data); err != nil {
		return data, err
	}
	return
}

func (c *Client) Blockchain() (data []map[string]string, err error) {
	res, err := c.do(http.MethodGet, blockchainURL, nil)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return data, err
	}
	if err = json.Unmarshal(body, &data); err != nil {
		return data, err
	}
	return
}

func (c *Client) Machines() (resp []types.Machine, err error) {
	res, err := c.do(http.MethodGet, machineURL, nil)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return resp, err
	}
	if err = json.Unmarshal(body, &resp); err != nil {
		return resp, err
	}
	return
}
