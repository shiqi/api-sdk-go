package smartling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	successResponseCode = "success"
)

// Post performs POST request to the Smartling API. You probably do not want
// to use it.
func (client *Client) Post(
	url string,
	body []byte,
	result interface{},
	options ...interface{},
) (json.RawMessage, int, error) {
	return client.requestJSON("POST", url, nil, body, result, options...)
}

// GetJSON performs GET request to the smartling API and tries to decode answer
// as JSON.
func (client *Client) GetJSON(
	url string,
	params url.Values,
	result interface{},
	options ...interface{},
) (json.RawMessage, int, error) {
	return client.requestJSON("GET", url, params, nil, result, options...)
}

// Get performs raw GET request to the Smartling API. You probably do not want
// to use it.
func (client *Client) Get(
	url string,
	params url.Values,
	options ...interface{},
) (io.ReadCloser, int, error) {
	return client.request("GET", url, params, nil, options...)
}

func (client *Client) request(
	method string,
	url string,
	params url.Values,
	body []byte,
	options ...interface{},
) (io.ReadCloser, int, error) {
	var (
		authenticate = true
		contentType  = "application/json"
	)

	for _, option := range options {
		switch value := option.(type) {
		case AuthenticationOption:
			authenticate = bool(value)

		case ContentTypeOption:
			contentType = string(value)
		}
	}

	if authenticate {
		err := client.Authenticate()
		if err != nil {
			return nil, 0, fmt.Errorf("unable to authenticate: %s", err)
		}
	}

	token := client.Credentials.AccessToken

	if body != nil {
		if contentType == "application/json" {
			client.Logger.Debugf(
				"<- %s %s %s [body %d bytes]\n%s",
				method, url, token, len(body), body,
			)
		} else {
			client.Logger.Debugf(
				"<- %s %s %s [body %d bytes form data]",
				method, url, token, len(body),
			)
		}
	} else {
		client.Logger.Debugf(
			"<- %s %s %s", method, url, token,
		)
	}

	startTime := time.Now()

	if len(params) > 0 {
		url += "?" + params.Encode()
	}

	request, err := http.NewRequest(
		method,
		client.BaseURL+url,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to create HTTP request: %s", err)
	}

	request.Header.Set("Content-Type", contentType)

	if client.Credentials.AccessToken != nil {
		request.Header.Set("Authorization", "Bearer "+token.Value)
	}

	reply, err := client.HTTP.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to perform HTTP request: %s", err)
	}

	client.Logger.Debugf(
		"-> %s [took %.2fs]",
		reply.Status,
		time.Now().Sub(startTime).Seconds(),
	)

	return reply.Body, reply.StatusCode, nil
}

func (client *Client) requestJSON(
	method string,
	url string,
	params url.Values,
	body []byte,
	result interface{},
	options ...interface{},
) (json.RawMessage, int, error) {
	reply, code, err := client.request(method, url, params, body, options...)
	if err != nil {
		return nil, code, err
	}

	defer reply.Close()

	var response struct {
		Response struct {
			Code string
			Data json.RawMessage
		}
	}

	if code != 200 {
		return nil, code, fmt.Errorf(
			"API call returned non-200 status code: %d", code,
		)
	}

	err = json.NewDecoder(reply).Decode(&response)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"unable to decode JSON response: %s", err,
		)
	}

	// we don't care about error here, it's only for logging
	message, _ := json.MarshalIndent(response, "", "  ")

	client.Logger.Debugf(
		"=> JSON [status=%s]\n%s",
		response.Response.Code,
		message,
	)

	if strings.ToLower(response.Response.Code) != "success" {
		return nil, 0, fmt.Errorf(
			`unexpected response status (expected "%s"): %#v`,
			successResponseCode,
			response.Response.Code,
		)
	}

	if result == nil {
		return response.Response.Data, code, nil
	}

	err = json.Unmarshal(response.Response.Data, result)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"unable to decode API response data: %s", err,
		)
	}

	return nil, code, nil
}
