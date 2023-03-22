package captcha

type CaptchaSolverProvider int64

type SolveRecaptchaOptions struct {
	Version   string
	Key       string
	Url       string
	Action    string
	Invisible bool
}

type CaptchaSolver interface {
	Initialize() error
	SolveRecaptcha(options *SolveRecaptchaOptions) (string, error)
}

type SolverError struct {
	Retryable     bool
	OriginalError error
}

func (e *SolverError) Error() string {
	return e.OriginalError.Error()
}
