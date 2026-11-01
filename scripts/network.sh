#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
case "${1:-}" in
 install)
  command -v docker >/dev/null
  mkdir -p network
  cd network
  curl -fL https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh -o install-fabric.sh
  # Review the downloaded upstream script before executing the next command.
  echo 'Downloaded network/install-fabric.sh. Review it, then run: cd network && bash install-fabric.sh --fabric-version 2.5.12 --ca-version 1.5.15 docker samples binary'
  ;;
 up)
  cd network/fabric-samples/test-network
  ./network.sh up createChannel -c mychannel
  ./network.sh deployCC -c mychannel -ccn evidence -ccp "$ROOT/chaincode" -ccl go -ccv 1.0 -ccs 1
  ;;
 down)
  cd network/fabric-samples/test-network
  ./network.sh down
  ;;
 *) echo 'Usage: bash scripts/network.sh install|up|down'; exit 1;;
esac
