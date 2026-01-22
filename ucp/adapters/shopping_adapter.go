package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/talosprotocol/talos-sdk-go/ucp/domain"
)

type ShoppingAdapter struct {
	Signer   *domain.Signer
	Platform *domain.PlatformProfile
	HTTP     *http.Client
}

func (a *ShoppingAdapter) CreateCheckout(ctx context.Context, merchantURL string, request interface{}, idempotencyKey string) (interface{}, error) {
	endpoint := fmt.Sprintf("%s/checkout-sessions", strings.TrimSuffix(merchantURL, "/"))

	bodyBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	sig, err := a.Signer.SignBody(bodyBytes)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	headers := domain.RequestHeaders{
		RequestID:      uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		Signature:      sig,
		AgentProfile:   a.Platform.Services.Platform.Profile.URL,
	}

	hMap, err := headers.ToMap()
	if err != nil {
		return nil, err
	}
	for k, v := range hMap {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("UCP error: %d", resp.StatusCode)
	}

	var response interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	return response, nil
}
