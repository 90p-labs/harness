from shop.billing.domain.invoice import Invoice


class PlaceOrder:
    def __init__(self, invoice: Invoice) -> None:
        self.invoice = invoice
