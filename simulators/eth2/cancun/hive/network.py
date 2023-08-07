from dataclasses import dataclass

import requests

from .client import Client


@dataclass(kw_only=True)
class Network:
    url: str
    name: str

    @classmethod
    def create(cls, url: str, name: str) -> "Network":
        response = requests.post(url)
        response.raise_for_status()
        return cls(url=f"{url}/{name}", name=name)

    def remove(self):
        response = requests.delete(self.url)
        response.raise_for_status()
        return response.json()

    def connect_client(self, client: Client):
        response = requests.post(f"{self.url}/{client.id}")
        response.raise_for_status()
        return response.json()

    def disconnect_client(self, client: Client):
        response = requests.delete(f"{self.url}/{client.id}")
        response.raise_for_status()
        return response.json()
    
    def client_ip(self, client: Client):
        response = requests.get(f"{self.url}/{client.id}")
        response.raise_for_status()
        return response.json()
