from dataclasses import dataclass

import sqlalchemy


@dataclass(frozen=True)
class Order:
    id: str
    table: sqlalchemy.Table
