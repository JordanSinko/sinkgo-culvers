package akamai

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	http "github.com/saucesteals/fhttp"
)

type Hawk struct {
	Authentication string

	initialized   bool
	userAgent     string
	scriptUrl     string
	scriptContent string
}

func (f *Hawk) initialize() error {

	hasher := md5.New()
	hasher.Write([]byte(f.scriptContent))
	hex := hex.EncodeToString(hasher.Sum(nil))

	reqPayload := fmt.Sprintf(`{"hash":"%s"}`, hex)
	req, err := http.NewRequest("POST", "https://ak-ppsaua.hwkapi.com/006180d12cf7/c", bytes.NewBuffer([]byte(reqPayload)))
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("X-Api-Key", "1eb72b04-0290-4733-8810-91630b1a0e49")
	req.Header.Set("X-Sec", "new")
	req.Header.Set("Content-Type", "application/json")

	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return err
	}

	bodyString := string(body)

	res.Body.Close()

	if bodyString == "false" {

		reqPayload := fmt.Sprintf(`{"body":"%s"}`, base64.StdEncoding.EncodeToString([]byte(f.scriptContent)))
		req, err := http.NewRequest("POST", "https://ak-ppsaua.hwkapi.com/006180d12cf7", bytes.NewBuffer([]byte(reqPayload)))

		req.Header.Set("Accept-Encoding", "gzip, deflate")
		req.Header.Set("X-Api-Key", "1eb72b04-0290-4733-8810-91630b1a0e49")
		req.Header.Set("X-Sec", "new")
		req.Header.Set("Content-Type", "application/json")

		if err != nil {
			return err
		}

		res, err := http.DefaultClient.Do(req)

		if err != nil {
			return err
		}

		body, err := io.ReadAll(res.Body)

		if err != nil {
			return err
		}

		bodyString := string(body)

		fmt.Println(string(bodyString))

		res.Body.Close()

	}

	f.initialized = true

	return nil

}

func (f *Hawk) WithScriptUrl(url string) {
	f.scriptUrl = url
}

func (f *Hawk) WithScriptContent(content string) {
	f.scriptContent = content
}

func (f *Hawk) String() string {
	return "Hawk"
}

func (f *Hawk) GenerateUserAgent() (data string, err error) {

	req, err := http.NewRequest(http.MethodGet, "https://ak01-eu.hwkapi.com/akamai/ua", nil)

	if err != nil {
		return "", err
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add("x-api-key", f.Authentication)

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	resBody, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return "", err
	}

	userAgent := string(resBody)
	f.userAgent = userAgent

	return userAgent, nil

}

func (f *Hawk) GenerateSensorData(options *GenerateSensorDataOptions) (data string, err error) {

	if !f.initialized {
		if err := f.initialize(); err != nil {
			return "", err
		}
	}

	reqBody := fmt.Sprintf(`{"site":"%s","abck":"%s","events":"1,1","bm_sz":"%s","user_agent":"%s"}`, options.Url, options.Abck, options.Bmsz, f.userAgent)
	req, _ := http.NewRequest("POST", "https://ak01-eu.hwkapi.com/akamai/generate", strings.NewReader(reqBody))
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("X-Api-Key", f.Authentication)
	req.Header.Set("X-Sec", "new")
	req.Header.Set("Content-Type", "application/json")

	if err != nil {
		return "", err
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

	payload := string(resBody)
	parts := strings.Split(payload, "****")
	sensorData := parts[0]

	return sensorData, nil

}
