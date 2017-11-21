package net

type PayloadHeaders struct {
	Type      string `json:"type"`
	RequestId *int   `json:"requestId,omitempty"`
}

func (h *PayloadHeaders) setRequestId(id int) {
	h.RequestId = &id
}

func (h *PayloadHeaders) getRequestId() int {
	return *h.RequestId
}
