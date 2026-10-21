package marketplace

import (
	"errors"
	"fmt"
)

type OrderState string

const (
	OrderPaid            OrderState = "paid"
	OrderShipped         OrderState = "shipped"
	OrderReadyForHandoff OrderState = "ready_for_handoff"
)

type UpdateKind string

const (
	SellerAsset  UpdateKind = "seller_asset"
	BuyerUpdate  UpdateKind = "buyer_update"
	OrderHandoff UpdateKind = "order_handoff"
)

type OrderUpdate struct {
	OrderID   string     `json:"order_id"`
	AccountID string     `json:"account_id"`
	State     OrderState `json:"state"`
	Kind      UpdateKind `json:"kind"`
	Message   string     `json:"message"`
	AssetID   string     `json:"asset_id,omitempty"`
}

func (u OrderUpdate) Validate() error {
	if u.OrderID == "" || u.AccountID == "" || u.Message == "" {
		return errors.New("order_id, account_id, and message are required")
	}
	want := map[UpdateKind]OrderState{
		SellerAsset:  OrderPaid,
		BuyerUpdate:  OrderShipped,
		OrderHandoff: OrderReadyForHandoff,
	}
	required, ok := want[u.Kind]
	if !ok {
		return fmt.Errorf("unknown update kind %q", u.Kind)
	}
	if u.State != required {
		return fmt.Errorf("%s requires order state %s", u.Kind, required)
	}
	if u.Kind == SellerAsset && u.AssetID == "" {
		return errors.New("seller_asset requires asset_id")
	}
	return nil
}

func ChannelFor(orderID string) string {
	return "marketplace-order-" + orderID
}
