# Centralis Protocol Reference

This document defines the modular Centralis protocol used between slave, master, and orchestrator.

## Design goals

- Line-delimited JSON packets for simple streaming IO.
- Shared packet types and helpers live in `src/core/protocol`.
- Request/reply uses `id` and `reply_to` to correlate responses.
- Handshake is uniform across connections.

## Terminology

- **Slave**: worker node running VMs/containers.
- **Master**: cluster leader, manages slaves.
- **Orchestrator**: global registry/UI, manages masters.
- **ID**: `base64url(raw(sha256(pubkey)))` for ed25519 public keys.

## Packet format

Every message is a JSON object on a single line:

```json
{"type":"heartbeat","id":"...","reply_to":"...","error":"...","payload":{}}
```

Fields:

- `type` (string, required)
- `id` (string, optional): unique id for request/response correlation.
- `reply_to` (string, optional): id of the packet being answered.
- `error` (string, optional): human-readable error.
- `payload` (object, optional): message body.

## Handshake (master <-> slave)

1. Client sends hello line: `CENTRALISD-MS/1`.
2. Client sends `auth.hello` packet with `{id, pubkey, role:"slave"}`.
3. Server replies `auth.challenge` with `{challenge}`.
4. Client replies `auth.proof` with `{signature}`.
5. Server replies `auth.ok` or `error`.

## Handshake (master <-> orchestrator)

1. Client sends hello line: `CENTRALISD-ORCH/1`.
2. Server replies `auth.challenge` with `{challenge}`.
3. Client sends `auth.hello` with `{id, pubkey, role:"master", name, cluster, advertise}`.
4. Client sends `auth.proof` with `{signature}`.
5. Server replies `auth.ok` or `error`.

## Core packet types

- `auth.hello` payload: `AuthHello`
- `auth.challenge` payload: `AuthChallenge`
- `auth.proof` payload: `AuthProof`
- `auth.ok`: no payload
- `error`: optional `error` string
- `heartbeat` / `heartbeat.reply` payload: `Heartbeat`
- `node.command` / `node.command.reply` payload: `CommandReply`
- `orchestrator.command` payload: `OrchestratorCommand`
- `master.register` / `master.heartbeat` payload: `MasterInfo`

## Node commands

`node.command` payload is a `NodeCommand` object with `action` and optional `params`.

Supported actions:

- `libvirt.domains.list`: return list of libvirt domains from the slave.

## Heartbeat flow

Master sends `heartbeat` to slave. Slave replies `heartbeat.reply` with:

```json
{
  "usage": {"cpu_percent": 12.5, "ram_percent": 43.2},
  "hardware": {"cpu_cores": 8, "ram_bytes": 34359738368}
}
```

## Orchestrator command flow

Orchestrator sends `orchestrator.command` to master with:

```json
{"node_id":"<slave-id>","command":{...}}
```

Master forwards as `node.command` to slave. Slave may respond `node.command.reply`.

## Shared code

All packet/handshake helpers are in `src/core/protocol` and should be reused by all components.
