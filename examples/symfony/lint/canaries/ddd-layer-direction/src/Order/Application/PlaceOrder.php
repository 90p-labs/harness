<?php

namespace App\Order\Application;

use App\Order\Infrastructure\OrderTable;

final class PlaceOrder
{
    public function __construct(private readonly OrderTable $orders)
    {
    }
}
