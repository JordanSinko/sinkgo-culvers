package captcha

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type TwoCaptcha struct {
	Key string
}

var baseUrl = `http://2captcha.com`

func (t *TwoCaptcha) generateReCaptchaV2(o *SolveRecaptchaOptions) (string, error) {

	invisible := "0"

	if o.Invisible {
		invisible = "1"
	}

	reqUrl := fmt.Sprintf(`%s/in.php?key=%s&method=userrecaptcha&googlekey=%s&pageurl=%s&invisible=%s`, baseUrl, t.Key, o.Key, o.Url, invisible)
	req, _ := http.NewRequest(http.MethodGet, reqUrl, nil)

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", &SolverError{Retryable: false, OriginalError: err}
	}

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return "", &SolverError{Retryable: true, OriginalError: err}
	}

	text := string(body)
	parts := strings.Split(text, `|`)
	status := parts[0]

	if status != "OK" {
		return "", &SolverError{Retryable: false, OriginalError: fmt.Errorf(`error occured while executing captcha request: %s`, status)}
	}

	requestId := parts[1]

	var response string
	var responseErr SolverError

	// Adding bailout (timeout, count, etc)
	for {

		time.Sleep(15 * time.Second)

		url := fmt.Sprintf(`%s/res.php?key=%s&action=get&id=%s`, baseUrl, t.Key, requestId)
		res, err := http.Get(url)

		if err != nil {
			return "", &SolverError{Retryable: true, OriginalError: err}
		}

		body, err := io.ReadAll(res.Body)

		if err != nil {
			return "", &SolverError{Retryable: true, OriginalError: err}
		}

		text := string(body)
		parts := strings.Split(text, `|`)
		status := parts[0]

		if status == "CAPCHA_NOT_READY" {
			continue
		}

		if status != "OK" {
			return "", &SolverError{Retryable: true, OriginalError: fmt.Errorf(`error occured while executing captcha request: %s`, status)}
		}

		response = parts[1]
		break

	}

	if responseErr.OriginalError == nil {
		return response, nil
	}

	return response, &responseErr

}

func (t *TwoCaptcha) generateReCaptchaV3(o *SolveRecaptchaOptions) (string, error) {
	reqUrl := fmt.Sprintf(`%s/in.php?key=%s&method=userrecaptcha&googlekey=%s&pageurl=%s&action=%s`, baseUrl, t.Key, o.Key, o.Url, o.Action)
	req, _ := http.NewRequest(http.MethodGet, reqUrl, nil)

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", &SolverError{Retryable: false, OriginalError: err}
	}

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return "", &SolverError{Retryable: true, OriginalError: err}
	}

	text := string(body)
	parts := strings.Split(text, `|`)
	status := parts[0]

	if status != "OK" {
		return "", &SolverError{Retryable: false, OriginalError: fmt.Errorf(`error occured while executing captcha request: %s`, status)}
	}

	requestId := parts[1]

	var response string
	var responseErr SolverError

	// Adding bailout (timeout, count, etc)
	for {

		time.Sleep(15 * time.Second)

		url := fmt.Sprintf(`%s/res.php?key=%s&action=get&id=%s`, baseUrl, t.Key, requestId)
		res, err := http.Get(url)

		if err != nil {
			return "", &SolverError{Retryable: true, OriginalError: err}
		}

		body, err := io.ReadAll(res.Body)

		if err != nil {
			return "", &SolverError{Retryable: true, OriginalError: err}
		}

		text := string(body)
		parts := strings.Split(text, `|`)
		status := parts[0]

		if status == "CAPCHA_NOT_READY" {
			continue
		}

		if status != "OK" {
			return "", &SolverError{Retryable: true, OriginalError: fmt.Errorf(`error occured while executing captcha request: %s`, status)}
		}

		response = parts[1]
		break

	}

	if responseErr.OriginalError == nil {
		return response, nil
	}

	return response, &responseErr

}

func (t *TwoCaptcha) Initialize() error {
	return nil
}

func (t *TwoCaptcha) SolveRecaptcha(o *SolveRecaptchaOptions) (string, error) {

	switch {
	case o.Version == "V2":
		return t.generateReCaptchaV2(o)
	case o.Version == "V3":
		return t.generateReCaptchaV3(o)
	default:
		return "", &SolverError{Retryable: false, OriginalError: fmt.Errorf("the version requested is not supported: %s", o.Version)}
	}

}
