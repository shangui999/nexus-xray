#!/bin/bash
# 生成 Server CA 证书（用于 mTLS）
set -e

CERT_DIR="${1:-./certs}"
mkdir -p "$CERT_DIR"

echo "Generating CA certificate..."
openssl req -x509 -newkey rsa:4096 -keyout "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
    -days 3650 -nodes -subj "/CN=XrayManager CA"

echo "Generating Server certificate..."
openssl req -newkey rsa:2048 -keyout "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" \
    -nodes -subj "/CN=xray-manager-server"
openssl x509 -req -in "$CERT_DIR/server.csr" -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
    -CAcreateserial -out "$CERT_DIR/server.crt" -days 365

rm -f "$CERT_DIR/server.csr" "$CERT_DIR/ca.srl"

echo "Certificates generated in $CERT_DIR/"
echo "  CA:     $CERT_DIR/ca.crt"
echo "  Server: $CERT_DIR/server.crt + $CERT_DIR/server.key"
