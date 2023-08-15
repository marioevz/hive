from dataclasses import dataclass
from typing import List

from hive.client import Client, ClientConfig


@dataclass(kw_only=True)
class ExecutionClient:
    config: ClientConfig
    client: Client | None = None


@dataclass(kw_only=True)
class BeaconClient:
    config: ClientConfig
    client: Client | None = None


@dataclass(kw_only=True)
class ValidatorClient:
    config: ClientConfig
    client: Client | None = None


@dataclass(kw_only=True)
class Node:
    index: int
    execution_client: ExecutionClient | None = None
    beacon_client: BeaconClient | None = None
    validation_client: ValidatorClient | None = None


@dataclass(kw_only=True)
class Testnet:
    nodes: List[Node]
