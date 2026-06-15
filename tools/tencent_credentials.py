#!/usr/bin/env python3
import getpass
import os


def read_tencent_credentials():
    secret_id = os.environ.get("TENCENTCLOUD_SECRET_ID") or getpass.getpass("SecretId: ")
    secret_key = os.environ.get("TENCENTCLOUD_SECRET_KEY") or getpass.getpass("SecretKey: ")
    return secret_id, secret_key
