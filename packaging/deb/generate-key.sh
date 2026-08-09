#!/bin/sh
# Creates the ES256 signing key for JWTs on first start, if it is not there yet.
#
# Run as ExecStartPre, not from postinst, on purpose: the unit uses DynamicUser=yes, so the
# owning UID only exists while the service runs. StateDirectory= is the one place systemd has
# already created and chowned to that UID by the time this executes.
set -eu

key="${AUTH_PRIVATE_KEY:?AUTH_PRIVATE_KEY is not set}"

if [ -s "$key" ]; then
    exit 0
fi

umask 077
# main.go parses the key with x509.ParsePKCS8PrivateKey and requires ECDSA — genpkey emits PKCS#8.
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -outform PEM -out "$key"
