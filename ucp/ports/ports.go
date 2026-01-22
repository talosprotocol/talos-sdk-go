package ports

import (
	"context"

	"github.com/talosprotocol/talos-sdk-go/ucp/domain"
)

type DiscoveryPort interface {
	FetchProfile(ctx context.Context, merchantBaseURL string) (*domain.MerchantProfile, error)
}

type ShoppingPort interface {
	CreateCheckout(ctx context.Context, merchantURL string, request interface{}, idempotencyKey string) (interface{}, error)
}
