package domain

type RequestHeaders struct {
	RequestID      string
	IdempotencyKey string
	Signature      string
	AgentProfile   string
}

func (h *RequestHeaders) ToMap() (map[string]string, error) {
	m := make(map[string]string)
	m["Request-Id"] = h.RequestID
	if h.IdempotencyKey != "" {
		m["Idempotency-Key"] = h.IdempotencyKey
	}
	if h.Signature != "" {
		m["Request-Signature"] = h.Signature
	}

	if h.AgentProfile != "" {
		dict := Dict{
			"profile": Item{Value: h.AgentProfile},
		}
		val, err := EncodeDict(dict)
		if err != nil {
			return nil, err
		}
		m["UCP-Agent"] = val
	}
	return m, nil
}
