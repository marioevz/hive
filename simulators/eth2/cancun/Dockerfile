# Build the simulator binary
FROM python:3.10.2

ADD . /source
WORKDIR /source

RUN python -m pip install -r requirements.txt

RUN chmod +x main.py

ENTRYPOINT ["./main.py"]
