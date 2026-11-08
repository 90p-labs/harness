from dataclasses import dataclass


@dataclass(frozen=True)
class Cart:
    lines: int
