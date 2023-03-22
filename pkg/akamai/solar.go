package akamai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Solar struct {
	Endpoint       string
	Authentication string

	userAgent string
}

func (f *Solar) GenerateUserAgent() (data string, err error) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
	//ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/102.0.5005.125 Safari/537.36"
	//ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.5359.99 Safari/537.36"
	f.userAgent = ua
	return ua, nil

}

func (f *Solar) String() string {
	return "Solar"
}

func (f *Solar) WithScriptContents(string) {

}

func (f *Solar) WithScriptUrl(string) {

}

func (f *Solar) GenerateSensorData(options *GenerateSensorDataOptions) (data string, err error) {

	// u, _ := url.Parse(options.Url)
	// baseUrl := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	reqJson := map[string]interface{}{
		"userAgent": options.UserAgent,
		"version":   "2",
		"pageUrl":   options.Url,
		"_abck":     options.Abck,
		"bm_sz":     options.Bmsz,
	}

	reqBody, err := json.Marshal(reqJson)

	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://akamai.publicapis.solarsystems.software/v1/sensor/generate", bytes.NewBuffer(reqBody))

	if err != nil {
		return "", err
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add("x-api-key", f.Authentication)

	type Response struct {
		SensorData string `json:"payload"`
	}

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		err := errors.New("response was not okay")
		return "", err
	}

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
