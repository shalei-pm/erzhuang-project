#!/usr/bin/env python3
import argparse
import base64
import datetime
import getpass
import hashlib
import hmac
import json
import sys
import time
import urllib.error
import urllib.request


def hmac_sha256(key, msg):
    return hmac.new(key, msg.encode("utf-8"), hashlib.sha256).digest()


def tc3_request(secret_id, secret_key, action, region, payload):
    service = "tat"
    host = "tat.tencentcloudapi.com"
    version = "2020-10-28"
    timestamp = int(time.time())
    date = datetime.datetime.utcfromtimestamp(timestamp).strftime("%Y-%m-%d")
    body = json.dumps(payload, separators=(",", ":"))

    content_type = "application/json; charset=utf-8"
    canonical_headers = (
        f"content-type:{content_type}\n"
        f"host:{host}\n"
        f"x-tc-action:{action.lower()}\n"
    )
    signed_headers = "content-type;host;x-tc-action"
    hashed_payload = hashlib.sha256(body.encode("utf-8")).hexdigest()
    canonical_request = "\n".join(
        ["POST", "/", "", canonical_headers, signed_headers, hashed_payload]
    )

    algorithm = "TC3-HMAC-SHA256"
    credential_scope = f"{date}/{service}/tc3_request"
    hashed_request = hashlib.sha256(canonical_request.encode("utf-8")).hexdigest()
    string_to_sign = "\n".join(
        [algorithm, str(timestamp), credential_scope, hashed_request]
    )
    secret_date = hmac_sha256(("TC3" + secret_key).encode("utf-8"), date)
    secret_service = hmac_sha256(secret_date, service)
    secret_signing = hmac_sha256(secret_service, "tc3_request")
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
        f"https://{host}", data=body.encode("utf-8"), headers=headers, method="POST"
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        text = exc.read().decode("utf-8")
        raise RuntimeError(text) from exc


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--region", default="ap-seoul")
    parser.add_argument("--instance-id", required=True)
    parser.add_argument("--timeout", type=int, default=60)
    parser.add_argument("--username", default="")
    parser.add_argument("command")
    args = parser.parse_args()

    secret_id = getpass.getpass("SecretId: ")
    secret_key = getpass.getpass("SecretKey: ")

    payload = {
        "Content": base64.b64encode(args.command.encode("utf-8")).decode("ascii"),
        "InstanceIds": [args.instance_id],
        "CommandType": "SHELL",
        "Timeout": args.timeout,
    }
    if args.username:
        payload["Username"] = args.username

    created = tc3_request(secret_id, secret_key, "RunCommand", args.region, payload)
    response = created.get("Response", {})
    if "Error" in response:
        print(json.dumps(response, ensure_ascii=False, indent=2))
        sys.exit(1)
    invocation_id = response["InvocationId"]
    print(f"InvocationId: {invocation_id}")

    filters = [{"Name": "invocation-id", "Values": [invocation_id]}]
    for _ in range(max(args.timeout, 10)):
        detail = tc3_request(
            secret_id,
            secret_key,
            "DescribeInvocationTasks",
            args.region,
            {"Filters": filters, "HideOutput": False, "Limit": 10},
        )
        task_set = detail.get("Response", {}).get("InvocationTaskSet", [])
        if task_set:
            task = task_set[0]
            status = task.get("TaskStatus")
            print(f"TaskStatus: {status}")
            if status in {
                "SUCCESS",
                "FAILED",
                "TIMEOUT",
                "TASK_TIMEOUT",
                "CANCELLED",
                "TERMINATED",
                "START_FAILED",
                "DELIVER_FAILED",
            }:
                result = task.get("TaskResult") or {}
                output = result.get("Output")
                if output:
                    try:
                        decoded = base64.b64decode(output).decode("utf-8", "replace")
                        print("----- decoded output -----")
                        print(decoded, end="" if decoded.endswith("\n") else "\n")
                        print("----- end decoded output -----")
                    except Exception:
                        pass
                print(json.dumps(task, ensure_ascii=False, indent=2))
                sys.exit(0 if status == "SUCCESS" else 1)
        time.sleep(1)

    print("Timed out waiting for command result", file=sys.stderr)
    sys.exit(1)


if __name__ == "__main__":
    main()
