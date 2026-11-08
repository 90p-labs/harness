<?php

namespace App\Order\Domain;

use Psr\Log\LoggerInterface;

final class Order
{
    public function __construct(private readonly LoggerInterface $logger)
    {
    }
}
