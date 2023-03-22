package captcha

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aidarkhanov/nanoid/v2"
	"gitlab.com/aycd-inc/autosolve-clients-v3/autosolve-client-go/autosolve"
)

type Aycd struct {
	Key string

	initialized bool
	service     *AutoSolveService
}

type Listener struct {
	autosolve.Listener
	service *AutoSolveService
}

func (l *Listener) OnStatusChanged(status autosolve.Status) {
	log.Printf("Status changed: %v\n", status)
}

func (l *Listener) OnError(err error) {
	log.Printf("Error: %v\n", err)
}

func (l *Listener) OnTokenResponse(tokenResponse *autosolve.CaptchaTokenResponse) {
	solveResponse := &CaptchaSolveResponse{
		Cancelled: false,
		Response:  tokenResponse,
	}

	l.service.mu.Lock()
	respChan := l.service.responseMap[tokenResponse.TaskId]

	if respChan != nil {
		respChan <- solveResponse
		delete(l.service.responseMap, tokenResponse.TaskId)
	}

	l.service.mu.Unlock()
}

func (l *Listener) OnTokenCancelResponse(cancelResponse *autosolve.CaptchaTokenCancelResponse) {
	solveResponse := &CaptchaSolveResponse{
		Cancelled: true,
		Response:  cancelResponse,
	}
	for _, request := range cancelResponse.Requests {
		respChan := l.service.responseMap[request.TaskId]
		if respChan != nil {
			respChan <- solveResponse
		}
	}
}

type AutoSolveService struct {
	listener    *Listener
	session     autosolve.Session
	responseMap map[string]chan *CaptchaSolveResponse
	mu          sync.Mutex
}

func NewAutoSolveService(clientId string) *AutoSolveService {
	autosolve.Init(clientId)
	service := &AutoSolveService{responseMap: make(map[string]chan *CaptchaSolveResponse)}
	service.listener = &Listener{service: service}
	return service
}

func (s *AutoSolveService) Connect(apiKey string) error {
	session, err := autosolve.GetSession(apiKey)
	if err != nil {
		return err
	}
	s.session = session
	s.session.SetListener(s.listener)
	return s.session.Open()
}

func (s *AutoSolveService) Close() error {
	if s.session != nil {
		return s.session.Close()
	}
	return nil
}

func (s *AutoSolveService) Solve(tokenRequest *autosolve.CaptchaTokenRequest) (*CaptchaSolveResponse, error) {

	if s.session == nil {
		return nil, autosolve.InvalidSessionError
	}
	channel := make(chan *CaptchaSolveResponse)

	s.mu.Lock()
	s.responseMap[tokenRequest.TaskId] = channel
	s.mu.Unlock()

	s.session.Send(tokenRequest)
	return <-channel, nil
}

func (s *AutoSolveService) SolveWithTimeout(tokenRequest *autosolve.CaptchaTokenRequest, timeout time.Duration) (*CaptchaSolveResponse, error) {
	if s.session == nil {
		return nil, autosolve.InvalidSessionError
	}
	channel := make(chan *CaptchaSolveResponse)

	s.mu.Lock()
	s.responseMap[tokenRequest.TaskId] = channel
	s.mu.Unlock()

	s.session.Send(tokenRequest)
	select {
	case msg := <-channel:
		return msg, nil
	case <-time.After(timeout):
		return &CaptchaSolveResponse{
			Cancelled: false,
			Timeout:   true,
			Response:  nil,
		}, nil
	}

}

type CaptchaSolveResponse struct {
	Cancelled bool
	Timeout   bool
	Response  autosolve.CaptchaResponse
}

func (r *CaptchaSolveResponse) SolveResponse() *autosolve.CaptchaTokenResponse {
	return r.Response.(*autosolve.CaptchaTokenResponse)
}

func (r *CaptchaSolveResponse) CancelResponse() *autosolve.CaptchaTokenCancelResponse {
	return r.Response.(*autosolve.CaptchaTokenCancelResponse)
}

func (a *Aycd) initialize() error {
	clientId := "SinkAIO-78db5223-4f6b-4b52-8c1a-5b5a5525eeb0"
	service := NewAutoSolveService(clientId)
	err := service.Connect(a.Key)

	if err != nil {
		return err
	}

	a.initialized = true
	a.service = service

	return nil
}

func (a *Aycd) generateReCaptchaV2(o *SolveRecaptchaOptions) (string, error) {

	taskId, _ := nanoid.New()

	message := &autosolve.CaptchaTokenRequest{
		TaskId:  taskId,
		Url:     o.Url,
		SiteKey: o.Key,
		Version: autosolve.ReCaptchaV2Checkbox,
	}

	log.Printf("trying to solve with aycd:%s", taskId)

	res, err := a.service.Solve(message)

	log.Printf("%v %v", res, err)

	if err != nil {
		return "", &SolverError{Retryable: true, OriginalError: err}
	}

	if res.Cancelled {
		return "", &SolverError{Retryable: true, OriginalError: fmt.Errorf("the request was cancelled")}
	} else if res.Timeout {
		return "", &SolverError{Retryable: true, OriginalError: fmt.Errorf("the request timed out")}
	}

	return res.SolveResponse().Token, nil
}

func (a *Aycd) generateReCaptchaV3(o *SolveRecaptchaOptions) (string, error) {

	taskId, _ := nanoid.New()

	message := &autosolve.CaptchaTokenRequest{
		TaskId:  taskId,
		Url:     o.Url,
		SiteKey: o.Key,
		Action:  o.Action,
		Version: autosolve.ReCaptchaV3,
	}

	res, err := a.service.Solve(message)

	if err != nil {
		return "", &SolverError{Retryable: true, OriginalError: err}
	}

	if res.Cancelled {
		return "", &SolverError{Retryable: true, OriginalError: fmt.Errorf("the request was cancelled")}
	} else if res.Timeout {
		return "", &SolverError{Retryable: true, OriginalError: fmt.Errorf("the request timed out")}
	}

	return res.SolveResponse().Token, nil
}

func (a *Aycd) Initialize() error {
	err := a.initialize()
	return err
}

func (a *Aycd) SolveRecaptcha(o *SolveRecaptchaOptions) (string, error) {

	if !a.initialized {
		if err := a.initialize(); err != nil {
			return "", &SolverError{Retryable: true, OriginalError: err}
		}
	}

	switch {
	case o.Version == "V2":
		return a.generateReCaptchaV2(o)
	case o.Version == "V3":
		return a.generateReCaptchaV3(o)
	default:
		return "", fmt.Errorf("the version requested is not supported: %s", o.Version)
	}

}
