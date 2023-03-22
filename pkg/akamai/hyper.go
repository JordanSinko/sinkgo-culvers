package akamai

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type Hyper struct {
	Authentication string

	initialized       bool
	userAgent         string
	requests          int
	scriptContent     string
	stringContentHash string
}

func (f *Hyper) String() string {
	return "Hyper"
}

func (f *Hyper) WithScriptUrl(content string) {

}

func (f *Hyper) WithScriptContents(content string) {
	f.scriptContent = content

	hasher := sha256.New()
	hasher.Write([]byte(f.scriptContent))
	hex := hex.EncodeToString(hasher.Sum(nil))

	f.stringContentHash = hex
}

func (f *Hyper) GenerateUserAgent() (data string, err error) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36"
	f.userAgent = ua
	return ua, nil
}

func (f *Hyper) GenerateSensorData(options *GenerateSensorDataOptions) (data string, err error) {
	f.requests = f.requests + 1

	reqBody := fmt.Sprintf(`{"pageUrl":"%s","abck":"%s","bmsz":"%s","userAgent":"%s","version":"2","scriptHash":"%s"}`, options.Url, options.Abck, options.Bmsz, options.UserAgent, f.stringContentHash)
	req, err := http.NewRequest(http.MethodPost, "https://api.justhyped.dev/sensor", strings.NewReader(reqBody))

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
	resBody, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return "", err
	}

	var response Response

	json.Unmarshal(resBody, &response)

	return response.SensorData, nil

}
