# Digital Twin Live Metrics

The dashboard metrics are supplied by the company `SdyService` Triple RPC.
The approved protocol source is kept at:

```text
proto/soyoung/primecrm/sdy.proto
```

The service full name is `com.soyoung.primecrm.SdyService`. The application
uses Dubbo-Go Triple with ZooKeeper interface discovery and never configures a
fixed provider `host:port`. `APP_RUN_ENV` selects the same registry convention
as the company base library: test uses `10.10.10.100:2181`, while production
uses the company register-center hostnames.

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

Implement `digitaltwin.Provider` with the generated Triple client and a lazy
Dubbo-Go consumer configuration. The project does not import the company
wrapper because its Linux qconf adapter requires CGO and is incompatible with
this project's static container build. The direct consumer keeps the same
Triple protocol, service name, ZooKeeper discovery convention, and request
contract without requiring instance-level configuration.

Map the three generated responses into `Overview`, `DutyStaff`, and
`TrafficFlow`. The adapter should use the request's `TenantID` and `Date` for
all three calls. `digitaltwin.Service` owns the shared timeout, concurrency,
validation, and all-or-nothing failure behavior.

The production constructor initializes the consumer on its first request. If
registry or client initialization fails, the dashboard endpoint returns a
controlled 503 rather than synthetic values.

The frontend requests metrics immediately when a store is selected, then
refreshes them every 30 seconds while the page is visible. Background tabs do
not poll. Returning to a visible tab triggers one immediate refresh and starts
a new 30-second interval. Failed refreshes preserve the last displayed values
and mark affected metrics as stale instead of replacing them with zero.
