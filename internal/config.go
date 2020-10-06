package internal

type Config struct {
	Url       string            `json:"url"`
	Method    string            `json:"method"`
	Amount    int               `json:"amount,omitempty"`
	TargetRPS int               `json:"target_rps,omitempty"`
	Headers   map[string]string `json:"headers"`
	Payload   string            `json:"payload"`
	Logfile   string            `json:"logfile"`
}
