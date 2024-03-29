package okex

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kerwinzlb/GridTradingServer/log"
)

/*
 http client, request, response
 @author Tony Tian
 @date 2018-03-17
 @version 1.0.0
*/

type Client struct {
	Config     Config
	HttpClient *http.Client
}

type ApiMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

/*
 Get a http client
*/
func NewClient(config Config) *Client {
	var client Client
	client.Config = config
	timeout := config.TimeoutSecond
	if timeout <= 0 {
		timeout = 30
	}
	client.HttpClient = &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	return &client
}

/*
 Send a http request to remote server and get a response data
*/
func (client *Client) Request(method string, requestPath string,
	params, result interface{}) (response *http.Response, err error) {
	config := client.Config
	// uri
	endpoint := config.Endpoint
	if strings.HasSuffix(config.Endpoint, "/") {
		endpoint = config.Endpoint[0 : len(config.Endpoint)-1]
	}
	url := endpoint + requestPath

	// get json and bin styles request body
	var jsonBody string
	var binBody = bytes.NewReader(make([]byte, 0))
	if params != nil {
		jsonBody, binBody, err = ParseRequestParams(params)
		if err != nil {
			return response, err
		}
	}

	// get a http request
	request, err := http.NewRequest(method, url, binBody)
	if err != nil {
		return response, err
	}

	// Sign and set request headers
	timestamp := IsoTime()
	preHash := PreHashString(timestamp, method, requestPath, jsonBody)
	sign, err := HmacSha256Base64Signer(preHash, config.SecretKey)
	if err != nil {
		return response, err
	}
	Headers(request, config, timestamp, sign)

	if config.IsPrint {
		printRequest(config, request, jsonBody, preHash)
	}

	// send a request to remote server, and get a response
	response, err = client.HttpClient.Do(request)
	if err != nil {
		return response, err
	}
	defer response.Body.Close()

	// get a response results and parse
	status := response.StatusCode
	message := response.Status
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return response, err
	}

	if config.IsPrint {
		printResponse(status, message, body)
	}

	responseBodyString := string(body)

	response.Header.Add(ResultDataJsonString, responseBodyString)

	limit := response.Header.Get("Ok-Limit")
	if limit != "" {
		var page PageResult
		page.Limit = StringToInt(limit)
		from := response.Header.Get("Ok-From")
		if from != "" {
			page.From = StringToInt(from)
		}
		to := response.Header.Get("Ok-To")
		if to != "" {
			page.To = StringToInt(to)
		}
		pageJsonString, err := Struct2JsonString(page)
		if err != nil {
			return response, err
		}
		response.Header.Add(ResultPageJsonString, pageJsonString)
	}

	if status >= 200 && status < 300 {
		if body != nil && result != nil {
			err := JsonBytes2Struct(body, result)
			if err != nil {
				return response, err
			}
		}
		return response, nil
	} else if status >= 400 || status <= 500 {
		errMsg := "Http error(400~500) result: status=" + IntToString(status) + ", message=" + message + ", body=" + responseBodyString
		log.Error(errMsg)
		if body != nil {
			err := errors.New(errMsg)
			return response, err
		}
	} else {
		log.Error("Http error result: status=" + IntToString(status) + ", message=" + message + ", body=" + responseBodyString)
		return response, errors.New(message)
	}
	return response, nil
}

func printRequest(config Config, request *http.Request, body string, preHash string) {
	if config.SecretKey != "" {
		log.Debug("  Secret-Key: " + config.SecretKey)
	}
	log.Debug("  Request(" + IsoTime() + "):")
	log.Debug("\tUrl: " + request.URL.String())
	log.Debug("\tMethod: " + strings.ToUpper(request.Method))
	if len(request.Header) > 0 {
		log.Debug("\tHeaders: ")
		for k, v := range request.Header {
			if strings.Contains(k, "Ok-") {
				k = strings.ToUpper(k)
			}
			log.Debug("\t\t" + k + ": " + v[0])
		}
	}
	log.Debug("\tBody: " + body)
	if preHash != "" {
		log.Debug("  PreHash: " + preHash)
	}
}

func printResponse(status int, message string, body []byte) {
	log.Debug("  Response(" + IsoTime() + "):")
	statusString := strconv.Itoa(status)
	message = strings.Replace(message, statusString, "", -1)
	message = strings.Trim(message, " ")
	log.Debug("\tStatus: " + statusString)
	log.Debug("\tMessage: " + message)
	log.Debug("\tBody: " + string(body))
}
