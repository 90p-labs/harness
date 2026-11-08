<?php

namespace App\Order\Domain;

use Symfony\Component\Uid\Uuid;

final class Order
{
    public function __construct(public readonly Uuid $id)
    {
    }
}
