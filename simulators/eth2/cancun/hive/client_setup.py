import io
import json
import os
import shutil
from dataclasses import asdict, dataclass, field
from http import client
from os import path
from typing import Callable, Dict, List

import requests


@dataclass(kw_only=True)
class ClientConfig:
    client: str
    networks: List[str] = field(default_factory=list)
    environment: Dict[str, str] = field(default_factory=dict)


@dataclass(kw_only=True)
class ClientSetup:
    config: ClientConfig
    files: Dict[str, str | Callable] = field(default_factory=dict)

    def post_with_files(self, url):
        pipe_r, pipe_w = os.pipe()
        buf_w = os.fdopen(pipe_w, "wb")
        form = MultipartForm(buf_w)

        # Write 'config' parameter first.
        fw = form.create_form_field("config")
        configs = json.dumps(asdict(self.config))
        fw.write(configs.encode("utf-8"))
        fw.write(b"\n")

        # Now upload the files.
        for filename, open_fn_or_name in self.files.items():
            fw = form.create_form_file(filename, path.basename(filename))
            try:
                if callable(open_fn_or_name):
                    with open_fn_or_name() as file_reader:
                        shutil.copyfileobj(file_reader, fw)
                elif isinstance(open_fn_or_name, str):
                    with open(open_fn_or_name, "rb") as file_reader:
                        shutil.copyfileobj(file_reader, fw)
            except Exception as err:
                print(f"Warning: upload error for {filename}: {err}")
                return err

        # Form must be closed or the request will be missing the terminating boundary.
        form.close()

        # Send the request.
        headers = {"content-type": form.get_content_type()}
        response = requests.post(url, data=pipe_r, headers=headers)
        os.close(pipe_r)

        if response.status_code != client.OK:
            return response.status_code, None

        try:
            response_json = response.json()
            return response.status_code, response_json
        except Exception as e:
            return response.status_code, str(e)


class MultipartForm:
    def __init__(self, buffer):
        self.form = buffer
        self.boundary = "Python-Multipart-Form"
        self.buffer = io.BytesIO()

    def write(self, data):
        self.buffer.write(data)

    def create_form_field(self, field_name):
        self.write(b"--" + self.boundary.encode("utf-8") + b"\r\n")
        self.write(f'Content-Disposition: form-data; name="{field_name}"\r\n\r\n'.encode("utf-8"))
        return self.buffer

    def create_form_file(self, field_name, filename):
        self.write(b"--" + self.boundary.encode("utf-8") + b"\r\n")
        self.write(
            f'Content-Disposition: form-data; name="{field_name}"; filename="{filename}"\r\n'.encode(
                "utf-8"
            )
        )
        self.write(b"Content-Type: application/octet-stream\r\n\r\n")
        return self.buffer

    def close(self):
        self.write(b"\r\n--" + self.boundary.encode("utf-8") + b"--\r\n")
        self.form.write(self.buffer.getvalue())

    def get_content_type(self):
        return f"multipart/form-data; boundary={self.boundary}"
