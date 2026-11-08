from shop.order.infrastructure.order_table import OrderTable


class PlaceOrder:
    def __init__(self, orders: OrderTable) -> None:
        self.orders = orders
