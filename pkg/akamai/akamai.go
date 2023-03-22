package akamai

type Provider int

const (
	FlashProvider Provider = 1 + iota
	FdisProvider
	RelentlessProvider
	HawkProvider
	HyperProvider
)

type GenerateSensorDataOptions struct {
	Url       string
	Abck      string
	Bmsz      string
	UserAgent string

	ScriptUrl     string
	ScriptEncoded string
	RequestCount  int
	MouseEvents   int
	KeyEvents     int
}

type AkamaiSolver interface {
	String() string
	WithScriptUrl(string)
	WithScriptContents(string)
	GenerateUserAgent() (string, error)
	GenerateSensorData(options *GenerateSensorDataOptions) (string, error)
}
