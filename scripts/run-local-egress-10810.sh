#!/usr/bin/env sh
set -eu

COMMON_EGRESS_ADDR=${COMMON_EGRESS_ADDR:-:10810}
DYNAMIC_EGRESS_ADDR=${DYNAMIC_EGRESS_ADDR:-:10811}
COMMON_EGRESS_UPSTREAM=${COMMON_EGRESS_UPSTREAM:-127.0.0.1:10810}
TEN24_PROXY_ADDR=${TEN24_PROXY_ADDR:-us.1024proxy.io:3000}
TEN24_PROXY_USERNAME=${TEN24_PROXY_USERNAME:-}
TEN24_PROXY_PASSWORD=${TEN24_PROXY_PASSWORD:-}
GOST_PATH=${GOST_PATH:-gost}

if [ -z "$TEN24_PROXY_USERNAME" ]; then
  printf 'TEN24_PROXY_USERNAME is required\n' >&2
  exit 1
fi
if [ -z "$TEN24_PROXY_PASSWORD" ]; then
  printf 'TEN24_PROXY_PASSWORD is required\n' >&2
  exit 1
fi

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

render_config() {
  common_addr=$(json_escape "$COMMON_EGRESS_ADDR")
  dynamic_addr=$(json_escape "$DYNAMIC_EGRESS_ADDR")
  common_upstream=$(json_escape "$COMMON_EGRESS_UPSTREAM")
  ten24_addr=$(json_escape "$TEN24_PROXY_ADDR")
  ten24_username=$(json_escape "$TEN24_PROXY_USERNAME")
  ten24_password=$(json_escape "$TEN24_PROXY_PASSWORD")

  cat <<EOF
{
  "services": [
    {
      "name": "common-egress-10810",
      "addr": "$common_addr",
      "handler": {
        "type": "socks5"
      },
      "listener": {
        "type": "tcp"
      }
    },
    {
      "name": "dynamic-ip-via-10810",
      "addr": "$dynamic_addr",
      "handler": {
        "type": "socks5",
        "chain": "dynamic-ip-via-10810"
      },
      "listener": {
        "type": "tcp"
      }
    }
  ],
  "chains": [
    {
      "name": "dynamic-ip-via-10810",
      "hops": [
        {
          "name": "common-egress-hop",
          "nodes": [
            {
              "name": "common-egress-10810",
              "addr": "$common_upstream",
              "connector": {
                "type": "socks5"
              },
              "dialer": {
                "type": "tcp"
              }
            }
          ]
        },
        {
          "name": "ten24-dynamic-ip-hop",
          "nodes": [
            {
              "name": "ten24-dynamic-ip",
              "addr": "$ten24_addr",
              "connector": {
                "type": "socks5",
                "auth": {
                  "username": "$ten24_username",
                  "password": "$ten24_password"
                }
              },
              "dialer": {
                "type": "tcp"
              }
            }
          ]
        }
      ]
    }
  ]
}
EOF
}

if [ "${1:-}" = "--print" ]; then
  render_config
  exit 0
fi

config_file=$(mktemp "${TMPDIR:-/tmp}/proxy-runtime-egress-10810.XXXXXX.json")
trap 'rm -f "$config_file"' EXIT INT TERM

render_config >"$config_file"
exec "$GOST_PATH" -C "$config_file"
