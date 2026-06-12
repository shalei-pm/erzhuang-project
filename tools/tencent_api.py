#!/usr/bin/env python3
import argparse
import datetime
import hashlib
import hmac
import json
import sys
import time
import urllib.error
import urllib.request

from tencent_credentials import read_tencent_credentials


def sign(key, msg):
    return hmac.new(key, msg.encode("utf-8"), hashlib.sha256).digest()


def request(secret_id, secret_key, service, host, action, version, region, payload):
    timestamp = int(time.time())
    date = datetime.datetime.utcfromtimestamp(timestamp).strftime("%Y-%m-%d")
    body = json.dumps(payload, separators=(",", ":"))

    method = "POST"
    canonical_uri = "/"
    canonical_query = ""
    content_type = "application/json; charset=utf-8"
    canonical_headers = (
        f"content-type:{content_type}\n"
        f"host:{host}\n"
        f"x-tc-action:{action.lower()}\n"
    )
    signed_headers = "content-type;host;x-tc-action"
    hashed_payload = hashlib.sha256(body.encode("utf-8")).hexdigest()
    canonical_request = "\n".join(
        [
            method,
            canonical_uri,
            canonical_query,
            canonical_headers,
            signed_headers,
            hashed_payload,
        ]
    )

    algorithm = "TC3-HMAC-SHA256"
    credential_scope = f"{date}/{service}/tc3_request"
    hashed_request = hashlib.sha256(canonical_request.encode("utf-8")).hexdigest()
    string_to_sign = "\n".join(
        [algorithm, str(timestamp), credential_scope, hashed_request]
    )

    secret_date = sign(("TC3" + secret_key).encode("utf-8"), date)
    secret_service = sign(secret_date, service)
    secret_signing = sign(secret_service, "tc3_request")
    signature = hmac.new(
        secret_signing, string_to_sign.encode("utf-8"), hashlib.sha256
    ).hexdigest()

    authorization = (
        f"{algorithm} Credential={secret_id}/{credential_scope}, "
        f"SignedHeaders={signed_headers}, Signature={signature}"
    )
    headers = {
        "Authorization": authorization,
        "Content-Type": content_type,
        "Host": host,
        "X-TC-Action": action,
        "X-TC-Version": version,
        "X-TC-Timestamp": str(timestamp),
        "X-TC-Region": region,
    }

    req = urllib.request.Request(
        f"https://{host}", data=body.encode("utf-8"), headers=headers, method=method
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.status, resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read().decode("utf-8")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--service", required=True)
    parser.add_argument("--host", required=True)
    parser.add_argument("--action", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--region", required=True)
    parser.add_argument("--payload", required=True)
    args = parser.parse_args()

    secret_id, secret_key = read_tencent_credentials()
    status, text = request(
        secret_id=secret_id,
        secret_key=secret_key,
        service=args.service,
        host=args.host,
        action=args.action,
        version=args.version,
        region=args.region,
        payload=json.loads(args.payload),
    )
    print(text)
    if status >= 400:
        sys.exit(1)


if __name__ == "__main__":
    main()
