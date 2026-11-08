<?php

namespace App\Billing\Application;

use App\Order\Domain\Order;

final class IssueInvoice
{
    public function __construct(private readonly Order $order)
    {
    }
}
