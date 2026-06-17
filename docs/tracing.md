# Tracing

Nanvil enables `SaveInvocations` by default, so transaction and invocation logs are available via standard RPC without extra flags.

Query execution results:

```json
{"method":"getapplicationlog","params":["<txhash>"]}
```

Use verbose invoke for test calls:

```json
{"method":"invokefunction","params":[..., true]}
```

The `--print-traces` CLI flag is reserved for future extended trace output.
