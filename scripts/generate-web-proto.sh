#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_ROOT="${SOURCE_ROOT:-$(cd "${ROOT}/.." && pwd)}"
WEBUI_ROOT="${WEBUI_ROOT:-${SOURCE_ROOT}/webui}"
PLUGIN="${WEBUI_ROOT}/node_modules/.bin/protoc-gen-ts_proto"
PROTO_DIR="${ROOT}/proto"
OUT_DIR="${ROOT}/webui/src/dashboard/proto"
PROTO="${PROTO_DIR}/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime.proto"

if [[ ! -x "${PLUGIN}" ]]; then
  printf 'ts-proto plugin not found at %s; run npm install in webui first\n' "${PLUGIN}" >&2
  exit 1
fi

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

protoc -I "${PROTO_DIR}" \
  --plugin="protoc-gen-ts_proto=${PLUGIN}" \
  --ts_proto_out="${OUT_DIR}" \
  --ts_proto_opt=onlyTypes=true,outputServices=none,esModuleInterop=true,useJsonWireFormat=true,snakeToCamel=false \
  "${PROTO}"
