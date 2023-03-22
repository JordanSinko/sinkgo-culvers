package akamai

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

type Relentless struct {
	Endpoint       string
	Authentication string

	mu             sync.Mutex
	userAgent      string
	scriptUrl      string
	scriptContents string
	config         string
}

func (f *Relentless) WithScriptUrl(url string) {
	f.scriptUrl = url
}

func (f *Relentless) WithScriptContents(contents string) {
	f.scriptContents = contents
}

func (f *Relentless) String() string {
	return "Relentless"
}

func (f *Relentless) GenerateUserAgent() (data string, err error) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
	f.userAgent = ua
	return ua, nil

}

func (f *Relentless) generateConfig() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.config == "" {

		scriptContentsEncoded := base64.StdEncoding.EncodeToString([]byte(f.scriptContents))

		reqBody := fmt.Sprintf(`{"key":"%s","b64":"%s"}`, f.Authentication, scriptContentsEncoded)
		req, err := http.NewRequest(http.MethodPost, "https://akamai.relentless-robotics.com/generateConfig", strings.NewReader(reqBody))

		if err != nil {
			return "", err
		}

		req.Header.Add("content-type", "application/json")

		res, err := http.DefaultClient.Do(req)

		if err != nil {
			return "", err
		}

		defer res.Body.Close()
		resBody, err := ioutil.ReadAll(res.Body)

		if err != nil {
			return "", err
		}

		return string(resBody), nil

	}

	return f.config, nil
}

func (f *Relentless) GenerateSensorData(options *GenerateSensorDataOptions) (data string, err error) {

	if f.config == "" {
		config, _ := f.generateConfig()
		f.config = config
	}

	var ioEvents string = ``

	if options.MouseEvents > 0 {
		ioEvents = fmt.Sprintf(`%s,"mouse_events":"%d"`, ioEvents, options.MouseEvents)
	}

	if options.KeyEvents > 0 {
		ioEvents = fmt.Sprintf(`%s,"key_events":"%d"`, ioEvents, options.KeyEvents)
	}

	// u, _ := url.Parse(options.Url)
	// baseUrl := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	reqBody := fmt.Sprintf(`{"key":"%s","url":"%s","_abck":"%s","bm_sz":"%s","user_agent":"%s"%s}`, f.Authentication, options.Url, options.Abck, options.Bmsz, options.UserAgent, ioEvents)
	req, err := http.NewRequest(http.MethodPost, "https://akamai.relentless-robotics.com/akamai", strings.NewReader(reqBody))

	if err != nil {
		return "", err
	}

	req.Header.Add("content-type", "application/json")

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
		return "", fmt.Errorf("unable to get valid sensor data")
	}

	return response.SensorData, nil

}
