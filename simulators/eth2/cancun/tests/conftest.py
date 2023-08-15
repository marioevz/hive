"""
Pytest definitions applied to all tests.
"""

from itertools import count, cycle
from typing import Generator, Iterator, List

import pytest
from hive.client import ClientConfig, ClientRole, ClientType
from hive.simulation import Simulation
from hive.testing import HiveTest, HiveTestResult, HiveTestSuite

from .common import BeaconClient, ExecutionClient, Node, Testnet, ValidatorClient


@pytest.fixture(scope="session")
def sim() -> Simulation:
    """
    Fixture to define the hive simulation.

    It automatically encapsulates all tests inside a test file.
    """
    return Simulation()


@pytest.fixture(scope="module")
def suite(sim: Simulation) -> Generator[HiveTestSuite, None, None]:
    """
    Fixture to define the hive test suite.

    It automatically encapsulates all tests inside a test file.
    """
    s = sim.start_suite("some test suite", "some test description")
    yield s
    s.end()


@pytest.fixture(autouse=True)
def t(suite: HiveTestSuite) -> Generator[HiveTest, None, None]:
    """
    Fixture to define a hive test.

    It automatically creates the hive test object for every single test.
    """
    t = suite.start_test("some test", "some test description")
    yield t
    t.end(result=HiveTestResult(test_pass=True, details="some details"))


@pytest.fixture
def execution_client_type_iterator(sim: Simulation) -> Iterator[ClientType]:
    """
    Fixture to return the execution client types in a cycle
    """
    clients = sim.client_types(role=ClientRole.ExecutionClient)
    assert len(clients) > 0
    return cycle(clients)


@pytest.fixture
def beacon_client_type_iterator(sim: Simulation) -> Iterator[ClientType]:
    """
    Fixture to return the beacon client types in a cycle
    """
    clients = sim.client_types(role=ClientRole.BeaconClient)
    assert len(clients) > 0
    return cycle(clients)


@pytest.fixture
def validator_client_type_iterator(sim: Simulation) -> Iterator[ClientType]:
    """
    Fixture to return the validator client types in a cycle
    """
    clients = sim.client_types(role=ClientRole.ValidatorClient)
    assert len(clients) > 0
    return cycle(clients)


@pytest.fixture
def node_iterator(
    execution_client_type_iterator: Iterator[ClientType],
    beacon_client_type_iterator: Iterator[ClientType],
    validator_client_type_iterator: Iterator[ClientType],
) -> Iterator[Node]:
    """
    Fixture to return the node iterator, which is used to create the testnet.

    The nodes returned only contain the node configuration, but the clients are not started.
    """

    class NodeIterator:
        index: Iterator[int]

        def __init__(self):
            self.index = count(0)

        def __iter__(self):
            return self

        def __next__(self) -> Node:
            return Node(
                index=next(self.index),
                execution_client=ExecutionClient(
                    config=ClientConfig(client_type=next(execution_client_type_iterator))
                ),
                beacon_client=BeaconClient(
                    config=ClientConfig(client_type=next(beacon_client_type_iterator))
                ),
                validation_client=ValidatorClient(
                    config=ClientConfig(client_type=next(validator_client_type_iterator))
                ),
            )

    return NodeIterator()


@pytest.fixture
def node_count() -> int:
    """
    Fixture to define the number of nodes in the testnet.
    """
    return 1


@pytest.fixture
def nodes(
    node_iterator: Iterator[Node],
    node_count: int,
) -> List[Node]:
    """
    Fixture to return the nodes of a given test case.
    """
    return [next(node_iterator) for _ in range(node_count)]
