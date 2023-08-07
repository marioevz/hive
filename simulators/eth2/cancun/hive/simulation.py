import os
from typing import List

import requests

from .client import ClientType
from .tests import TestSuite

# from enode import enode


class Simulation:
    url: str

    def __init__(self, url: str | None = None):
        if not url:
            url = os.getenv("HIVE_SIMULATOR")
            if not url:
                raise ValueError("HIVE_SIMULATOR environment variable not set")

        """
        p = os.getenv("HIVE_TEST_PATTERN")
        if p:
            m = parse_test_pattern(p)
            self.m = m
        """

        self.url = url

    """
    def set_test_pattern(self, p):
        m = parse_test_pattern(p)
        self.m = m

    def test_pattern(self):
        se = self.m.suite.String() if self.m.suite else ""
        te = self.m.test.String() if self.m.test else ""
        return se, te
    """

    def start_suite(self, name: str, description: str) -> TestSuite:
        url = f"{self.url}/testsuite"
        return TestSuite.start(url=url, name=name, description=description)

    def client_types(self) -> List[ClientType]:
        url = f"{self.url}/clients"
        response = requests.get(url)
        response.raise_for_status()
        client_list = response.json()
        assert isinstance(client_list, list)
        return [ClientType(**x) for x in client_list]
