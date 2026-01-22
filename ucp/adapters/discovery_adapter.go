package adapters

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/talosprotocol/talos-sdk-go/ucp/domain"
)

type DiscoveryAdapter struct {
	HttpClient *http.Client
}

func NewDiscoveryAdapter() *DiscoveryAdapter {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, _ := net.SplitHostPort(addr)
			if port != "443" {
				return nil, fmt.Errorf("disallowed port: %s", port)
			}
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
		},
	}
	return &DiscoveryAdapter{
		HttpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

func (a *DiscoveryAdapter) FetchProfile(ctx context.Context, merchantBaseURL string) (*domain.MerchantProfile, error) {
	wellKnown := fmt.Sprintf("%s/.well-known/ucp", strings.TrimSuffix(merchantBaseURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", wellKnown, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery status %d", resp.StatusCode)
	}

	var profile domain.MerchantProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}
