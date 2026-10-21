package marketplace

import "testing"

func TestOrderUpdateValidate(t *testing.T) {
	tests := []struct {
		name      string
		update    OrderUpdate
		wantError bool
	}{
		{"paid seller asset", OrderUpdate{OrderID: "ord-7", AccountID: "seller-2", State: OrderPaid, Kind: SellerAsset, Message: "Proof approved", AssetID: "asset-9"}, false},
		{"asset before payment", OrderUpdate{OrderID: "ord-7", AccountID: "seller-2", State: OrderShipped, Kind: SellerAsset, Message: "Proof approved", AssetID: "asset-9"}, true},
		{"buyer update after shipment", OrderUpdate{OrderID: "ord-7", AccountID: "buyer-4", State: OrderShipped, Kind: BuyerUpdate, Message: "Delivery window received"}, false},
		{"handoff too early", OrderUpdate{OrderID: "ord-7", AccountID: "seller-2", State: OrderPaid, Kind: OrderHandoff, Message: "Keys transferred"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.update.Validate()
			if (err != nil) != tt.wantError {
				t.Fatalf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
