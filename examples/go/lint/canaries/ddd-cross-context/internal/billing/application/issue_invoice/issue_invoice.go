package issueinvoice

import "example.com/shop/internal/order/domain"

type Invoice struct {
	Order domain.Order
}
