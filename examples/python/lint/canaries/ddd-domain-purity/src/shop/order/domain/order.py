from dataclasses import dataclass

import requests


@dataclass(frozen=True)
class Order:
    id: str
    session: requests.Session
