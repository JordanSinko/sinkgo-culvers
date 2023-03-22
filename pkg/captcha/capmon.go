package captcha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/valyala/fastjson"
)

type CapMon struct {
	Key string
}

func (t *CapMon) generateReCaptchaV2(o *SolveRecaptchaOptions) (string, error) {

	reqJson := map[string]interface{}{
		"clientKey": t.Key,
		"task": map[string]interface{}{
			"type":       "NoCaptchaTaskProxyless",
			"websiteURL": o.Url,
			"websiteKey": o.Key,
		},
	}

	reqBody, _ := json.Marshal(reqJson)
	req, _ := http.NewRequest(http.MethodPost, `https://api.capmonster.cloud/createTask`, bytes.NewBuffer(reqBody))

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", &SolverError{Retryable: false, OriginalError: err}
	}

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return "", &SolverError{Retryable: true, OriginalError: err}
	}

	resJson, err := fastjson.ParseBytes(body)

	if err != nil {
		return "", &SolverError{Retryable: true, OriginalError: err}
	}

	taskId := resJson.GetInt("taskId")
	errorId := resJson.GetInt("errorId")

	if errorId != 0 {
		return "", &SolverError{Retryable: false, OriginalError: fmt.Errorf(`error occured while executing captcha request: %d`, errorId)}
	}

	var response string
	var responseErr SolverError

	// Adding bailout (timeout, count, etc)
	for {

		time.Sleep(5 * time.Second)

		reqJson := map[string]interface{}{
			"clientKey": t.Key,
			"taskId":    taskId,
		}

		reqBody, _ := json.Marshal(reqJson)
		req, _ := http.NewRequest(http.MethodPost, `https://api.capmonster.cloud/getTaskResult`, bytes.NewBuffer(reqBody))

		res, err := http.DefaultClient.Do(req)

		if err != nil {
			return "", &SolverError{Retryable: false, OriginalError: err}
		}

		body, err := io.ReadAll(res.Body)

		if err != nil {
			return "", &SolverError{Retryable: true, OriginalError: err}
		}

		resJson, err := fastjson.ParseBytes(body)

		if err != nil {
			return "", &SolverError{Retryable: true, OriginalError: err}
		}

		statusBytes := resJson.GetStringBytes("status")
		errorId := resJson.GetInt("errorId")
		status := string(statusBytes)

		if errorId != 0 {
			return "", &SolverError{Retryable: false, OriginalError: fmt.Errorf(`error occured while executing captcha request: %d`, errorId)}
		}

		if status != "ready" {
			continue
		}

		responseBytes := resJson.GetStringBytes("solution", "gRecaptchaResponse")

		response = string(responseBytes)
		break

	}

	if responseErr.OriginalError == nil {
		return response, nil
	}

	return response, &responseErr

}

func (t *CapMon) Initialize() error {
	return nil
}

func (t *CapMon) SolveRecaptcha(o *SolveRecaptchaOptions) (string, error) {

	switch {
	case o.Version == "V2":
		return t.generateReCaptchaV2(o)
	// case o.Version == "V3":
	// 	return t.generateReCaptchaV3(o)
	default:
		return "", &SolverError{Retryable: false, OriginalError: fmt.Errorf("the version requested is not supported: %s", o.Version)}
	}

}
