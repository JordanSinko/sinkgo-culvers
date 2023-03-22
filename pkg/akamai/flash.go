package akamai

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type Flash struct {
	Endpoint       string
	Authentication string

	userAgent string
	scriptUrl string
}

func (f *Flash) GenerateUserAgent() (data string, err error) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
	//ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/102.0.5005.125 Safari/537.36"
	//ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.5359.99 Safari/537.36"
	f.userAgent = ua
	return ua, nil

}

func (f *Flash) String() string {
	return "Flash"
}

func (f *Flash) WithScriptContents(string) {

}

func (f *Flash) WithScriptUrl(url string) {
	f.scriptUrl = url
}

func (f *Flash) GenerateSensorData(options *GenerateSensorDataOptions) (data string, err error) {

	// u, _ := url.Parse(options.Url)
	// baseUrl := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	reqBody := fmt.Sprintf(`{"base_url":"%s","abckCookie":"%s","bmszCookie":"%s","UA":"%s","akamaiEndpoint":"%s"}`, options.Url, options.Abck, options.Bmsz, options.UserAgent, f.scriptUrl)
	req, err := http.NewRequest(http.MethodPost, "https://lqrjkw049i.execute-api.us-west-1.amazonaws.com/akamai/akamai", strings.NewReader(reqBody))

	if err != nil {
		return "", err
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add("x-api-key", f.Authentication)

	type Response struct {
		SensorData string `json:"sensor_data"`
	}

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	resBody, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return "", err
	}

	var response Response

	json.Unmarshal(resBody, &response)

	if response.SensorData == "" {
		return "", fmt.Errorf("unable to get sensor data")
	}

	return response.SensorData, nil

}
