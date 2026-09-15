# Digital Twin Live Metrics

The dashboard metrics are supplied by the company `SdyService` Triple RPC.
The approved protocol source is kept at:

```text
proto/soyoung/primecrm/sdy.proto
```

The service full name is `com.soyoung.primecrm.SdyService`. The company base
library uses ZooKeeper interface discovery, so the application must not put a
provider `host:port` in source code. In test, the base library selects
`10.10.10.100:2181` from `APP_RUN_ENV=test`; production selection remains the
base library's responsibility.

## Required generated artifacts

Generate the protocol and Triple client with versions compatible with the
company base library's Dubbo-Go 3.1.1 runtime. The generated files should be
kept outside this package, for example:

```text
internal/digitaltwin/primecrm/sdy.pb.go
internal/digitaltwin/primecrm/sdy_triple.pb.go
```

The checked-in generator is `protoc-gen-go-triple 1.0.2`, which already emits
the legacy client shape expected by the company wrapper. Do not add
`useOldVersion=true` to this generator, and do not hand-edit generated files.

From the repository root, the generation command is:

```bash
PATH="$PWD/.tools/bin:$PATH" protoc \
  -I . \
  --go_out=. \
  --go_opt=module=github.com/shalei-pm/erzhuang-project \
  --go-triple_out=. \
  --go-triple_opt=module=github.com/shalei-pm/erzhuang-project \
  proto/soyoung/primecrm/sdy.proto
```

## Provider adapter contract

Implement `digitaltwin.Provider` by obtaining the generated client through the
company wrapper:

```go
client := dubbo.GetClient[primecrm.SdyServiceClientImpl]()
```

Map the three generated responses into `Overview`, `DutyStaff`, and
`TrafficFlow`. The adapter should use the request's `TenantID` and `Date` for
all three calls. `digitaltwin.Service` owns the shared timeout, concurrency,
validation, and all-or-nothing failure behavior.

The production constructor initializes this provider when the company library
is available. The library selects ZooKeeper from `APP_RUN_ENV`; if client
initialization fails, the dashboard endpoint returns a controlled 503 rather
than synthetic values.
