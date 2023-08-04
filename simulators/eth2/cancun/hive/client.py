from dataclasses import dataclass
from typing import Any, Dict


@dataclass(kw_only=True)
class ClientType:
    name: str
    version: str
    meta: Dict[str, Any]


@dataclass(kw_only=True)
class Client:
    type: str
    id: str
