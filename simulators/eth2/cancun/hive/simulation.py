import os
from dataclasses import asdict, dataclass
from typing import Callable, Dict, List

import requests

from .client import ClientType
from .client_setup import ClientConfig, ClientSetup

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

    def start_suite(self, name: str, description: str) -> "TestSuite | None":
        url = f"{self.url}/testsuite"
        req = {"Name": name, "Description": description}
        try:
            id = self.post(url, req, None)
            return TestSuite(name=name, description=description, id=id, sim=self)
        except requests.exceptions.HTTPError:
            pass
        return None

    def end_suite(self, test_suite: int):
        url = f"{self.url}/testsuite/{test_suite}"
        return self.request_delete(url)

    def start_test(self, test_suite: "TestSuite", name: str, description: str):
        url = f"{self.url}/testsuite/{test_suite.id}/test"
        req = {"Name": name, "Description": description}

        try:
            id = self.post(url, req, None)
            return Test(name=name, description=description, id=id, suite=test_suite, sim=self)
        except requests.exceptions.HTTPError:
            pass
        return None

    def end_test(self, test_suite: int, test: int, result: "TestResult"):
        url = f"{self.url}/testsuite/{test_suite}/test/{test}"
        self.post(url, result.to_dict(), None)

    def client_types(self) -> List[ClientType]:
        url = f"{self.url}/clients"
        resp = self.get(url)
        assert isinstance(resp, list)
        return [ClientType(**x) for x in resp]

    def start_client(
        self,
        test_suite: "TestSuite",
        test: "Test",
        client_type: ClientType,
        parameters: Dict[str, str],
        init_files: Dict[str, str | Callable],
    ):
        if not client_type:
            raise ValueError("missing 'CLIENT' parameter")
        return self.start_client_with_options(
            test_suite, test, client_type, parameters, init_files
        )

    def stop_client(self, test_suite, test, node_id):
        url = f"{self.url}/testsuite/{test_suite}/test/{test}/node/{node_id}"
        return self.request_delete(url)

    def start_client_with_options(
        self,
        test_suite: "TestSuite",
        test: "Test",
        client_type: ClientType,
        parameters: Dict[str, str],
        files: Dict[str, str | Callable],
    ):
        url = f"{self.url}/testsuite/{test_suite.id}/test/{test.id}/node"
        setup = ClientSetup(
            config=ClientConfig(client=client_type.name, environment=parameters),
            files=files,
        )

        resp = setup.post_with_files(url)
        ip = resp["IP"]
        if not ip:
            raise ValueError("no IP address returned")
        return resp["ID"], ip

    """
    def pause_client(self, test_suite, test, node_id):
        url = f"{self.url}/testsuite/{test_suite}/test/{test}/node/{node_id}/pause"
        return self.post(url, None, None)

    def unpause_client(self, test_suite, test, node_id):
        url = f"{self.url}/testsuite/{test_suite}/test/{test}/node/{node_id}/pause"
        return self.request_delete(url)

    def client_enode_url(self, test_suite, test, node):
        return self.client_enode_url_network(test_suite, test, node, "bridge")

    def client_enode_url_network(self, test_suite, test, node, network):
        resp, _ = self.client_exec(test_suite, test, node, ["enode.sh"])
        if resp["ExitCode"] != 0:
            raise ValueError("unexpected exit code for enode.sh")

        output = resp["Stdout"].strip()
        n = enode.ParseV4(output)

        tcp_port = n.TCP() or 30303
        udp_port = n.UDP() or 30303

        ip, _ = self.container_network_ip(test_suite, network, node)
        fixed_ip = enode.NewV4(n.Pubkey(), ip, tcp_port, udp_port)
        return fixed_ip.URLv4(), None

    def client_exec(self, test_suite, test, node_id, cmd):
        url = f"{self.url}/testsuite/{test_suite}/test/{test}/node/{node_id}/exec"
        req = {"Command": cmd}
        resp = self.post(url, req, None)
        return resp, None
    """

    def create_network(self, test_suite, network_name):
        url = f"{self.url}/testsuite/{test_suite}/network/{network_name}"
        return self.post(url, None, None)

    def remove_network(self, test_suite, network):
        url = f"{self.url}/testsuite/{test_suite}/network/{network}"
        return self.request_delete(url)

    def connect_container(self, test_suite, network, container_id):
        url = f"{self.url}/testsuite/{test_suite}/network/{network}/{container_id}"
        return self.post(url, None, None)

    def disconnect_container(self, test_suite, network, container_id):
        url = f"{self.url}/testsuite/{test_suite}/network/{network}/{container_id}"
        return self.request_delete(url)

    def container_network_ip(self, test_suite, network, container_id):
        url = f"{self.url}/testsuite/{test_suite}/network/{network}/{container_id}"
        resp = self.get(url)
        return resp, None

    def post(self, url, data, files):
        response = requests.post(url, json=data, files=files)
        response.raise_for_status()
        return response.json()

    def get(self, url):
        response = requests.get(url)
        response.raise_for_status()
        return response.json()

    def request_delete(self, url):
        response = requests.delete(url)
        response.raise_for_status()
        return response.json()


@dataclass(kw_only=True)
class TestSuite:
    name: str
    description: str
    id: int
    sim: Simulation

    def end(self):
        self.sim.end_suite(self.id)

    def start_test(self, name: str, description: str):
        return self.sim.start_test(self, name, description)


@dataclass(kw_only=True)
class TestResult:
    test_pass: bool
    details: str

    def to_dict(self):
        d = asdict(self)
        d["pass"] = d.pop("test_pass")
        return d


@dataclass(kw_only=True)
class Test:
    name: str
    description: str
    id: int
    suite: TestSuite
    sim: Simulation

    def end(self, result):
        self.sim.end_test(self.suite.id, self.id, result)

    def start_client(
        self,
        client_type: ClientType,
        parameters: Dict[str, str],
        init_files: Dict[str, str | Callable],
    ):
        return self.sim.start_client(self.suite, self, client_type, parameters, init_files)
