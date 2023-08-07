#!/usr/bin/env python

from hive.simulation import Simulation
from hive.tests import TestResult

print("Hello, World!")

sim = Simulation()

clients = sim.client_types()

assert clients

suite = sim.start_suite("my_test_suite", "my test suite description")

assert suite is not None

t = suite.start_test("my_test", "my test description")
assert t is not None

t.start_client(client_type=clients[0], parameters={}, files={"genesis.json": "genesis.json"})

t.end(result=TestResult(test_pass=True, details="some details"))
suite.end()
