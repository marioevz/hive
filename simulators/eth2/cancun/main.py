#!/usr/bin/env python
from re import match

from hive.parameters import Parameter
from hive.simulation import Simulation
from hive.testing import HiveTestResult

sim = Simulation(url="http://127.0.0.1:3000")

clients = sim.client_types()

assert clients

suite = sim.start_suite("my_test_suite", "my test suite description")

assert suite is not None

t = suite.start_test("my_test", "my test description")
assert t is not None

c1 = t.start_client(
    client_type=clients[0],
    parameters={},
    files={
        "genesis.json": "genesis.json",
    },
)

# Check enode
enode = c1.enode()
assert (
    match(r"enode://[0-9a-fA-F]{128}@\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d+", str(enode))
    is not None
)

n = suite.create_network("my_network")

n.connect_client(c1)

c2 = t.start_client(
    client_type=clients[0],
    parameters={
        Parameter.Bootnode: str(enode),
    },
    files={
        "genesis.json": "genesis.json",
    },
)

n.connect_client(c2)

t.end(result=HiveTestResult(test_pass=True, details="some details"))
suite.end()
